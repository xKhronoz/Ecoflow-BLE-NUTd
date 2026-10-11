//go:build linux

package ecoflow

import "tinygo.org/x/bluetooth"

func enabledBLEAdapter(id string) (*bluetooth.Adapter, error) {
	adapter := bluetooth.NewAdapter(id)
	return adapter, adapter.Enable()
}
