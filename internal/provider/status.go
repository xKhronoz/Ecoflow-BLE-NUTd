package provider

import (
	"github.com/xkhronoz/ecoflow-ble-nutd/internal/config"
	"github.com/xkhronoz/ecoflow-ble-nutd/internal/state"
)

func MarkDevicesWaiting(store *state.Store, devices []config.Device) {
	for _, device := range devices {
		store.Upsert(device.Name, device.Description, baseVars(device, "WAIT"))
	}
}
