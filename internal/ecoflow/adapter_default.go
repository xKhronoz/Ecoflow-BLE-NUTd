//go:build !linux

package ecoflow

import "tinygo.org/x/bluetooth"

func newBLEAdapter(_ string) *bluetooth.Adapter {
	return bluetooth.DefaultAdapter
}
