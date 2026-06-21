package ecoflow

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/xkhronoz/ecoflow-ble-nutd/internal/config"
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
	return Packet{
		Src:     0x02,
		Dst:     0x21,
		CmdSet:  0x20,
		CmdID:   0x02,
		Payload: payload,
		Version: 0x02,
	}.MarshalBinary()
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
	return Packet{
		Src:     0x02,
		Dst:     0x20,
		CmdSet:  0xFE,
		CmdID:   0x15,
		Payload: payload,
		DSrc:    0x01,
		DDst:    0x01,
		Version: 0x13,
	}.MarshalBinary()
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
