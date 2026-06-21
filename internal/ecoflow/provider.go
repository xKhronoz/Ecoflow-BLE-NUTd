package ecoflow

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/xkhronoz/ecoflow-ble-nutd/internal/config"
	"github.com/xkhronoz/ecoflow-ble-nutd/internal/state"
)

type Provider struct {
	cfg         *config.Config
	transport   Transport
	loginClient *LoginClient
}

func NewProvider(cfg *config.Config) *Provider {
	return &Provider{
		cfg:         cfg,
		transport:   &TinygoTransport{},
		loginClient: &LoginClient{},
	}
}

func (p *Provider) Run(ctx context.Context, store *state.Store) error {
	userID, err := p.loginClient.ResolveUserID(ctx, p.cfg.Provider.Auth)
	if err != nil {
		return err
	}
	authMode := "user_id"
	if p.cfg.Provider.Auth.UserID == "" {
		authMode = "cloud-bootstrap"
	}
	slog.Info("eco-ble auth resolved", "mode", authMode)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	errCh := make(chan error, len(p.cfg.Devices))
	var wg sync.WaitGroup
	for _, device := range p.cfg.Devices {
		device := device
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := p.runDevice(ctx, store, device, userID, authMode); err != nil && ctx.Err() == nil {
				select {
				case errCh <- err:
				default:
				}
				cancel()
			}
		}()
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-ctx.Done():
		<-done
		if err := context.Cause(ctx); err != nil && !errors.Is(err, context.Canceled) {
			return err
		}
		return nil
	case err := <-errCh:
		<-done
		return err
	}
}

func (p *Provider) runDevice(ctx context.Context, store *state.Store, device config.Device, userID string, authMode string) error {
	store.Upsert(device.Name, device.Description, staleVars(device, device.Model, device.Serial))
	reconnectDelay := time.Duration(p.cfg.Provider.ReconnectDelaySeconds) * time.Second
	for {
		err := p.runSession(ctx, store, device, userID, authMode)
		if err == nil || ctx.Err() != nil {
			return nil
		}
		store.Upsert(device.Name, device.Description, staleVars(device, device.Model, device.Serial))
		slog.Warn("eco-ble reconnect scheduled", "device", device.Name, "error", err, "delay", reconnectDelay.String())
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(reconnectDelay):
		}
	}
}

func (p *Provider) runSession(ctx context.Context, store *state.Store, device config.Device, userID string, authMode string) error {
	scanTimeout := time.Duration(p.cfg.Provider.ScanTimeoutSeconds) * time.Second
	connectTimeout := time.Duration(p.cfg.Provider.ConnectTimeoutSeconds) * time.Second
	discovery, err := p.transport.FindByMAC(ctx, p.cfg.Provider.Adapter, device.MAC, scanTimeout)
	if err != nil {
		return err
	}
	slog.Info("eco-ble scan match", "device", device.Name, "mac", discovery.MAC, "name", discovery.LocalName)

	desc, err := resolveDescriptor(discovery)
	if err != nil {
		slog.Warn("eco-ble unsupported-device rejection", "device", device.Name, "mac", discovery.MAC, "error", err)
		return err
	}
	conn, err := p.transport.Connect(ctx, p.cfg.Provider.Adapter, discovery, connectTimeout)
	if err != nil {
		return err
	}
	defer conn.Close()
	slog.Info("eco-ble connected", "device", device.Name, "serial", desc.Serial, "profile", desc.ProfileName, "encrypt_type", desc.EncryptType)

	handler := newHandler(desc.Profile)
	if handler == nil {
		return fmt.Errorf("no handler for profile %s", desc.Profile)
	}

	rawCh := make(chan []byte, 128)
	if err := conn.StartNotifications(func(data []byte) {
		select {
		case rawCh <- data:
		default:
			slog.Warn("eco-ble notification dropped", "device", device.Name, "bytes", len(data))
		}
	}); err != nil {
		return err
	}

	assembler, pending, err := p.bootstrapSession(ctx, conn, rawCh, desc, userID)
	if err != nil {
		slog.Warn("eco-ble auth failure", "device", device.Name, "serial", desc.Serial, "mode", authMode, "error", err)
		return err
	}
	slog.Info("eco-ble authenticated", "device", device.Name, "serial", desc.Serial, "mode", authMode)

	responder := &timeResponder{}
	pollInterval := time.Duration(p.cfg.Provider.PollSeconds) * time.Second
	staleTimeout := time.Duration(device.StaleTimeoutSeconds) * time.Second
	idleTicker := time.NewTicker(pollInterval)
	defer idleTicker.Stop()
	lastPacket := time.Now()
	var lastFresh time.Time
	isStale := true

	processPacket := func(packet Packet) error {
		if desc.SupportsTime && isTimeRequest(packet) {
			packets, send, err := responder.Packets(time.Now())
			if err != nil {
				return err
			}
			if send {
				for _, reply := range packets {
					if err := p.writePacket(conn, assembler, reply); err != nil {
						return err
					}
				}
			}
		}
		telemetry, outbound, handled, err := handler.HandlePacket(packet)
		if err != nil {
			return err
		}
		if !handled {
			return nil
		}
		for _, outboundPacket := range outbound {
			if err := p.writePacket(conn, assembler, outboundPacket); err != nil {
				return err
			}
		}
		if telemetry == nil || telemetry.Empty() {
			return nil
		}
		lastFresh = time.Now()
		isStale = false
		store.Upsert(device.Name, device.Description, liveVars(device, desc, *telemetry))
		return nil
	}

	for _, packet := range pending {
		if err := processPacket(packet); err != nil {
			return err
		}
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case chunk := <-rawCh:
			lastPacket = time.Now()
			payloads, err := assembler.Reassemble(chunk)
			if err != nil {
				return err
			}
			for _, payload := range payloads {
				packet, err := ParsePacket(payload, desc.XORPayload)
				if err != nil {
					continue
				}
				if isAuthReply(packet) {
					if err := authErrorFromPayload(packet.Payload); err != nil {
						return err
					}
					continue
				}
				if err := processPacket(packet); err != nil {
					return err
				}
			}
		case <-idleTicker.C:
			now := time.Now()
			if staleAfter(lastFresh, staleTimeout, now) && !isStale {
				isStale = true
				slog.Warn("eco-ble stale transition", "device", device.Name, "serial", desc.Serial)
				store.Upsert(device.Name, device.Description, staleVars(device, desc.Model, desc.Serial))
			}
			if now.Sub(lastPacket) >= pollInterval {
				if err := p.writePacket(conn, assembler, authStatusPacket(desc)); err != nil {
					return err
				}
			}
			if now.Sub(lastPacket) >= 2*pollInterval {
				return fmt.Errorf("idle timeout waiting for telemetry")
			}
		}
	}
}

func (p *Provider) bootstrapSession(ctx context.Context, conn BLEConnection, rawCh <-chan []byte, desc DeviceDescriptor, userID string) (FrameAssembler, []Packet, error) {
	switch desc.EncryptType {
	case 0:
		assembler := &PassthroughAssembler{}
		pending, err := p.finishAuthentication(ctx, conn, rawCh, desc, userID, assembler)
		return assembler, pending, err
	case 1:
		sessionKey, iv := type1Session(desc.Serial)
		assembler := &RawHeaderAssembler{encryption: Type1Encryption{SessionKey: sessionKey, IV: iv}}
		pending, err := p.finishAuthentication(ctx, conn, rawCh, desc, userID, assembler)
		return assembler, pending, err
	case 7:
		simple := &SimplePacketAssembler{}
		privateKey, publicKey, err := generateSECP160KeyPair()
		if err != nil {
			return nil, nil, err
		}
		if err := conn.Write(simple.Encode(append([]byte{0x01, 0x00}, publicKey...)), true); err != nil {
			return nil, nil, err
		}
		payload, err := waitForSimplePayload(ctx, rawCh, simple)
		if err != nil {
			return nil, nil, err
		}
		if len(payload) < 4 {
			return nil, nil, fmt.Errorf("invalid device public key response")
		}
		keySize := getECDHTypeSize(payload[2])
		if keySize != 40 {
			return nil, nil, fmt.Errorf("unsupported type7 ECDH public key size %d", keySize)
		}
		sharedSecret, err := deriveSharedSecret(privateKey, payload[3:3+keySize])
		if err != nil {
			return nil, nil, err
		}
		initialKey, iv := type7SessionSeed(sharedSecret)
		tempEnc := Type7Encryption{SessionKey: initialKey, IV: iv}
		if err := conn.Write(simple.Encode([]byte{0x02}), true); err != nil {
			return nil, nil, err
		}
		keyInfo, err := waitForSimplePayload(ctx, rawCh, simple)
		if err != nil {
			return nil, nil, err
		}
		if len(keyInfo) < 17 || keyInfo[0] != 0x02 {
			return nil, nil, fmt.Errorf("invalid key info response")
		}
		decrypted, err := tempEnc.Decrypt(keyInfo[1:])
		if err != nil {
			return nil, nil, err
		}
		if len(decrypted) < 18 {
			return nil, nil, fmt.Errorf("key info payload too short")
		}
		finalKey, err := deriveFinalSessionKey(decrypted[16:18], decrypted[:16])
		if err != nil {
			return nil, nil, err
		}
		assembler := &EncPacketAssembler{encryption: Type7Encryption{SessionKey: finalKey, IV: iv}}
		pending, err := p.finishAuthentication(ctx, conn, rawCh, desc, userID, assembler)
		return assembler, pending, err
	default:
		return nil, nil, fmt.Errorf("unsupported encrypt_type %d", desc.EncryptType)
	}
}

func (p *Provider) finishAuthentication(ctx context.Context, conn BLEConnection, rawCh <-chan []byte, desc DeviceDescriptor, userID string, assembler FrameAssembler) ([]Packet, error) {
	if err := p.writePacket(conn, assembler, authStatusPacket(desc)); err != nil {
		return nil, err
	}
	if err := p.writePacket(conn, assembler, authPacket(desc, userID)); err != nil {
		return nil, err
	}
	timeout := time.NewTimer(time.Duration(p.cfg.Provider.ConnectTimeoutSeconds) * time.Second)
	defer timeout.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timeout.C:
			return nil, fmt.Errorf("authentication timeout")
		case chunk := <-rawCh:
			payloads, err := assembler.Reassemble(chunk)
			if err != nil {
				return nil, err
			}
			var pending []Packet
			for _, payload := range payloads {
				packet, err := ParsePacket(payload, desc.XORPayload)
				if err != nil {
					continue
				}
				if isAuthReply(packet) {
					if err := authErrorFromPayload(packet.Payload); err != nil {
						return nil, err
					}
					return pending, nil
				}
				pending = append(pending, packet)
			}
			if len(pending) > 0 {
				return pending, nil
			}
		}
	}
}

func waitForSimplePayload(ctx context.Context, rawCh <-chan []byte, assembler *SimplePacketAssembler) ([]byte, error) {
	timer := time.NewTimer(20 * time.Second)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timer.C:
			return nil, fmt.Errorf("timeout waiting for handshake response")
		case chunk := <-rawCh:
			if payload, ok := assembler.Parse(chunk); ok {
				return payload, nil
			}
		}
	}
}

func (p *Provider) writePacket(conn BLEConnection, assembler FrameAssembler, packet Packet) error {
	data, err := assembler.Encode(packet)
	if err != nil {
		return err
	}
	return conn.Write(data, assembler.WriteWithResponse())
}

func authStatusPacket(desc DeviceDescriptor) Packet {
	return Packet{
		Src:     0x21,
		Dst:     desc.AuthHeaderDst,
		CmdSet:  0x35,
		CmdID:   0x89,
		DSrc:    0x01,
		DDst:    0x01,
		Version: desc.PacketVersion,
	}
}

func authPacket(desc DeviceDescriptor, userID string) Packet {
	sum := md5.Sum([]byte(userID + desc.Serial))
	payload := strings.ToUpper(hex.EncodeToString(sum[:]))
	return Packet{
		Src:     0x21,
		Dst:     desc.AuthHeaderDst,
		CmdSet:  0x35,
		CmdID:   0x86,
		Payload: []byte(payload),
		DSrc:    0x01,
		DDst:    0x01,
		Version: desc.PacketVersion,
	}
}

func isAuthReply(packet Packet) bool {
	return packet.Src == 0x35 && packet.CmdSet == 0x35 && packet.CmdID == 0x86
}

func authErrorFromPayload(payload []byte) error {
	if len(payload) == 0 || payload[0] == 0x00 {
		return nil
	}
	switch payload[0] {
	case 0x01:
		return fmt.Errorf("authentication failed: need refresh token")
	case 0x02:
		return fmt.Errorf("authentication failed: device internal error")
	case 0x03:
		return fmt.Errorf("authentication failed: device already bound")
	case 0x04:
		return fmt.Errorf("authentication failed: need bind/install first")
	case 0x05:
		return fmt.Errorf("authentication failed: app send data error")
	case 0x06:
		return fmt.Errorf("authentication failed: wrong key")
	case 0x07:
		return fmt.Errorf("authentication failed: maximum devices reached")
	default:
		return fmt.Errorf("authentication failed: unknown error 0x%02x", payload[0])
	}
}

func liveVars(device config.Device, desc DeviceDescriptor, telemetry Telemetry) map[string]string {
	status := "OB"
	if telemetry.InputPresent != nil && *telemetry.InputPresent {
		status = "OL"
	}
	if lowBattery(device, telemetry) {
		status += " LB"
	}
	vars := identityVars(device, desc.Model, desc.Serial, status)
	if telemetry.Charge != nil {
		vars["battery.charge"] = state.FormatInt(*telemetry.Charge)
	}
	if telemetry.RuntimeSeconds != nil {
		vars["battery.runtime"] = state.FormatInt(*telemetry.RuntimeSeconds)
	}
	if telemetry.InputPower != nil {
		vars["input.power"] = state.FormatInt(*telemetry.InputPower)
	}
	if telemetry.OutputPower != nil {
		vars["output.power"] = state.FormatInt(*telemetry.OutputPower)
	}
	return vars
}

func staleVars(device config.Device, model string, serial string) map[string]string {
	return identityVars(device, model, serial, "WAIT")
}

func identityVars(device config.Device, fallbackModel string, serial string, status string) map[string]string {
	model := device.Model
	if model == "" {
		model = fallbackModel
	}
	if model == "" {
		model = "EcoFlow"
	}
	if serial == "" {
		serial = device.Serial
	}
	if serial == "" {
		serial = device.MAC
	}
	return map[string]string{
		"device.mfr":            "EcoFlow",
		"device.model":          model,
		"device.serial":         serial,
		"ups.mfr":               "EcoFlow",
		"ups.model":             model,
		"ups.serial":            serial,
		"ups.status":            status,
		"ups.realpower.nominal": "0",
	}
}

func lowBattery(device config.Device, telemetry Telemetry) bool {
	if telemetry.Charge != nil && *telemetry.Charge <= device.LowBatteryPercent {
		return true
	}
	if telemetry.RuntimeSeconds != nil && *telemetry.RuntimeSeconds <= device.LowRuntimeSeconds {
		return true
	}
	return false
}
