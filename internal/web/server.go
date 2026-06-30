package web

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"html/template"
	"log/slog"
	"net/http"
	"sort"
	"time"

	"github.com/xkhronoz/ecoflow-ble-nutd/internal/config"
	"github.com/xkhronoz/ecoflow-ble-nutd/internal/runtime"
	"github.com/xkhronoz/ecoflow-ble-nutd/internal/state"
)

type Controller interface {
	Enable()
	Disable()
	Status() runtime.ProviderStatus
}

type Server struct {
	cfg        *config.Config
	store      *state.Store
	controller Controller
	page       *template.Template
}

type statusResponse struct {
	Daemon   daemonInfo             `json:"daemon"`
	Provider runtime.ProviderStatus `json:"provider"`
	Devices  []deviceResponse       `json:"devices"`
}

type daemonInfo struct {
	NUTListen string `json:"nut_listen"`
	WebListen string `json:"web_listen"`
}

type deviceResponse struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	UpdatedAt   time.Time         `json:"updated_at"`
	AgeSeconds  int64             `json:"age_seconds"`
	Vars        map[string]string `json:"vars"`
}

type pageData struct {
	Title      string
	Provider   runtime.ProviderStatus
	Daemon     daemonInfo
	Devices    []pageDevice
	ControlURL string
}

type pageDevice struct {
	Name        string
	Description string
	UpdatedAt   time.Time
	AgeSeconds  int64
	Vars        []pageVar
}

type pageVar struct {
	Key   string
	Value string
}

func New(cfg *config.Config, store *state.Store, controller Controller) *Server {
	return &Server{
		cfg:        cfg,
		store:      store,
		controller: controller,
		page:       template.Must(template.New("status").Parse(statusPage)),
	}
}

func (s *Server) Run(ctx context.Context) error {
	if !s.cfg.Web.Enable {
		return nil
	}
	if s.cfg.Web.Listen == "" {
		return nil
	}
	srv := &http.Server{
		Addr:    s.cfg.Web.Listen,
		Handler: s.routes(),
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()
	slog.Info("web server listening", "addr", s.cfg.Web.Listen)
	err := srv.ListenAndServe()
	if err == nil || err == http.ErrServerClosed {
		return nil
	}
	return err
}

func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", s.handlePage)
	mux.HandleFunc("GET /api/status", s.handleStatus)
	mux.HandleFunc("POST /api/provider/enable", s.handleEnable)
	mux.HandleFunc("POST /api/provider/disable", s.handleDisable)
	mux.HandleFunc("POST /api/ble/enable", s.handleEnable)
	mux.HandleFunc("POST /api/ble/disable", s.handleDisable)
	return s.protect(mux)
}

func (s *Server) protect(next http.Handler) http.Handler {
	if s.cfg.Auth.Username == "" && s.cfg.Auth.Password == "" {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username, password, ok := r.BasicAuth()
		if !ok || subtle.ConstantTimeCompare([]byte(username), []byte(s.cfg.Auth.Username)) != 1 || subtle.ConstantTimeCompare([]byte(password), []byte(s.cfg.Auth.Password)) != 1 {
			w.Header().Set("WWW-Authenticate", `Basic realm="ecoflow-ble-nutd"`)
			http.Error(w, "authentication required", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) handlePage(w http.ResponseWriter, r *http.Request) {
	data := pageData{
		Title:      controlTitle(s.cfg.Provider.Type),
		Provider:   s.controller.Status(),
		Daemon:     daemonInfo{NUTListen: s.cfg.Listen, WebListen: s.cfg.Web.Listen},
		Devices:    s.pageDevices(),
		ControlURL: controlNote(s.cfg.Provider.Type),
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.page.Execute(w, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.snapshot())
}

func (s *Server) handleEnable(w http.ResponseWriter, r *http.Request) {
	s.controller.Enable()
	writeJSON(w, http.StatusOK, s.snapshot())
}

func (s *Server) handleDisable(w http.ResponseWriter, r *http.Request) {
	s.controller.Disable()
	writeJSON(w, http.StatusOK, s.snapshot())
}

func (s *Server) snapshot() statusResponse {
	return statusResponse{
		Daemon: daemonInfo{
			NUTListen: s.cfg.Listen,
			WebListen: s.cfg.Web.Listen,
		},
		Provider: s.controller.Status(),
		Devices:  s.devices(),
	}
}

func (s *Server) devices() []deviceResponse {
	now := time.Now()
	snapshot := s.store.Snapshot()
	out := make([]deviceResponse, 0, len(snapshot))
	for _, ups := range snapshot {
		out = append(out, deviceResponse{
			Name:        ups.Name,
			Description: ups.Description,
			UpdatedAt:   ups.UpdatedAt,
			AgeSeconds:  int64(now.Sub(ups.UpdatedAt).Seconds()),
			Vars:        ups.Vars,
		})
	}
	return out
}

func (s *Server) pageDevices() []pageDevice {
	now := time.Now()
	snapshot := s.store.Snapshot()
	out := make([]pageDevice, 0, len(snapshot))
	for _, ups := range snapshot {
		keys := make([]string, 0, len(ups.Vars))
		for key := range ups.Vars {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		vars := make([]pageVar, 0, len(keys))
		for _, key := range keys {
			vars = append(vars, pageVar{Key: key, Value: ups.Vars[key]})
		}
		out = append(out, pageDevice{
			Name:        ups.Name,
			Description: ups.Description,
			UpdatedAt:   ups.UpdatedAt,
			AgeSeconds:  int64(now.Sub(ups.UpdatedAt).Seconds()),
			Vars:        vars,
		})
	}
	return out
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(payload)
}

func controlTitle(providerType string) string {
	if providerType == "eco-ble" {
		return "BLE Collector Control"
	}
	return "Provider Control"
}

func controlNote(providerType string) string {
	if providerType == "eco-ble" {
		return "Disabling the BLE collector pauses this daemon's BLE session so another local tool can connect. It does not change the device-side Bluetooth setting."
	}
	return "Disabling the provider pauses data collection in this daemon."
}

const statusPage = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>ecoflow-ble-nutd status</title>
  <style>
    :root {
      color-scheme: light;
      --bg: #f4f1ea;
      --panel: #fffaf2;
      --ink: #17212b;
      --muted: #586472;
      --accent: #1e6f5c;
      --accent-strong: #154f42;
      --line: #d9d1c4;
      --warn: #8a4b00;
    }
    body {
      margin: 0;
      font-family: "Iowan Old Style", "Palatino Linotype", serif;
      background:
        radial-gradient(circle at top right, rgba(30, 111, 92, 0.15), transparent 30%),
        linear-gradient(180deg, #faf6ee 0%, var(--bg) 100%);
      color: var(--ink);
    }
    main {
      max-width: 980px;
      margin: 0 auto;
      padding: 24px 16px 48px;
    }
    .hero, .card {
      background: rgba(255, 250, 242, 0.92);
      border: 1px solid var(--line);
      border-radius: 18px;
      box-shadow: 0 10px 30px rgba(23, 33, 43, 0.08);
    }
    .hero {
      padding: 24px;
      margin-bottom: 16px;
    }
    .hero h1 {
      margin: 0 0 8px;
      font-size: clamp(2rem, 4vw, 3rem);
      line-height: 1;
    }
    .hero p {
      margin: 8px 0;
      color: var(--muted);
      max-width: 60rem;
    }
    .controls {
      display: flex;
      gap: 12px;
      flex-wrap: wrap;
      align-items: center;
      margin-top: 18px;
    }
    button, .link {
      border: 0;
      border-radius: 999px;
      padding: 10px 16px;
      font: inherit;
      cursor: pointer;
      text-decoration: none;
      color: white;
      background: var(--accent);
    }
    button.secondary, .link.secondary {
      background: #44515f;
    }
    button:hover, .link:hover {
      background: var(--accent-strong);
    }
    .grid {
      display: grid;
      grid-template-columns: repeat(auto-fit, minmax(260px, 1fr));
      gap: 16px;
      margin-bottom: 16px;
    }
    .card {
      padding: 18px;
    }
    dl {
      margin: 0;
      display: grid;
      grid-template-columns: max-content 1fr;
      gap: 8px 12px;
    }
    dt {
      color: var(--muted);
    }
    dd {
      margin: 0;
      word-break: break-word;
    }
    .device {
      margin-top: 16px;
    }
    .device h2 {
      margin: 0 0 4px;
      font-size: 1.35rem;
    }
    .meta {
      color: var(--muted);
      font-size: 0.95rem;
      margin-bottom: 12px;
    }
    table {
      width: 100%;
      border-collapse: collapse;
      font-family: ui-monospace, "SFMono-Regular", monospace;
      font-size: 0.92rem;
    }
    th, td {
      text-align: left;
      padding: 8px 0;
      border-top: 1px solid var(--line);
      vertical-align: top;
    }
    th {
      color: var(--muted);
      width: 40%;
      font-weight: 600;
    }
    .warning {
      color: var(--warn);
    }
    @media (max-width: 640px) {
      .hero, .card {
        border-radius: 14px;
      }
      table, tbody, tr, th, td {
        display: block;
      }
      th {
        width: auto;
        border-top: 1px solid var(--line);
        padding-bottom: 4px;
      }
      td {
        padding-top: 0;
      }
    }
  </style>
</head>
<body>
  <main>
    <section class="hero">
      <h1>{{ .Title }}</h1>
      <p>{{ .ControlURL }}</p>
      <div class="controls">
        <form method="post" action="/api/ble/enable">
          <button type="submit">Enable</button>
        </form>
        <form method="post" action="/api/ble/disable">
          <button class="secondary" type="submit">Disable</button>
        </form>
        <a class="link secondary" href="/api/status">JSON status</a>
      </div>
    </section>
    <section class="grid">
      <article class="card">
        <h2>Collector</h2>
        <dl>
          <dt>Type</dt><dd>{{ .Provider.Type }}</dd>
          <dt>Enabled</dt><dd>{{ .Provider.Enabled }}</dd>
          <dt>Running</dt><dd>{{ .Provider.Running }}</dd>
          <dt>State</dt><dd>{{ .Provider.State }}</dd>
          <dt>Changed</dt><dd>{{ .Provider.UpdatedAt.Format "2006-01-02 15:04:05 MST" }}</dd>
          {{ if .Provider.LastError }}
          <dt class="warning">Last error</dt><dd class="warning">{{ .Provider.LastError }}</dd>
          {{ end }}
        </dl>
      </article>
      <article class="card">
        <h2>Daemon</h2>
        <dl>
          <dt>NUT listen</dt><dd>{{ .Daemon.NUTListen }}</dd>
          <dt>Web listen</dt><dd>{{ .Daemon.WebListen }}</dd>
        </dl>
      </article>
    </section>
    {{ range .Devices }}
    <section class="card device">
      <h2>{{ .Name }}</h2>
      <div class="meta">{{ .Description }} · updated {{ .AgeSeconds }}s ago at {{ .UpdatedAt.Format "2006-01-02 15:04:05 MST" }}</div>
      <table>
        <tbody>
          {{ range .Vars }}
          <tr>
            <th>{{ .Key }}</th>
            <td>{{ .Value }}</td>
          </tr>
          {{ end }}
        </tbody>
      </table>
    </section>
    {{ end }}
  </main>
</body>
</html>
`
