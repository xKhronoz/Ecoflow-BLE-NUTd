package ecoflow

import (
	"context"
	"time"
)

type Transport interface {
	FindByMAC(ctx context.Context, adapterID string, mac string, timeout time.Duration) (Discovery, error)
	Connect(ctx context.Context, adapterID string, discovery Discovery, timeout time.Duration) (BLEConnection, error)
}

type BLEConnection interface {
	StartNotifications(func([]byte)) error
	Write(data []byte, withResponse bool) error
	Close() error
}
