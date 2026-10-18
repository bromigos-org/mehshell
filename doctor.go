package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type doctorIssue struct {
	Line    int    `json:"line,omitempty"`
	Key     string `json:"key,omitempty"`
	Message string `json:"message"`
}

type doctorTerminal struct {
	Term    string `json:"term"`
	Profile string `json:"profile"`
}

type doctorConfig struct {
	Path   string        `json:"path"`
	Exists bool          `json:"exists"`
	Loaded bool          `json:"loaded"`
	Issues []doctorIssue `json:"issues"`
}

type doctorEffective struct {
	Style           string            `json:"style"`
	Icons           bool              `json:"icons"`
	Brand           string            `json:"brand"`
	TransientPrompt bool              `json:"transient_prompt"`
	InstantPrompt   bool              `json:"instant_prompt"`
	ViMode          bool              `json:"vi_mode"`
	Segments        map[string]bool   `json:"segments"`
	Colours         map[string]string `json:"colours"`
}

type doctorTiming struct {
	Segment      string  `json:"segment"`
	Milliseconds float64 `json:"milliseconds"`
	Result       string  `json:"result"`
}

type doctorReport struct {
	SchemaVersion int             `json:"schema_version"`
	Terminal      doctorTerminal  `json:"terminal"`
	Config        doctorConfig    `json:"config"`
	Effective     doctorEffective `json:"effective"`
	Timings       []doctorTiming  `json:"timings,omitempty"`
}

// Config diagnostics never echo values: a mistaken pasted token must stay private.
func doctorKey(key string) string {
	if len(key) > 64 {
		return ""
	}
	for _, r := range key {
		if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '_') {
			return ""
		}
	}
	return key
}

func doctorConfigIssues(data []byte, cfg config) []doctorIssue {
	issues := []doctorIssue{}
	seen := map[string]bool{}
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			issues = append(issues, doctorIssue{Line: lineNumber, Message: "Expected a key = value assignment; line is ignored"})
			continue
		}
		key, value := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
		issue := func(message string) {
			issues = append(issues, doctorIssue{Line: lineNumber, Key: doctorKey(key), Message: message})
		}
		if seen[key] {
			issue("Repeated setting; a later valid value can replace the earlier one")
		}
		seen[key] = true
		_, segment := cfg.Segments[key]
		switch {
		case segment || key == "transient_prompt" || key == "instant_prompt" || key == "vi_mode" || key == "icons":
			if value != "true" && value != "false" && value != "1" && value != "0" && value != "yes" && value != "no" {
				issue("Use true or false (also accepted: 1, 0, yes, no)")
			}
		case key == "style":
			if value != "lean" && value != "classic" && value != "rainbow" {
				issue("Choose lean, classic or rainbow; this value is ignored")
			}
		case key == "brand":
			if value != "auto" && value != "bromigos" && value != "os" && value != "none" {
				issue("Choose auto, bromigos, os or none; this value is ignored")
			}
		case strings.HasPrefix(key, "color_"):
			index, err := strconv.Atoi(strings.TrimPrefix(key, "color_"))
			if err != nil || index < 0 || index > 255 {
				issue("Colour keys use color_ followed by an index from 0 to 255")
			}
			if !validColour(value) {
				issue("Use a six-digit hexadecimal colour beginning with #")
			}
		default:
			issue("Unknown setting; it is ignored")
		}
	}
	if scanner.Err() != nil {
		issues = append(issues, doctorIssue{Line: lineNumber + 1, Message: "A configuration line is too long; later settings may not be read"})
	}
	return issues
}

func readPromptConfig(path string) ([]byte, bool, error) {
	// O_NONBLOCK allows reporting a mistaken FIFO without hanging the shell.
	fd, err := syscall.Open(path, syscall.O_RDONLY|syscall.O_NONBLOCK, 0)
	if os.IsNotExist(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, true, err
	}
	file := os.NewFile(uintptr(fd), path)
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, true, err
	}
	const limit = 256 << 10
	if !info.Mode().IsRegular() || info.Size() > limit {
		return nil, true, fmt.Errorf("configuration must be a regular file of at most 256 KiB")
	}
	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	if len(data) > limit {
		return nil, true, fmt.Errorf("configuration is too large")
	}
	return data, true, err
}

type doctorProbe struct {
	name string
	run  func() string
}

func doctorProbes(cwd, home string, cfg config, icons bool) []doctorProbe {
	// These read project metadata and local runtime versions. Kubeconfig, cloud
	// configuration, credential files and remote services are deliberately absent.
	return []doctorProbe{
		{"os", func() string { return segBrand(cfg, icons) }},
		{"dir", func() string { return segDir(cwd, home) }},
		{"git", func() string { return segGit(cwd, icons) }},
		{"node", func() string { return segNode(cwd, icons) }},
		{"python", func() string { return segPython(cwd, icons) }},
		{"go", func() string { return segGo(cwd, icons) }},
		{"rust", func() string { return segRust(cwd, icons) }},
		{"ruby", func() string { return segRuby(cwd, icons) }},
		{"java", func() string { return segJava(cwd, icons) }},
		{"terraform", func() string { return segTerraform(cwd, icons) }},
		{"docker", func() string { return segDocker(cwd, icons) }},
		{"battery", func() string { return segBattery(icons) }},
	}
}

func measureDoctorProbes(probes []doctorProbe, enabled map[string]bool) []doctorTiming {
	results := []doctorTiming{}
	measured := map[string]bool{}
	for _, probe := range probes {
		measured[probe.name] = true
		result := doctorTiming{Segment: probe.name, Result: "disabled"}
		if enabled[probe.name] {
			start := time.Now()
			value := probe.run()
			result.Milliseconds = float64(time.Since(start).Nanoseconds()) / 1e6
			result.Result = "hidden or unavailable"
			if value != "" {
				result.Result = "shown"
			}
		}
		results = append(results, result)
	}
	for name, on := range enabled {
		if !measured[name] {
			result := "not probed"
			if !on {
				result = "disabled"
			}
			results = append(results, doctorTiming{Segment: name, Result: result})
		}
	}
	sort.Slice(results, func(i, j int) bool { return results[i].Segment < results[j].Segment })
	return results
}

func collectDoctor(timings bool) doctorReport {
	path := configPath()
	data, exists, err := readPromptConfig(path)
	cfg := defaultConfig()
	issues := []doctorIssue{}
	loaded := false
	if err != nil {
		issues = append(issues, doctorIssue{Message: "Cannot inspect this configuration safely; effective settings below use defaults"})
	} else if exists {
		cfg = parseConfig(data)
		loaded = true
		issues = doctorConfigIssues(data, cfg)
	}
	profile := "desktop"
	style, icons := cfg.Style, cfg.Icons
	switch os.Getenv("TERM") {
	case "linux":
		profile, style, icons = "linux-console", "console", false
	case "dumb":
		profile, style, icons = "plain", "console", false
	}
	colours := map[string]string{}
	if !basicTerminal() {
		for key, value := range cfg.Colours {
			if index, err := strconv.Atoi(key); err == nil && index >= 0 && index <= 255 && validColour(value) {
				colours[key] = value
			}
		}
	}
	report := doctorReport{
		SchemaVersion: 1,
		Terminal:      doctorTerminal{Term: os.Getenv("TERM"), Profile: profile},
		Config:        doctorConfig{Path: path, Exists: exists, Loaded: loaded, Issues: issues},
		Effective: doctorEffective{Style: style, Icons: icons, Brand: effectiveBrand(cfg),
			TransientPrompt: cfg.TransientPrompt, InstantPrompt: cfg.InstantPrompt, ViMode: cfg.ViMode,
			Segments: cfg.Segments, Colours: colours},
	}
	if timings {
		cwd, cwdErr := os.Getwd()
		home, homeErr := os.UserHomeDir()
		if cwdErr != nil || homeErr != nil {
			report.Config.Issues = append(report.Config.Issues, doctorIssue{Message: "Cannot determine a local directory for timing probes"})
		} else {
			report.Timings = measureDoctorProbes(doctorProbes(cwd, home, cfg, icons), cfg.Segments)
		}
	}
	return report
}

func runDoctor(args []string) int {
	flags := flag.NewFlagSet("mehshell doctor", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	jsonOutput := flags.Bool("json", false, "Print diagnostics as JSON")
	timings := flags.Bool("timings", false, "Measure enabled local project/runtime segments; no remote or credential probes")
	if err := flags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "usage: mehshell doctor [--json] [--timings]")
		return 2
	}
	report := collectDoctor(*timings)
	if *jsonOutput {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(report); err != nil {
			return 1
		}
	} else {
		fmt.Printf("Terminal: %s (TERM=%q)\n", report.Terminal.Profile, report.Terminal.Term)
		state := "defaults; no config file"
		if report.Config.Exists {
			state = "loaded"
			if !report.Config.Loaded {
				state = "unavailable; defaults shown"
			}
		}
		fmt.Printf("Config: %q (%s)\n", report.Config.Path, state)
		fmt.Printf("Prompt: %s; brand=%s; icons=%t\n", report.Effective.Style, report.Effective.Brand, report.Effective.Icons)
		fmt.Printf("Hooks: transient=%t; instant=%t; vi mode=%t\n", report.Effective.TransientPrompt, report.Effective.InstantPrompt, report.Effective.ViMode)
		var enabled []string
		for name, on := range report.Effective.Segments {
			if on {
				enabled = append(enabled, name)
			}
		}
		sort.Strings(enabled)
		fmt.Printf("Enabled segments: %s\n", strings.Join(enabled, ", "))
		if len(report.Config.Issues) == 0 {
			fmt.Println("Config checks: passed")
		}
		for _, issue := range report.Config.Issues {
			location := "Config"
			if issue.Line > 0 {
				location = fmt.Sprintf("Line %d", issue.Line)
			}
			if issue.Key != "" {
				location += " (" + issue.Key + ")"
			}
			fmt.Printf("%s: %s\n", location, issue.Message)
		}
		if *timings {
			fmt.Println("Local segment timings (segment text and credentials are not printed):")
			for _, timing := range report.Timings {
				fmt.Printf("  %-12s %8.2f ms  %s\n", timing.Segment, timing.Milliseconds, timing.Result)
			}
		}
		fmt.Println("Installed shell hooks and font glyph coverage are not inspected by this command.")
	}
	if len(report.Config.Issues) > 0 {
		return 1
	}
	return 0
}
