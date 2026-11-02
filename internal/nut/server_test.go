package nut

import (
	"bufio"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/xkhronoz/ecoflow-ble-nutd/internal/config"
	"github.com/xkhronoz/ecoflow-ble-nutd/internal/state"
)

func newTestServer() *Server {
	store := state.New()
	store.Upsert("river3", "EcoFlow River 3", map[string]string{
		"battery.charge": "75",
		"ups.status":     "OL",
	})
	return New(&config.Config{
		Auth: config.AuthConfig{Username: "monuser", Password: "secret"},
	}, store)
}

func TestReadCommandsWithoutAuthentication(t *testing.T) {
	t.Parallel()

	server := newTestServer()
	for _, tc := range []struct {
		command string
		want    string
	}{
		{"VER", "Network UPS Tools ecoflow-ble-nutd 0.1\n"},
		{"PING", "PONG\n"},
		{"LIST UPS", "BEGIN LIST UPS\nUPS river3 \"EcoFlow River 3\"\nEND LIST UPS\n"},
		{"LIST VAR river3", "BEGIN LIST VAR river3\nVAR river3 battery.charge \"75\"\nVAR river3 ups.status \"OL\"\nEND LIST VAR river3\n"},
		{"GET VAR river3 ups.status", "VAR river3 ups.status \"OL\"\n"},
		{"GET UPSDESC river3", "UPSDESC river3 \"EcoFlow River 3\"\n"},
		{"GET NUMLOGINS river3", "NUMLOGINS river3 1\n"},
		{"GET VAR missing ups.status", "ERR UNKNOWN-UPS\n"},
		{"GET VAR river3 missing", "ERR VAR-NOT-SUPPORTED\n"},
		{"GET VAR river3", "ERR INVALID-ARGUMENT\n"},
		{"LIST VAR missing", "ERR UNKNOWN-UPS\n"},
		{"SET VAR river3 ups.status OB", "ERR UNKNOWN-COMMAND\n"},
	} {
		t.Run(tc.command, func(t *testing.T) {
			authed := false
			got, closeConn := server.command(tc.command, &authed)
			if got != tc.want || closeConn {
				t.Fatalf("response = %q, close = %t; want %q, false", got, closeConn, tc.want)
			}
			if authed {
				t.Fatal("read command authenticated the client")
			}
		})
	}
}

func TestClientSessionAuthentication(t *testing.T) {
	t.Parallel()

	serverConn, clientConn := net.Pipe()
	defer clientConn.Close()
	if err := clientConn.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	go newTestServer().handle(serverConn)
	reader := bufio.NewReader(clientConn)

	for _, tc := range []struct {
		command string
		want    string
	}{
		{"LIST UPS", "BEGIN LIST UPS\nUPS river3 \"EcoFlow River 3\"\nEND LIST UPS\n"},
		{"LIST VAR river3", "BEGIN LIST VAR river3\nVAR river3 battery.charge \"75\"\nVAR river3 ups.status \"OL\"\nEND LIST VAR river3\n"},
		{"LOGIN river3", "ERR ACCESS-DENIED\n"},
		{"PRIMARY river3", "ERR ACCESS-DENIED\n"},
		{"MASTER river3", "ERR ACCESS-DENIED\n"},
		{"FSD river3", "ERR ACCESS-DENIED\n"},
		{"USERNAME monuser", "OK\n"},
		{"PASSWORD wrong", "ERR ACCESS-DENIED\n"},
		{"FSD river3", "ERR ACCESS-DENIED\n"},
		{"GET VAR river3 ups.status", "VAR river3 ups.status \"OL\"\n"},
		{"PASSWORD secret", "OK\n"},
		{"LOGIN river3", "OK\n"},
		{"PRIMARY river3", "OK\n"},
		{"MASTER river3", "OK\n"},
		{"FSD river3", "OK\n"},
	} {
		if _, err := io.WriteString(clientConn, tc.command+"\n"); err != nil {
			t.Fatalf("%s: write: %v", tc.command, err)
		}
		for _, want := range strings.SplitAfter(tc.want, "\n") {
			if want == "" {
				continue
			}
			got, err := reader.ReadString('\n')
			if err != nil {
				t.Fatalf("%s: read: %v", tc.command, err)
			}
			if got != want {
				t.Fatalf("%s: response line = %q; want %q", tc.command, got, want)
			}
		}
	}
	if _, err := io.WriteString(clientConn, "LOGOUT\n"); err != nil {
		t.Fatal(err)
	}
	if _, err := reader.ReadByte(); err != io.EOF {
		t.Fatalf("read after LOGOUT = %v; want EOF", err)
	}
}
