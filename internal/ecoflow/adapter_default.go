//go:build !linux

package ecoflow

import (
	"sync"

	"tinygo.org/x/bluetooth"
)

// The default adapter is a process-wide singleton. In particular, macOS
// rejects a second Enable call even after the first call has completed.
var enabledDefaultAdapter = sync.OnceValues(func() (*bluetooth.Adapter, error) {
	adapter := bluetooth.DefaultAdapter
	return adapter, adapter.Enable()
})

func enabledBLEAdapter(_ string) (*bluetooth.Adapter, error) {
	return enabledDefaultAdapter()
}
