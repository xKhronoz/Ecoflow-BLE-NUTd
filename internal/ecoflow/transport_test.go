package ecoflow

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

type trackedConnection struct {
	fakeConn
	closed chan struct{}
}

func (c *trackedConnection) Close() error {
	close(c.closed)
	return nil
}

func TestConnectWithTimeoutClosesLateConnections(t *testing.T) {
	for _, mode := range []string{"timeout", "cancel"} {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			conn := &trackedConnection{closed: make(chan struct{})}
			release := make(chan struct{})
			started := make(chan struct{})
			done := make(chan error, 1)
			timeout := time.Second
			if mode == "timeout" {
				timeout = 20 * time.Millisecond
			}
			go func() {
				_, err := connectWithTimeout(ctx, "AA:BB:CC:DD:EE:FF", timeout, func() (BLEConnection, error) {
					close(started)
					<-release
					return conn, nil
				})
				done <- err
			}()
			<-started
			if mode == "cancel" {
				cancel()
			}
			select {
			case err := <-done:
				if mode == "timeout" && (err == nil || !strings.Contains(err.Error(), "connect timeout")) {
					t.Fatalf("expected timeout, got %v", err)
				}
				if mode == "cancel" && !errors.Is(err, context.Canceled) {
					t.Fatalf("expected cancellation, got %v", err)
				}
			case <-time.After(time.Second):
				t.Fatal("connection attempt did not return")
			}
			close(release)
			select {
			case <-conn.closed:
			case <-time.After(time.Second):
				t.Fatal("late connection was not closed")
			}
		})
	}
}

func TestConnectWithTimeoutHandsOffSuccessfulConnection(t *testing.T) {
	conn := &trackedConnection{closed: make(chan struct{})}
	got, err := connectWithTimeout(context.Background(), "AA:BB:CC:DD:EE:FF", time.Second, func() (BLEConnection, error) {
		return conn, nil
	})
	if err != nil || got != conn {
		t.Fatalf("connection = %v, error = %v", got, err)
	}
	select {
	case <-conn.closed:
		t.Fatal("successful connection was closed during handoff")
	default:
	}
	_ = got.Close()
}
