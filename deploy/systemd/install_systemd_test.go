package systemd_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

type installerFixture struct {
	script, binary, source, destination, calls, binDir, installedBinary string
	failSource                                                          string
}

func newInstallerFixture(t *testing.T) *installerFixture {
	t.Helper()
	root := filepath.Join(t.TempDir(), "fake root")
	f := &installerFixture{
		script: filepath.Join(root, "source", "install-systemd.sh"), binary: filepath.Join(root, "binary"),
		source: filepath.Join(root, "edited config.conf"), destination: filepath.Join(root, "etc", "ecoflow-ble-nutd.conf"),
		calls: filepath.Join(root, "systemctl.log"), binDir: filepath.Join(root, "stubs"),
		installedBinary: filepath.Join(root, "usr", "local", "bin", "ecoflow-ble-nutd"),
	}
	write := func(path, body string, mode os.FileMode) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), mode); err != nil {
			t.Fatal(err)
		}
	}
	script, err := os.ReadFile("install-systemd.sh")
	if err != nil {
		t.Fatal(err)
	}
	// Redirect only install paths; execute the real script's argument and
	// replacement logic without root access or host service/file changes.
	text := strings.ReplaceAll(string(script), "/etc/", filepath.Join(root, "etc")+"/")
	text = strings.ReplaceAll(text, "/usr/local/bin/", filepath.Join(root, "usr", "local", "bin")+"/")
	write(f.script, text, 0o755)
	write(filepath.Join(root, "source", "ecoflow-ble-nutd.service"), "service fixture\n", 0o644)
	write(f.binary, "binary fixture\n", 0o755)
	write(f.source, "new config\n", 0o600)
	write(f.destination, "old config\n", 0o600)
	write(filepath.Join(f.binDir, "id"), "#!/bin/sh\necho 0\n", 0o755)
	write(filepath.Join(f.binDir, "systemd-sysusers"), "#!/bin/sh\ncat >/dev/null\n", 0o755)
	write(filepath.Join(f.binDir, "systemctl"), `#!/bin/sh
if [ "$1" = --version ]; then echo 'systemd 247'; else printf '%s\n' "$*" >> "$TEST_SYSTEMCTL_LOG"; fi
`, 0o755)
	// The host can be macOS, where install has no GNU -D option.
	write(filepath.Join(f.binDir, "install"), `#!/bin/sh
set -eu
mode=755
while [ $# -gt 0 ]; do
  case "$1" in -D) shift;; -m) mode="$2"; shift 2;; *) break;; esac
done
mkdir -p "$(dirname "$2")"
if [ "${TEST_FAIL_CONFIG:-}" = "$1" ]; then printf partial > "$2"; exit 3; fi
cp "$1" "$2"
chmod "$mode" "$2"
`, 0o755)
	return f
}

func (f *installerFixture) run(args ...string) (string, error) {
	cmd := exec.Command("bash", append([]string{f.script, "--binary", f.binary}, args...)...)
	cmd.Env = append(os.Environ(), "PATH="+f.binDir+string(os.PathListSeparator)+os.Getenv("PATH"),
		"TEST_SYSTEMCTL_LOG="+f.calls, "TEST_FAIL_CONFIG="+f.failSource)
	output, err := cmd.CombinedOutput()
	return string(output), err
}

func assertConfig(t *testing.T, path, want string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil || string(data) != want {
		t.Fatalf("%s: content=%q, error=%v, want %q", path, data, err, want)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("%s: expected private mode 0600, info=%v, error=%v", path, info, err)
	}
}

func TestInstallConfigModes(t *testing.T) {
	for _, tc := range []struct {
		name                          string
		fresh, replace, skip, noStart bool
		want                          string
	}{
		{name: "first install", fresh: true, want: "new config\n"},
		{name: "first install with replacement flag", fresh: true, replace: true, want: "new config\n"},
		{name: "preserve by default", want: "old config\n"},
		{name: "explicit replacement", replace: true, want: "new config\n"},
		{name: "replacement without restart", replace: true, noStart: true, want: "new config\n"},
		{name: "skip config", skip: true, want: "old config\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newInstallerFixture(t)
			if tc.fresh {
				if err := os.Remove(f.destination); err != nil {
					t.Fatal(err)
				}
			}
			args := []string{"--config", f.source}
			if tc.replace {
				args = append(args, "--replace-config")
			}
			if tc.skip {
				args = append(args, "--skip-config")
			}
			if tc.noStart {
				args = append(args, "--no-start")
			}
			if out, err := f.run(args...); err != nil {
				t.Fatalf("%v: %s", err, out)
			}
			assertConfig(t, f.destination, tc.want)
			backups, _ := filepath.Glob(f.destination + ".bak.*")
			if tc.replace && !tc.fresh {
				if len(backups) != 1 {
					t.Fatalf("backups = %v", backups)
				}
				assertConfig(t, backups[0], "old config\n")
			} else if len(backups) != 0 {
				t.Fatalf("unexpected backups: %v", backups)
			}
			calls, err := os.ReadFile(f.calls)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(calls), "restart ecoflow-ble-nutd.service") == tc.noStart {
				t.Fatalf("unexpected restart behavior: %s", calls)
			}
		})
	}
}

func TestReplaceConfigSourceAlreadyInstalled(t *testing.T) {
	f := newInstallerFixture(t)
	if out, err := f.run("--config", f.destination, "--replace-config"); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	assertConfig(t, f.destination, "old config\n")
	backups, _ := filepath.Glob(f.destination + ".bak.*")
	if len(backups) != 0 {
		t.Fatalf("unexpected backup of unchanged source: %v", backups)
	}
}

func TestReplaceConfigRejectsInvalidArgumentsBeforeInstall(t *testing.T) {
	for _, args := range [][]string{
		{"--replace-config"}, {"--replace-config", "--skip-config"}, {"--config"},
		{"--replace-config", "--config", "missing.conf"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			f := newInstallerFixture(t)
			if out, err := f.run(args...); err == nil {
				t.Fatalf("expected failure: %s", out)
			}
			assertConfig(t, f.destination, "old config\n")
			if _, err := os.Stat(f.installedBinary); !os.IsNotExist(err) {
				t.Fatalf("binary installed on invalid input: %v", err)
			}
		})
	}
}

func TestReplaceConfigFailureLeavesOriginalAndBackup(t *testing.T) {
	f := newInstallerFixture(t)
	f.failSource = f.source
	if out, err := f.run("--config", f.source, "--replace-config"); err == nil {
		t.Fatalf("expected copy failure: %s", out)
	}
	assertConfig(t, f.destination, "old config\n")
	backups, _ := filepath.Glob(f.destination + ".bak.*")
	if len(backups) != 1 {
		t.Fatalf("backups = %v", backups)
	}
	assertConfig(t, backups[0], "old config\n")
	leftovers, _ := filepath.Glob(f.destination + ".tmp.*")
	if len(leftovers) != 0 {
		t.Fatalf("staged config left behind: %v", leftovers)
	}
	if calls, _ := os.ReadFile(f.calls); strings.Contains(string(calls), "restart") {
		t.Fatalf("restarted after failed replacement: %s", calls)
	}
}

func TestRepeatedReplacementKeepsDistinctBackups(t *testing.T) {
	f := newInstallerFixture(t)
	for range 2 {
		if out, err := f.run("--config", f.source, "--replace-config"); err != nil {
			t.Fatalf("%v: %s", err, out)
		}
	}
	backups, _ := filepath.Glob(f.destination + ".bak.*")
	if len(backups) != 2 {
		t.Fatalf("backups = %v", backups)
	}
	contents := map[string]bool{}
	for _, path := range backups {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		contents[string(data)] = true
	}
	if !contents["old config\n"] || !contents["new config\n"] {
		t.Fatalf("backup contents = %v", contents)
	}
}
