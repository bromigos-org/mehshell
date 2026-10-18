package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func doctorSandbox(t *testing.T, content string) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "config"))
	t.Setenv("TERM", "xterm-kitty")
	t.Setenv("MEHSHELL_BROMIGOS", "0")
	if err := os.MkdirAll(filepath.Dir(configPath()), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath(), []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	return home
}

func captureDoctor(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	stdout, err := os.CreateTemp(t.TempDir(), "stdout")
	if err != nil {
		t.Fatal(err)
	}
	defer stdout.Close()
	stderr, err := os.CreateTemp(t.TempDir(), "stderr")
	if err != nil {
		t.Fatal(err)
	}
	defer stderr.Close()
	oldOut, oldErr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = stdout, stderr
	defer func() { os.Stdout, os.Stderr = oldOut, oldErr }()
	code := runDoctor(args)
	output, err := os.ReadFile(stdout.Name())
	if err != nil {
		t.Fatal(err)
	}
	errors, err := os.ReadFile(stderr.Name())
	if err != nil {
		t.Fatal(err)
	}
	return code, string(output), string(errors)
}

func TestDoctorReportsInvalidConfigWithoutExposingValues(t *testing.T) {
	secret := "PRIVATE_VALUE_MUST_NOT_APPEAR"
	doctorSandbox(t, "brand=none\nicons=maybe\nstyle=mystery\nunknown="+secret+"\ncolor_75="+secret+"\ntime=true\ntime=false\ninvalid assignment\n")
	code, output, stderr := captureDoctor(t, "--json")
	if code != 1 || stderr != "" || strings.Contains(output, secret) {
		t.Fatalf("unsafe or unexpected diagnostics: code=%d output=%s stderr=%s", code, output, stderr)
	}
	var report doctorReport
	if err := json.Unmarshal([]byte(output), &report); err != nil {
		t.Fatal(err)
	}
	if report.SchemaVersion != 1 || !report.Config.Loaded || len(report.Config.Issues) != 6 || report.Timings != nil {
		t.Fatalf("missing diagnostic detail: %#v", report)
	}
	if report.Effective.Icons || report.Effective.Segments["time"] {
		t.Fatal("diagnostics did not report the effective parsed boolean settings")
	}
	code, human, _ := captureDoctor(t)
	if code != 1 || !strings.Contains(human, "Line 4 (unknown): Unknown setting") || strings.Contains(human, secret) {
		t.Fatalf("human diagnostic missing or leaked a value: %s", human)
	}
}

func TestDoctorReportsEffectiveTTYSettingsAndBrand(t *testing.T) {
	doctorSandbox(t, "style=rainbow\nicons=true\nbrand=bromigos\ncolor_75=#123456\nsession=true\nstatus=true\ngame=false\n")
	for _, term := range []string{"linux", "dumb", "xterm-kitty"} {
		t.Run(term, func(t *testing.T) {
			t.Setenv("TERM", term)
			report := collectDoctor(false)
			if report.Effective.Brand != "bromigos" || len(report.Config.Issues) != 0 || report.Timings != nil {
				t.Fatalf("unexpected effective configuration: %#v", report)
			}
			if term == "xterm-kitty" {
				if report.Terminal.Profile != "desktop" || report.Effective.Style != "rainbow" || !report.Effective.Icons || report.Effective.Colours["75"] != "#123456" {
					t.Fatal("desktop configuration changed")
				}
			} else if report.Effective.Style != "console" || report.Effective.Icons || len(report.Effective.Colours) != 0 {
				t.Fatal("TTY fallback not reflected in effective settings")
			}
		})
	}
}

func TestDoctorDefaultsAndUnsafeConfigFiles(t *testing.T) {
	doctorSandbox(t, "brand=none\n")
	path := configPath()
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	missing := collectDoctor(false)
	if missing.Config.Exists || missing.Config.Loaded || len(missing.Config.Issues) != 0 {
		t.Fatalf("missing configuration should use normal defaults: %#v", missing.Config)
	}
	if err := syscall.Mkfifo(path, 0600); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	unsafe := collectDoctor(false)
	if time.Since(start) > time.Second || unsafe.Config.Loaded || len(unsafe.Config.Issues) != 1 {
		t.Fatalf("FIFO was not rejected immediately: %#v", unsafe.Config)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(strings.Repeat("x", (256<<10)+1)), 0600); err != nil {
		t.Fatal(err)
	}
	large := collectDoctor(false)
	if large.Config.Loaded || len(large.Config.Issues) != 1 {
		t.Fatal("oversized configuration was loaded")
	}
}

func TestDoctorTimingsDiscardTextAndDoNotInspectCredentialSegments(t *testing.T) {
	calls := 0
	results := measureDoctorProbes([]doctorProbe{
		{"node", func() string { calls++; return "DO_NOT_PRINT_SEGMENT_CONTENT" }},
		{"git", func() string { t.Fatal("disabled probe executed"); return "" }},
	}, map[string]bool{"node": true, "git": false, "k8s": true})
	data, err := json.Marshal(results)
	if err != nil || calls != 1 || strings.Contains(string(data), "DO_NOT_PRINT") {
		t.Fatalf("probe content escaped into the report: %s (%v)", data, err)
	}
	for _, probe := range doctorProbes(t.TempDir(), t.TempDir(), defaultConfig(), false) {
		switch probe.name {
		case "k8s", "aws", "azure", "gcloud":
			t.Fatalf("credential configuration segment %s must not be probed", probe.name)
		}
	}
}

func TestDoctorTimingUsesBoundedRuntimeCommandOnlyWhenRequested(t *testing.T) {
	var config strings.Builder
	config.WriteString("brand=none\n")
	for name := range defaultConfig().Segments {
		value := "false"
		if name == "node" {
			value = "true"
		}
		config.WriteString(name + "=" + value + "\n")
	}
	home := doctorSandbox(t, config.String())
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(home); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(cwd); err != nil {
			t.Error(err)
		}
	})
	if err := os.WriteFile(filepath.Join(home, "package.json"), []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(home, "bin")
	if err := os.Mkdir(bin, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bin, "node"), []byte("#!/bin/sh\nprintf called > \"$HOME/runtime-called\"\nexec /bin/sleep 3\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	code, _, _ := captureDoctor(t, "--json")
	if code != 0 {
		t.Fatal("valid configuration should pass")
	}
	if _, err := os.Stat(filepath.Join(home, "runtime-called")); !os.IsNotExist(err) {
		t.Fatal("default diagnostics ran a runtime command")
	}
	start := time.Now()
	code, output, _ := captureDoctor(t, "--json", "--timings")
	if code != 0 || time.Since(start) > 1500*time.Millisecond {
		t.Fatal("runtime version probe was not bounded")
	}
	if _, err := os.Stat(filepath.Join(home, "runtime-called")); err != nil {
		t.Fatal("explicit timing did not exercise the runtime probe")
	}
	if !strings.Contains(output, "hidden or unavailable") {
		t.Fatal("timing did not report the unavailable runtime segment")
	}
}

func TestDoctorRejectsUnexpectedArguments(t *testing.T) {
	doctorSandbox(t, "brand=none\n")
	code, stdout, stderr := captureDoctor(t, "extra")
	if code != 2 || stdout != "" || !strings.Contains(stderr, "usage:") {
		t.Fatalf("unexpected argument handling: %d %q %q", code, stdout, stderr)
	}
}
