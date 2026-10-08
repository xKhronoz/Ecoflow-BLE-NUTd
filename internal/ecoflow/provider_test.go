package ecoflow

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/xkhronoz/ecoflow-ble-nutd/internal/config"
	"github.com/xkhronoz/ecoflow-ble-nutd/internal/ecoflow/pb/pd335pb"
	"github.com/xkhronoz/ecoflow-ble-nutd/internal/ecoflow/pb/pr705pb"
	"github.com/xkhronoz/ecoflow-ble-nutd/internal/state"
	"google.golang.org/protobuf/proto"
)

type fakeTransport struct {
	mu           sync.Mutex
	discoveries  map[string]Discovery
	connections  map[string][]*fakeConn
	findCalls    []string
	connectCalls int
}

func (f *fakeTransport) FindByMAC(ctx context.Context, adapterID string, mac string, timeout time.Duration) (Discovery, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.findCalls = append(f.findCalls, mac)
	discovery, ok := f.discoveries[mac]
	if !ok {
		return Discovery{}, fmt.Errorf("unknown MAC %s", mac)
	}
	return discovery, nil
}

func (f *fakeTransport) Connect(ctx context.Context, adapterID string, discovery Discovery, timeout time.Duration) (BLEConnection, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.connectCalls++
	queue := f.connections[discovery.MAC]
	if len(queue) == 0 {
		return nil, fmt.Errorf("no fake connection for %s", discovery.MAC)
	}
	conn := queue[0]
	f.connections[discovery.MAC] = queue[1:]
	return conn, nil
}

type fakeConn struct {
	mu        sync.Mutex
	handler   func([]byte)
	writes    [][]byte
	writeHook func(*fakeConn, []byte, bool) error
}

func (f *fakeConn) StartNotifications(handler func([]byte)) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.handler = handler
	return nil
}

func (f *fakeConn) Write(data []byte, withResponse bool) error {
	f.mu.Lock()
	f.writes = append(f.writes, append([]byte(nil), data...))
	hook := f.writeHook
	f.mu.Unlock()
	if hook != nil {
		return hook(f, data, withResponse)
	}
	return nil
}

func (f *fakeConn) Close() error { return nil }

func (f *fakeConn) emit(data []byte) {
	f.mu.Lock()
	handler := f.handler
	f.mu.Unlock()
	if handler != nil {
		handler(append([]byte(nil), data...))
	}
}

func TestProviderCloudBootstrapResolvesUserIDOnce(t *testing.T) {
	t.Parallel()

	var loginCalls atomic.Int32
	cfg := &config.Config{
		Provider: config.ProviderCfg{
			Type:                  "eco-ble",
			Adapter:               "hci0",
			ScanTimeoutSeconds:    1,
			ConnectTimeoutSeconds: 1,
			ReconnectDelaySeconds: 1,
			PollSeconds:           1,
			Auth: config.ProviderAuthConfig{
				Email:    "user@example.com",
				Password: "secret",
				Region:   "auto",
			},
		},
		Devices: []config.Device{
			{Name: "delta2-a", MAC: "AA:BB:CC:DD:EE:01", StaleTimeoutSeconds: 1, LowBatteryPercent: 20, LowRuntimeSeconds: 300},
			{Name: "delta2-b", MAC: "AA:BB:CC:DD:EE:02", StaleTimeoutSeconds: 1, LowBatteryPercent: 20, LowRuntimeSeconds: 300},
		},
	}
	transport := &fakeTransport{
		discoveries: map[string]Discovery{
			"AA:BB:CC:DD:EE:01": makeDiscovery("AA:BB:CC:DD:EE:01", "R331123456789012", 0),
			"AA:BB:CC:DD:EE:02": makeDiscovery("AA:BB:CC:DD:EE:02", "R331123456789013", 0),
		},
		connections: map[string][]*fakeConn{
			"AA:BB:CC:DD:EE:01": {{}, {}},
			"AA:BB:CC:DD:EE:02": {{}, {}},
		},
	}
	provider := NewProvider(cfg)
	provider.transport = transport
	provider.loginClient = &LoginClient{
		HTTPClient: &http.Client{Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
			loginCalls.Add(1)
			return jsonHTTPResponse(http.StatusOK, `{"code":"0","message":"OK","data":{"user":{"userId":"cloud-user"}}}`), nil
		})},
		BaseURLByRegion: map[string]string{"api": "https://api.ecoflow.test"},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1200*time.Millisecond)
	defer cancel()
	_ = provider.Run(ctx, state.New())

	if loginCalls.Load() != 1 {
		t.Fatalf("login calls = %d", loginCalls.Load())
	}
}

func TestRunSessionPublishesTelemetryAndTransitionsToWAIT(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Provider: config.ProviderCfg{
			Type:                  "eco-ble",
			Adapter:               "hci0",
			ScanTimeoutSeconds:    1,
			ConnectTimeoutSeconds: 1,
			ReconnectDelaySeconds: 1,
			PollSeconds:           1,
			Auth:                  config.ProviderAuthConfig{UserID: "user-123"},
		},
	}
	device := config.Device{
		Name:                "delta2",
		MAC:                 "AA:BB:CC:DD:EE:10",
		StaleTimeoutSeconds: 1,
		LowBatteryPercent:   20,
		LowRuntimeSeconds:   300,
	}
	conn := &fakeConn{}
	conn.writeHook = func(c *fakeConn, data []byte, withResponse bool) error {
		c.mu.Lock()
		writeCount := len(c.writes)
		c.mu.Unlock()
		if writeCount == 2 {
			go c.emit(makeV2TelemetryPacket(74, 1800, 120, 85))
		}
		return nil
	}
	provider := NewProvider(cfg)
	provider.transport = &fakeTransport{
		discoveries: map[string]Discovery{
			device.MAC: makeDiscovery(device.MAC, "R331123456789099", 0),
		},
		connections: map[string][]*fakeConn{
			device.MAC: {conn},
		},
	}
	store := state.New()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errCh := make(chan error, 1)
	go func() {
		errCh <- provider.runSession(ctx, store, device, "user-123", "user_id")
	}()

	waitForCondition(t, 1500*time.Millisecond, func() bool {
		ups, ok := store.Get(device.Name)
		return ok && ups.Vars["battery.charge"] == "74" && ups.Vars["ups.status"] == "OL"
	})
	waitForCondition(t, 1800*time.Millisecond, func() bool {
		ups, ok := store.Get(device.Name)
		return ok && ups.Vars["ups.status"] == "WAIT"
	})
	cancel()
	<-errCh
}

func TestRunDeviceReconnectsAfterDisconnect(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Provider: config.ProviderCfg{
			Type:                  "eco-ble",
			Adapter:               "hci0",
			ScanTimeoutSeconds:    1,
			ConnectTimeoutSeconds: 1,
			ReconnectDelaySeconds: 1,
			PollSeconds:           1,
			Auth:                  config.ProviderAuthConfig{UserID: "user-123"},
		},
	}
	device := config.Device{
		Name:                "delta2",
		MAC:                 "AA:BB:CC:DD:EE:11",
		StaleTimeoutSeconds: 1,
		LowBatteryPercent:   20,
		LowRuntimeSeconds:   300,
	}
	first := &fakeConn{}
	first.writeHook = func(c *fakeConn, data []byte, withResponse bool) error {
		c.mu.Lock()
		writeCount := len(c.writes)
		c.mu.Unlock()
		if writeCount == 2 {
			go c.emit(makeV2TelemetryPacket(70, 2000, 90, 60))
			return nil
		}
		if writeCount >= 3 {
			return fmt.Errorf("simulated disconnect")
		}
		return nil
	}
	second := &fakeConn{}
	second.writeHook = func(c *fakeConn, data []byte, withResponse bool) error {
		c.mu.Lock()
		writeCount := len(c.writes)
		c.mu.Unlock()
		if writeCount == 2 {
			go c.emit(makeV2TelemetryPacket(69, 1900, 100, 55))
		}
		return nil
	}
	transport := &fakeTransport{
		discoveries: map[string]Discovery{
			device.MAC: makeDiscovery(device.MAC, "R331123456789100", 0),
		},
		connections: map[string][]*fakeConn{
			device.MAC: {first, second},
		},
	}
	provider := NewProvider(cfg)
	provider.transport = transport

	ctx, cancel := context.WithTimeout(context.Background(), 3200*time.Millisecond)
	defer cancel()
	go provider.runDevice(ctx, state.New(), device, "user-123", "user_id")

	waitForCondition(t, 3*time.Second, func() bool {
		transport.mu.Lock()
		defer transport.mu.Unlock()
		return transport.connectCalls >= 2
	})
}

func TestRunSessionRejectsUnsupportedSerialPrefix(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Provider: config.ProviderCfg{
			Type:                  "eco-ble",
			Adapter:               "hci0",
			ScanTimeoutSeconds:    1,
			ConnectTimeoutSeconds: 1,
			ReconnectDelaySeconds: 1,
			PollSeconds:           1,
			Auth:                  config.ProviderAuthConfig{UserID: "user-123"},
		},
	}
	device := config.Device{
		Name:                "unknown",
		MAC:                 "AA:BB:CC:DD:EE:12",
		StaleTimeoutSeconds: 1,
		LowBatteryPercent:   20,
		LowRuntimeSeconds:   300,
	}
	provider := NewProvider(cfg)
	provider.transport = &fakeTransport{
		discoveries: map[string]Discovery{
			device.MAC: makeDiscovery(device.MAC, "ZZZZ123456789012", 0),
		},
		connections: map[string][]*fakeConn{
			device.MAC: {{}},
		},
	}
	err := provider.runSession(context.Background(), state.New(), device, "user-123", "user_id")
	if err == nil {
		t.Fatalf("expected unsupported serial error")
	}
}

func TestBootstrapRejectsShortPublicKeyReply(t *testing.T) {
	p := NewProvider(&config.Config{})
	raw := make(chan []byte, 1)
	raw <- mustEncode((&SimplePacketAssembler{}).Encode([]byte{1, 0, 0, 1}))
	_, _, err := p.bootstrapSession(context.Background(), &fakeConn{}, raw, DeviceDescriptor{EncryptType: 7}, "user")
	if err == nil || !strings.Contains(err.Error(), "public key response") {
		t.Fatalf("expected short public key error, got %v", err)
	}
}

func TestFinishAuthenticationPreservesFollowingTelemetry(t *testing.T) {
	p := NewProvider(&config.Config{Provider: config.ProviderCfg{ConnectTimeoutSeconds: 1}})
	raw := make(chan []byte, 1)
	reply := mustEncode(Packet{Src: 0x35, CmdSet: 0x35, CmdID: 0x86, Payload: []byte{0}, Version: 2}.MarshalBinary())
	raw <- append(reply, makeV2TelemetryPacket(74, 1800, 120, 85)...)
	pending, err := p.finishAuthentication(context.Background(), &fakeConn{}, raw, DeviceDescriptor{PacketVersion: 2}, "user", &PassthroughAssembler{})
	if err != nil || len(pending) != 1 || pending[0].CmdSet != 0x20 {
		t.Fatalf("pending packets = %#v, error = %v", pending, err)
	}
}

func TestBootstrapType7Delta3(t *testing.T) {
	p := NewProvider(&config.Config{Provider: config.ProviderCfg{ConnectTimeoutSeconds: 1}})
	desc, err := resolveDescriptor(makeDiscovery("AA:BB:CC:DD:EE:FF", "P231FAB4PJ7X3193", 7))
	if err != nil {
		t.Fatal(err)
	}
	devicePrivate, devicePublic, err := generateSECP160KeyPair()
	if err != nil {
		t.Fatal(err)
	}
	raw := make(chan []byte, 16)
	conn := &fakeConn{}
	_ = conn.StartNotifications(func(data []byte) { raw <- data })
	simple := &SimplePacketAssembler{}
	var initialEncryption Type7Encryption
	var sessionAssembler *EncPacketAssembler
	step := 0
	conn.writeHook = func(c *fakeConn, data []byte, withResponse bool) error {
		step++
		if !withResponse {
			t.Fatal("type7 handshake must write with response")
		}
		switch step {
		case 1:
			command, ok := simple.Parse(data)
			if !ok || len(command) != 42 || command[0] != 1 {
				t.Fatalf("public key request = %x", command)
			}
			shared, err := deriveSharedSecret(devicePrivate, command[2:])
			if err != nil {
				t.Fatal(err)
			}
			key, iv := type7SessionSeed(shared)
			initialEncryption = Type7Encryption{SessionKey: key, IV: iv}
			reply := mustEncode(simple.Encode(append([]byte{1, 0, 0}, devicePublic...)))
			reply = append(reply, reply...) // Repeated replies must not confuse the next stage.
			c.emit(reply[:1])
			c.emit(reply[1:])
		case 2:
			command, ok := simple.Parse(data)
			if !ok || !bytes.Equal(command, []byte{2}) {
				t.Fatalf("session key request = %x", command)
			}
			keyInfo := make([]byte, 18)
			for i := range 16 {
				keyInfo[i] = byte(i)
			}
			keyInfo[17] = 1
			encrypted, err := initialEncryption.Encrypt(keyInfo)
			if err != nil {
				t.Fatal(err)
			}
			c.emit(mustEncode(simple.Encode(append([]byte{2}, encrypted...))))
			// Independent expected vector for seed 0001 and srand 000102...0f.
			key, _ := hex.DecodeString("cf19aab35c7605235e885521945354d8")
			sessionAssembler = &EncPacketAssembler{encryption: Type7Encryption{SessionKey: key, IV: initialEncryption.IV}}
		case 3, 4:
			payloads, err := sessionAssembler.Reassemble(data)
			if err != nil || len(payloads) != 1 {
				t.Fatalf("authentication request payloads = %x, %v", payloads, err)
			}
			request, err := ParsePacket(payloads[0], false)
			if err != nil {
				t.Fatal(err)
			}
			if step == 3 && request.CmdID != 0x89 {
				t.Fatalf("expected auth status request, got %#v", request)
			}
			if step == 4 {
				if request.CmdID != 0x86 || !bytes.Equal(request.Payload, authPacket(desc, "user").Payload) {
					t.Fatalf("unexpected auth request: %#v", request)
				}
				reply, err := sessionAssembler.Encode(Packet{Src: 0x35, CmdSet: 0x35, CmdID: 0x86, Payload: []byte{0}, Version: 0x03})
				if err != nil {
					t.Fatal(err)
				}
				charge := float32(74)
				telemetryPayload, err := proto.Marshal(&pd335pb.DisplayPropertyUpload{CmsBattSoc: &charge})
				if err != nil {
					t.Fatal(err)
				}
				telemetry, err := sessionAssembler.Encode(Packet{Src: 2, CmdSet: 0xfe, CmdID: 0x15, Payload: telemetryPayload, Version: 0x03})
				if err != nil {
					t.Fatal(err)
				}
				c.emit(append(reply, telemetry...))
			}
		default:
			t.Fatalf("unexpected handshake step %d", step)
		}
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	assembler, pending, err := p.bootstrapSession(ctx, conn, raw, desc, "user")
	if err != nil || assembler == nil || len(pending) != 1 || step != 4 {
		t.Fatalf("handshake: steps = %d, pending = %#v, error = %v", step, pending, err)
	}
	telemetry, _, handled, err := newHandler(desc.Profile).HandlePacket(pending[0])
	if err != nil || !handled || telemetry == nil || telemetry.Charge == nil || *telemetry.Charge != 74 {
		t.Fatalf("DELTA 3 telemetry = %#v, error = %v", telemetry, err)
	}
}

func TestRunSessionRetainsPartialTelemetry(t *testing.T) {
	cfg := &config.Config{Provider: config.ProviderCfg{ConnectTimeoutSeconds: 1, PollSeconds: 1}}
	device := config.Device{Name: "delta3", MAC: "AA:BB:CC:DD:EE:FF", StaleTimeoutSeconds: 60}
	conn := &fakeConn{}
	conn.writeHook = func(c *fakeConn, data []byte, _ bool) error {
		request, err := ParsePacket(data, false)
		if err != nil {
			return err
		}
		if request.CmdID == 0x86 {
			c.emit(makeV3TelemetryPacket(74, 1800, 120, 85))
		}
		return nil
	}
	p := NewProvider(cfg)
	p.transport = &fakeTransport{
		discoveries: map[string]Discovery{device.MAC: makeDiscovery(device.MAC, "P231FAB4PJ7X3193", 0)},
		connections: map[string][]*fakeConn{device.MAC: {conn}},
	}
	store := state.New()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- p.runSession(ctx, store, device, "user", "user_id") }()
	defer func() {
		cancel()
		if err := <-done; err != nil {
			t.Error(err)
		}
	}()
	waitForCondition(t, time.Second, func() bool {
		u, ok := store.Get(device.Name)
		return ok && u.Vars["battery.charge"] == "74"
	})
	output := float32(42)
	payload, err := proto.Marshal(&pd335pb.DisplayPropertyUpload{PowOutSumW: &output})
	if err != nil {
		t.Fatal(err)
	}
	conn.emit(mustEncode(Packet{Src: 2, CmdSet: 0xfe, CmdID: 0x15, Payload: payload, Version: 0x03}.MarshalBinary()))
	waitForCondition(t, time.Second, func() bool {
		u, ok := store.Get(device.Name)
		return ok && u.Vars["output.power"] == "42"
	})
	u, _ := store.Get(device.Name)
	if u.Vars["ups.status"] != "OL" || u.Vars["battery.charge"] != "74" || u.Vars["battery.runtime"] != "1800" || u.Vars["input.power"] != "120" {
		t.Fatalf("partial telemetry discarded prior state: %#v", u.Vars)
	}
}

func makeDiscovery(mac string, serial string, encryptType int) Discovery {
	return Discovery{
		MAC:       mac,
		LocalName: "EcoFlow Test",
		ManufacturerData: []ManufacturerData{
			{
				CompanyID: ecoFlowManufacturerID,
				Data:      makeManufacturerBytes(serial, encryptType),
			},
		},
	}
}

func makeV2TelemetryPacket(charge int, runtime int32, inputPower uint16, outputPower uint16) []byte {
	payload := mustBinary(v2PDPrefix{
		SOC:         uint8(charge),
		WattsInSum:  inputPower,
		WattsOutSum: outputPower,
		RemainTime:  runtime,
	})
	return mustEncode(Packet{
		Src:     0x02,
		Dst:     0x21,
		CmdSet:  0x20,
		CmdID:   0x02,
		Payload: payload,
		Version: 0x02,
	}.MarshalBinary())
}

func makeV3TelemetryPacket(charge int, runtime uint32, input float32, output float32) []byte {
	chargeF := float32(charge)
	msg := &pr705pb.DisplayPropertyUpload{
		CmsBattSoc:    &chargeF,
		CmsDsgRemTime: &runtime,
		PowInSumW:     &input,
		PowOutSumW:    &output,
	}
	payload, err := proto.Marshal(msg)
	if err != nil {
		panic(err)
	}
	return mustEncode(Packet{
		Src:     0x02,
		Dst:     0x20,
		CmdSet:  0xFE,
		CmdID:   0x15,
		Payload: payload,
		DSrc:    0x01,
		DDst:    0x01,
		Version: 0x13,
	}.MarshalBinary())
}

func mustBinary[T any](value T) []byte {
	var buf bytes.Buffer
	if err := binary.Write(&buf, binary.LittleEndian, value); err != nil {
		panic(err)
	}
	return buf.Bytes()
}

func waitForCondition(t *testing.T, timeout time.Duration, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("condition not met before timeout")
}
