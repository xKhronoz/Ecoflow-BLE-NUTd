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
	mux.HandleFunc("GET /assets/logo.svg", serveLogo)
	mux.HandleFunc("GET /favicon.svg", serveLogo)
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
			AgeSeconds:  int64(now.Sub(ups.UpdatedAt).Round(time.Second) / time.Second),
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
			AgeSeconds:  int64(now.Sub(ups.UpdatedAt).Round(time.Second) / time.Second),
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
		return "EcoFlow BLE Collector Control"
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
  <title>Ecoflow-BLE-NUTd Status</title>
  <link rel="icon" type="image/svg+xml" href="/favicon.svg">
  <style>
    :root {
      color-scheme: light;
      --bg: #f4f1ea;
      --panel: #fffaf2;
      --ink: #17212b;
      --muted: #586472;
      --accent: #1e6f5c;
      --accent-strong: #154f42;
      --danger: #b33b36;
      --danger-strong: #8a2322;
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
    .brand {
      display: flex;
      align-items: center;
      gap: 16px;
      margin-bottom: 12px;
    }
    .brand-logo {
      width: 64px;
      height: 64px;
      flex: none;
    }
    .hero h1 {
      margin: 0;
      min-width: 0;
      font-size: clamp(2rem, 4vw, 3rem);
      line-height: 1.1;
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
    button.danger {
      background: var(--danger);
    }
    button.danger:hover {
      background: var(--danger-strong);
    }
    button:focus-visible, .link:focus-visible {
      outline: 3px solid var(--accent);
      outline-offset: 3px;
    }
    button:disabled {
      cursor: wait;
      opacity: 0.65;
    }
    .control-message {
      min-height: 1.4em;
      font-weight: 600;
    }
    .hero .control-message[data-kind="success"] {
      color: var(--accent-strong);
    }
    .hero .control-message[data-kind="error"] {
      color: var(--danger-strong);
    }
    .status-badge {
      display: inline-block;
      border-radius: 6px;
      padding: 2px 8px;
      font-weight: 600;
      background: #e7ebee;
      color: #44515f;
    }
    .status-badge[data-value="true"], .status-badge[data-state="running"] {
      background: #e3f1e9;
      color: #155640;
    }
    .status-badge[data-value="false"],
    .status-badge[data-state="disabled"], .status-badge[data-state="stopped"] {
      background: #fbe7e5;
      color: #8b2925;
    }
    .status-badge[data-state="starting"], .status-badge[data-state="stopping"],
    .status-badge[data-state="retrying"] {
      background: #fff0cf;
      color: #7a4806;
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
    .card h2 {
      margin: 0 0 12px;
      font-size: 1.35rem;
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
      .brand {
        align-items: flex-start;
        gap: 12px;
      }
      .brand-logo {
        width: 48px;
        height: 48px;
      }
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
      <div class="brand">
        <img class="brand-logo" src="/assets/logo.svg" width="64" height="64" alt="">
        <h1>{{ .Title }}</h1>
      </div>
      <p>{{ .ControlURL }}</p>
      <div class="controls" id="collector-controls" aria-busy="false">
        <button type="button" data-action="enable">Enable</button>
        <button class="danger" type="button" data-action="disable">Disable</button>
        <a class="link secondary" href="/api/status">JSON Status</a>
      </div>
      <p class="control-message" id="control-message" role="status" aria-live="polite" aria-atomic="true"></p>
      <noscript><p class="warning">Enable JavaScript to use the collector controls and live status updates.</p></noscript>
    </section>
    <section class="grid">
      <article class="card">
        <h2>Collector</h2>
        <dl>
          <dt>Type</dt><dd id="collector-type">{{ .Provider.Type }}</dd>
          <dt>Enabled</dt><dd><span class="status-badge" id="collector-enabled" data-value="{{ .Provider.Enabled }}">{{ .Provider.Enabled }}</span></dd>
          <dt>Running</dt><dd><span class="status-badge" id="collector-running" data-value="{{ .Provider.Running }}">{{ .Provider.Running }}</span></dd>
          <dt>State</dt><dd><span class="status-badge" id="collector-state" data-state="{{ .Provider.State }}">{{ .Provider.State }}</span></dd>
          <dt>Changed</dt><dd id="collector-changed">{{ .Provider.UpdatedAt.Format "2006-01-02 15:04:05 MST" }}</dd>
          <dt class="warning" id="collector-error-label" {{ if not .Provider.LastError }}hidden{{ end }}>Last error</dt>
          <dd class="warning" id="collector-error" {{ if not .Provider.LastError }}hidden{{ end }}>{{ .Provider.LastError }}</dd>
        </dl>
      </article>
      <article class="card">
        <h2>Daemon</h2>
        <dl>
          <dt>NUT listen</dt><dd id="nut-listen">{{ .Daemon.NUTListen }}</dd>
          <dt>Web listen</dt><dd id="web-listen">{{ .Daemon.WebListen }}</dd>
        </dl>
      </article>
    </section>
    <div id="devices">
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
    </div>
  </main>
  <script>
    (() => {
      const controls = document.getElementById('collector-controls');
      const buttons = Array.from(controls.querySelectorAll('button[data-action]'));
      const message = document.getElementById('control-message');
      let busy = false;
      let refreshing = false;
      let actionVersion = 0;
      let refreshError = false;

      function showMessage(text, kind) {
        message.textContent = text;
        message.dataset.kind = kind;
      }

      function formatTime(value) {
        return new Date(value).toLocaleString(undefined, { timeZoneName: 'short' });
      }

      function textElement(tag, text, className) {
        const element = document.createElement(tag);
        element.textContent = text;
        if (className) element.className = className;
        return element;
      }

      function updateStatus(status) {
        const provider = status.provider;
        if (!provider || typeof provider.enabled !== 'boolean' ||
            typeof provider.running !== 'boolean' || typeof provider.state !== 'string') {
          throw new Error('The server returned an invalid status. Reload the page to check the collector.');
        }
        document.getElementById('collector-type').textContent = provider.type;
        for (const name of ['enabled', 'running']) {
          const badge = document.getElementById('collector-' + name);
          badge.textContent = String(provider[name]);
          badge.dataset.value = String(provider[name]);
        }
        const state = document.getElementById('collector-state');
        state.textContent = provider.state;
        state.dataset.state = provider.state;
        document.getElementById('collector-changed').textContent = formatTime(provider.updated_at);
        document.getElementById('collector-error-label').hidden = !provider.last_error;
        const error = document.getElementById('collector-error');
        error.hidden = !provider.last_error;
        error.textContent = provider.last_error || '';
        document.getElementById('nut-listen').textContent = status.daemon.nut_listen;
        document.getElementById('web-listen').textContent = status.daemon.web_listen;

        const devices = document.createDocumentFragment();
        for (const device of status.devices || []) {
          const section = textElement('section', '', 'card device');
          section.append(textElement('h2', device.name));
          section.append(textElement('div', device.description + ' · updated ' +
            Math.max(0, device.age_seconds) + 's ago at ' + formatTime(device.updated_at), 'meta'));
          const table = document.createElement('table');
          const body = document.createElement('tbody');
          for (const key of Object.keys(device.vars || {}).sort()) {
            const row = document.createElement('tr');
            row.append(textElement('th', key), textElement('td', device.vars[key]));
            body.append(row);
          }
          table.append(body);
          section.append(table);
          devices.append(section);
        }
        document.getElementById('devices').replaceChildren(devices);
      }

      async function requestStatus(url, method) {
        const controller = new AbortController();
        const timeout = setTimeout(() => controller.abort(), 10000);
        try {
          const response = await fetch(url, {
            method: method,
            credentials: 'same-origin',
            cache: 'no-store',
            headers: { Accept: 'application/json' },
            signal: controller.signal
          });
          if (!response.ok) {
            if (response.status === 401) {
              throw new Error('Authentication required. Reload the page to sign in, then try again.');
            }
            throw new Error('The server could not complete the request (HTTP ' + response.status + '). Try again.');
          }
          return await response.json();
        } finally {
          clearTimeout(timeout);
        }
      }

      function requestError(error) {
        if (error.name === 'AbortError') {
          return 'The request timed out. Status could not be confirmed; retry or reload the page.';
        }
        if (error instanceof TypeError || error instanceof SyntaxError) {
          return 'Status could not be confirmed. Check your connection and retry or reload the page.';
        }
        return error.message;
      }

      async function changeCollector(button) {
        if (busy) return;
        busy = true;
        actionVersion++;
        refreshError = false;
        const action = button.dataset.action;
        const label = button.textContent;
        const hadFocus = document.activeElement === button;
        controls.setAttribute('aria-busy', 'true');
        for (const control of buttons) control.disabled = true;
        button.textContent = action === 'enable' ? 'Enabling…' : 'Disabling…';
        showMessage(button.textContent, 'pending');
        try {
          const status = await requestStatus('/api/ble/' + action, 'POST');
          updateStatus(status);
          showMessage('Collector ' + (status.provider.enabled ? 'enabled.' : 'disabled.'), 'success');
        } catch (error) {
          showMessage(requestError(error), 'error');
        } finally {
          busy = false;
          button.textContent = label;
          for (const control of buttons) control.disabled = false;
          controls.setAttribute('aria-busy', 'false');
          if (hadFocus && document.activeElement === document.body) {
            button.focus({ preventScroll: true });
          }
        }
      }

      async function refreshStatus() {
        if (busy || refreshing || document.hidden) return;
        refreshing = true;
        const version = actionVersion;
        try {
          const status = await requestStatus('/api/status', 'GET');
          // A poll started before a control request must not overwrite it.
          if (version !== actionVersion || busy) return;
          updateStatus(status);
          if (refreshError) showMessage('Status updated.', 'success');
          refreshError = false;
        } catch (error) {
          if (version !== actionVersion || busy) return;
          refreshError = true;
          showMessage(requestError(error) + ' Showing the last known status.', 'error');
        } finally {
          refreshing = false;
        }
      }

      for (const button of buttons) {
        button.addEventListener('click', () => changeCollector(button));
      }
      setInterval(refreshStatus, 5000);
      document.addEventListener('visibilitychange', () => {
        if (!document.hidden) refreshStatus();
      });
    })();
  </script>
</body>
</html>
`
