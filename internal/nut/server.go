package nut

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"sort"
	"strings"

	"github.com/xkhronoz/ecoflow-ble-nutd/internal/config"
	"github.com/xkhronoz/ecoflow-ble-nutd/internal/state"
)

type Server struct {
	cfg   *config.Config
	store *state.Store
}

func New(cfg *config.Config, store *state.Store) *Server { return &Server{cfg: cfg, store: store} }

func (s *Server) Run(ctx context.Context) error {
	ln, err := net.Listen("tcp", s.cfg.Listen)
	if err != nil {
		return err
	}
	defer ln.Close()
	slog.Info("NUT server listening", "addr", s.cfg.Listen)
	go func() { <-ctx.Done(); _ = ln.Close() }()
	for {
		c, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		go s.handle(c)
	}
}

func (s *Server) handle(c net.Conn) {
	defer c.Close()
	authed := s.cfg.Auth.Username == "" && s.cfg.Auth.Password == ""
	r := bufio.NewReader(c)
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.EqualFold(line, "LOGOUT") || strings.EqualFold(line, "QUIT") {
			return
		}
		resp, closeConn := s.command(line, &authed)
		if resp != "" {
			_, _ = io.WriteString(c, resp)
		}
		if closeConn {
			return
		}
	}
}

func quote(v string) string { return `"` + strings.ReplaceAll(v, `"`, `\"`) + `"` }

func (s *Server) command(line string, authed *bool) (string, bool) {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return "", false
	}
	cmd := strings.ToUpper(fields[0])

	switch cmd {
	case "VER":
		return "Network UPS Tools ecoflow-ble-nutd 0.1\n", false
	case "USERNAME":
		if len(fields) < 2 {
			return "ERR INVALID-ARGUMENT\n", false
		}
		if s.cfg.Auth.Username == "" || fields[1] == s.cfg.Auth.Username {
			return "OK\n", false
		}
		return "ERR ACCESS-DENIED\n", false
	case "PASSWORD":
		if len(fields) < 2 {
			return "ERR INVALID-ARGUMENT\n", false
		}
		if s.cfg.Auth.Password == "" || fields[1] == s.cfg.Auth.Password {
			*authed = true
			return "OK\n", false
		}
		return "ERR ACCESS-DENIED\n", false
	case "LOGIN", "PRIMARY", "MASTER", "FSD":
		if !*authed {
			return "ERR ACCESS-DENIED\n", false
		}
		return "OK\n", false
	}

	if !*authed {
		return "ERR ACCESS-DENIED\n", false
	}

	switch cmd {
	case "LIST":
		if len(fields) < 2 {
			return "ERR INVALID-ARGUMENT\n", false
		}
		switch strings.ToUpper(fields[1]) {
		case "UPS":
			var b strings.Builder
			b.WriteString("BEGIN LIST UPS\n")
			for _, name := range s.store.Names() {
				u, _ := s.store.Get(name)
				b.WriteString(fmt.Sprintf("UPS %s %s\n", u.Name, quote(u.Description)))
			}
			b.WriteString("END LIST UPS\n")
			return b.String(), false
		case "VAR":
			if len(fields) < 3 {
				return "ERR INVALID-ARGUMENT\n", false
			}
			u, ok := s.store.Get(fields[2])
			if !ok {
				return "ERR UNKNOWN-UPS\n", false
			}
			keys := make([]string, 0, len(u.Vars))
			for k := range u.Vars {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			var b strings.Builder
			b.WriteString(fmt.Sprintf("BEGIN LIST VAR %s\n", u.Name))
			for _, k := range keys {
				b.WriteString(fmt.Sprintf("VAR %s %s %s\n", u.Name, k, quote(u.Vars[k])))
			}
			b.WriteString(fmt.Sprintf("END LIST VAR %s\n", u.Name))
			return b.String(), false
		default:
			return "ERR UNKNOWN-COMMAND\n", false
		}
	case "GET":
		if len(fields) < 3 {
			return "ERR INVALID-ARGUMENT\n", false
		}
		switch strings.ToUpper(fields[1]) {
		case "UPSDESC":
			u, ok := s.store.Get(fields[2])
			if !ok {
				return "ERR UNKNOWN-UPS\n", false
			}
			return fmt.Sprintf("UPSDESC %s %s\n", u.Name, quote(u.Description)), false
		case "VAR":
			if len(fields) < 4 {
				return "ERR INVALID-ARGUMENT\n", false
			}
			u, ok := s.store.Get(fields[2])
			if !ok {
				return "ERR UNKNOWN-UPS\n", false
			}
			v, ok := u.Vars[fields[3]]
			if !ok {
				return "ERR VAR-NOT-SUPPORTED\n", false
			}
			return fmt.Sprintf("VAR %s %s %s\n", u.Name, fields[3], quote(v)), false
		case "NUMLOGINS":
			return fmt.Sprintf("NUMLOGINS %s 1\n", fields[2]), false
		default:
			return "ERR UNKNOWN-COMMAND\n", false
		}
	case "PING":
		return "PONG\n", false
	default:
		return "ERR UNKNOWN-COMMAND\n", false
	}
}
