//go:build linux

package ecoflow

import "tinygo.org/x/bluetooth"

func newBLEAdapter(id string) *bluetooth.Adapter {
	return bluetooth.NewAdapter(id)
}
