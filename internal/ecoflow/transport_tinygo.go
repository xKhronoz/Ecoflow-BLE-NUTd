package ecoflow

import (
	"context"
	"fmt"
	"strings"
	"time"

	"tinygo.org/x/bluetooth"
)

type TinygoTransport struct{}

func (t *TinygoTransport) FindByMAC(ctx context.Context, adapterID string, mac string, timeout time.Duration) (Discovery, error) {
	adapter := newBLEAdapter(adapterID)
	if err := adapter.Enable(); err != nil {
		return Discovery{}, err
	}
	want := strings.ToUpper(mac)
	matches := make(chan Discovery, 1)
	errCh := make(chan error, 1)
	go func() {
		errCh <- adapter.Scan(func(_ *bluetooth.Adapter, result bluetooth.ScanResult) {
			if strings.ToUpper(result.Address.String()) != want {
				return
			}
			md := make([]ManufacturerData, 0, len(result.ManufacturerData()))
			for _, element := range result.ManufacturerData() {
				md = append(md, ManufacturerData{
					CompanyID: element.CompanyID,
					Data:      append([]byte(nil), element.Data...),
				})
			}
			select {
			case matches <- Discovery{
				MAC:              strings.ToUpper(result.Address.String()),
				LocalName:        result.LocalName(),
				ManufacturerData: md,
			}:
			default:
			}
			_ = adapter.StopScan()
		})
	}()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		_ = adapter.StopScan()
		return Discovery{}, ctx.Err()
	case match := <-matches:
		return match, nil
	case err := <-errCh:
		if err != nil {
			return Discovery{}, err
		}
		return Discovery{}, fmt.Errorf("scan ended before finding %s", want)
	case <-timer.C:
		_ = adapter.StopScan()
		return Discovery{}, fmt.Errorf("scan timeout waiting for %s", want)
	}
}

func (t *TinygoTransport) Connect(ctx context.Context, adapterID string, discovery Discovery, timeout time.Duration) (BLEConnection, error) {
	adapter := newBLEAdapter(adapterID)
	if err := adapter.Enable(); err != nil {
		return nil, err
	}
	var addr bluetooth.Address
	addr.Set(discovery.MAC)
	type result struct {
		conn BLEConnection
		err  error
	}
	ch := make(chan result, 1)
	go func() {
		device, err := adapter.Connect(addr, bluetooth.ConnectionParams{})
		if err != nil {
			ch <- result{err: err}
			return
		}
		conn, err := newTinygoConnection(device)
		ch <- result{conn: conn, err: err}
	}()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case result := <-ch:
		return result.conn, result.err
	case <-timer.C:
		return nil, fmt.Errorf("connect timeout for %s", discovery.MAC)
	}
}

type tinygoConnection struct {
	device bluetooth.Device
	write  bluetooth.DeviceCharacteristic
	notify *bluetooth.DeviceCharacteristic
}

func newTinygoConnection(device bluetooth.Device) (*tinygoConnection, error) {
	services, err := device.DiscoverServices(nil)
	if err != nil {
		_ = device.Disconnect()
		return nil, err
	}
	var writeChar bluetooth.DeviceCharacteristic
	var notifyChar *bluetooth.DeviceCharacteristic
	for _, service := range services {
		chars, err := service.DiscoverCharacteristics(nil)
		if err != nil {
			continue
		}
		var rfcommNotify, rfcommWrite *bluetooth.DeviceCharacteristic
		var nusNotify, nusWrite *bluetooth.DeviceCharacteristic
		for i := range chars {
			uuid := strings.ToLower(chars[i].UUID().String())
			switch uuid {
			case "00000003-0000-1000-8000-00805f9b34fb":
				rfcommNotify = &chars[i]
			case "00000002-0000-1000-8000-00805f9b34fb":
				rfcommWrite = &chars[i]
			case "6e400003-b5a3-f393-e0a9-e50e24dcca9e":
				nusNotify = &chars[i]
			case "6e400002-b5a3-f393-e0a9-e50e24dcca9e":
				nusWrite = &chars[i]
			}
		}
		if rfcommNotify != nil && rfcommWrite != nil {
			writeChar = *rfcommWrite
			notifyChar = rfcommNotify
			break
		}
		if nusNotify != nil && nusWrite != nil {
			writeChar = *nusWrite
			notifyChar = nusNotify
			break
		}
	}
	if notifyChar == nil {
		_ = device.Disconnect()
		return nil, fmt.Errorf("no supported EcoFlow notify/write characteristic pair found")
	}
	return &tinygoConnection{
		device: device,
		write:  writeChar,
		notify: notifyChar,
	}, nil
}

func (c *tinygoConnection) StartNotifications(handler func([]byte)) error {
	return c.notify.EnableNotifications(func(buf []byte) {
		if len(buf) == 0 {
			return
		}
		cp := append([]byte(nil), buf...)
		handler(cp)
	})
}

func (c *tinygoConnection) Write(data []byte, withResponse bool) error {
	if withResponse {
		_, err := c.write.Write(data)
		return err
	}
	if _, err := c.write.WriteWithoutResponse(data); err == nil {
		return nil
	}
	_, err := c.write.Write(data)
	return err
}

func (c *tinygoConnection) Close() error {
	return c.device.Disconnect()
}
