package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

func segOS(icons bool) string {
	if !icons {
		return ""
	}
	icon := "\uf17c"
	switch runtime.GOOS {
	case "linux":
		if data, err := os.ReadFile("/etc/os-release"); err == nil {
			lower := strings.ToLower(string(data))
			switch {
			case strings.Contains(lower, "arch"):
				icon = "\uf303"
			case strings.Contains(lower, "ubuntu"):
				icon = "\uf31b"
			case strings.Contains(lower, "fedora"):
				icon = "\uf30a"
			case strings.Contains(lower, "debian"):
				icon = "\uf306"
			case strings.Contains(lower, "nixos"):
				icon = "\uf313"
			}
		}
	case "darwin":
		icon = "\uf179"
	}
	return fc(cBlue, icon)
}

func segDir(cwd, home string) string {
	dir := cwd
	if home != "" && (dir == home || strings.HasPrefix(dir, home+string(filepath.Separator))) {
		dir = "~" + dir[len(home):]
	}
	if dir == "~/" {
		dir = "~"
	}
	return fc(cBlue, dir)
}

func segGit(cwd string, icons bool) string {
	return formatGitSegment(cwd, icons)
}

func segNode(cwd string, icons bool) string {
	if _, err := os.Stat(filepath.Join(cwd, "package.json")); err != nil {
		return ""
	}
	prefix := "node "
	if icons {
		prefix = "\ue718 "
	}
	for _, f := range []string{".node-version", ".nvmrc"} {
		if data, err := os.ReadFile(filepath.Join(cwd, f)); err == nil {
			ver := strings.TrimSpace(string(data))
			if ver != "" {
				return fc(cGreen, prefix+ver)
			}
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if out, err := exec.CommandContext(ctx, "node", "--version").Output(); err == nil {
		return fc(cGreen, prefix+strings.TrimSpace(string(out)))
	}
	return ""
}

func segPython(cwd string, icons bool) string {
	if !hasMarkerUp(cwd, []string{".python-version", "pyproject.toml", "setup.py", "Pipfile", "requirements.txt"}) {
		return ""
	}
	prefix := "py "
	if icons {
		prefix = "\ue73c "
	}
	if data, err := os.ReadFile(filepath.Join(cwd, ".python-version")); err == nil {
		ver := strings.TrimSpace(string(data))
		if ver != "" {
			return fc(cYellow, prefix+ver)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if out, err := exec.CommandContext(ctx, "python", "--version").Output(); err == nil {
		ver := strings.TrimPrefix(strings.TrimSpace(string(out)), "Python ")
		return fc(cYellow, prefix+ver)
	}
	return ""
}

func segGo(cwd string, icons bool) string {
	gomod := filepath.Join(cwd, "go.mod")
	data, err := os.ReadFile(gomod)
	if err != nil {
		return ""
	}
	prefix := "go "
	if icons {
		prefix = "\ue627 "
	}
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "go ") {
			ver := strings.Fields(line)[1]
			return fc(cCyan, prefix+ver)
		}
	}
	return ""
}

func segRust(cwd string, icons bool) string {
	if _, err := os.Stat(filepath.Join(cwd, "Cargo.toml")); err != nil {
		return ""
	}
	prefix := "rs "
	if icons {
		prefix = "\ue7a8 "
	}
	for _, f := range []string{"rust-toolchain", ".rust-version"} {
		if data, err := os.ReadFile(filepath.Join(cwd, f)); err == nil {
			ver := strings.TrimSpace(string(data))
			if ver != "" {
				return fc(cOrange, prefix+ver)
			}
		}
	}
	if data, err := os.ReadFile(filepath.Join(cwd, "rust-toolchain.toml")); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "channel") {
				parts := strings.SplitN(line, "=", 2)
				if len(parts) == 2 {
					ver := strings.Trim(strings.TrimSpace(parts[1]), "\"'")
					if ver != "" {
						return fc(cOrange, prefix+ver)
					}
				}
			}
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if out, err := exec.CommandContext(ctx, "rustc", "--version").Output(); err == nil {
		ver := strings.TrimPrefix(strings.TrimSpace(string(out)), "rustc ")
		if i := strings.Index(ver, " "); i != -1 {
			ver = ver[:i]
		}
		return fc(cOrange, prefix+ver)
	}
	return ""
}

func segRuby(cwd string, icons bool) string {
	if !hasMarkerUp(cwd, []string{".ruby-version", "Gemfile", "Rakefile"}) {
		return ""
	}
	prefix := "rb "
	if icons {
		prefix = "\ue739 "
	}
	if data, err := os.ReadFile(filepath.Join(cwd, ".ruby-version")); err == nil {
		ver := strings.TrimSpace(string(data))
		if ver != "" {
			return fc(cRed, prefix+ver)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if out, err := exec.CommandContext(ctx, "ruby", "--version").Output(); err == nil {
		ver := strings.TrimPrefix(strings.TrimSpace(string(out)), "ruby ")
		if i := strings.Index(ver, " "); i != -1 {
			ver = ver[:i]
		}
		return fc(cRed, prefix+ver)
	}
	return ""
}

func segJava(cwd string, icons bool) string {
	markers := []string{"pom.xml", "build.gradle", "build.gradle.kts", ".java-version"}
	found := false
	for _, m := range markers {
		if _, err := os.Stat(filepath.Join(cwd, m)); err == nil {
			found = true
			break
		}
	}
	if !found {
		return ""
	}
	prefix := "java "
	if icons {
		prefix = "\ue738 "
	}
	if data, err := os.ReadFile(filepath.Join(cwd, ".java-version")); err == nil {
		ver := strings.TrimSpace(string(data))
		if ver != "" {
			return fc(cOrange, prefix+ver)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if out, err := exec.CommandContext(ctx, "java", "-version").CombinedOutput(); err == nil {
		line := strings.SplitN(string(out), "\n", 2)[0]
		if start := strings.Index(line, "\""); start != -1 {
			if end := strings.Index(line[start+1:], "\""); end != -1 {
				return fc(cOrange, prefix+line[start+1:start+1+end])
			}
		}
	}
	return ""
}

func segConda(icons bool) string {
	env := os.Getenv("CONDA_DEFAULT_ENV")
	if env == "" || env == "base" {
		return ""
	}
	prefix := "conda "
	if icons {
		prefix = "\ue73c "
	}
	return fc(cGreen, prefix+env)
}

func segVenv(icons bool) string {
	venv := os.Getenv("VIRTUAL_ENV")
	if venv == "" {
		return ""
	}
	prefix := "venv "
	if icons {
		prefix = "\ue73c "
	}
	return fc(cGreen, prefix+filepath.Base(venv))
}

func segK8s(cwd, home string, icons bool) string {
	markers := []string{"skaffold.yaml", "helmfile.yaml", "Chart.yaml", "kustomization.yaml"}
	hasMarker := false
	for _, m := range markers {
		if _, err := os.Stat(filepath.Join(cwd, m)); err == nil {
			hasMarker = true
			break
		}
	}
	if !hasMarker {
		return ""
	}

	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		kubeconfig = filepath.Join(home, ".kube", "config")
	}
	data, err := os.ReadFile(kubeconfig)
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "current-context:") {
			ctx := strings.TrimSpace(strings.TrimPrefix(line, "current-context:"))
			if ctx != "" && ctx != "\"\"" {
				prefix := "k8s "
				if icons {
					prefix = "\u2388 "
				}
				return fc(cBlue, prefix+ctx)
			}
		}
	}
	return ""
}

func segTerraform(cwd string, icons bool) string {
	entries, err := os.ReadDir(cwd)
	if err != nil {
		return ""
	}
	hasTF := false
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".tf") {
			hasTF = true
			break
		}
	}
	if !hasTF {
		return ""
	}
	prefix := "tf "
	if icons {
		prefix = "\uf0ac "
	}
	if data, err := os.ReadFile(filepath.Join(cwd, ".terraform-version")); err == nil {
		ver := strings.TrimSpace(string(data))
		if ver != "" {
			return fc(cMagenta, prefix+ver)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if out, err := exec.CommandContext(ctx, "terraform", "version").Output(); err == nil {
		line := strings.SplitN(strings.TrimSpace(string(out)), "\n", 2)[0]
		line = strings.TrimPrefix(line, "Terraform ")
		return fc(cMagenta, prefix+line)
	}
	return ""
}

func segDocker(cwd string, icons bool) string {
	markers := []string{"Dockerfile", "docker-compose.yml", "docker-compose.yaml", "compose.yml", "compose.yaml"}
	found := false
	for _, m := range markers {
		if _, err := os.Stat(filepath.Join(cwd, m)); err == nil {
			found = true
			break
		}
	}
	if !found {
		return ""
	}
	prefix := "docker"
	if icons {
		prefix = "\uf308"
	}
	if ctx := os.Getenv("DOCKER_CONTEXT"); ctx != "" && ctx != "default" {
		return fc(cCyan, prefix+" "+ctx)
	}
	if name := os.Getenv("DOCKER_MACHINE_NAME"); name != "" {
		return fc(cCyan, prefix+" "+name)
	}
	return fc(cCyan, prefix)
}

func segAWS(icons bool) string {
	profile := os.Getenv("AWS_PROFILE")
	if profile == "" {
		return ""
	}
	prefix := "aws "
	if icons {
		prefix = "\uf52c "
	}
	return fc(cOrange, prefix+profile)
}

func segAzure(icons bool) string {
	acct := os.Getenv("AZURE_DEFAULTS_GROUP")
	if acct == "" {
		return ""
	}
	prefix := "az "
	if icons {
		prefix = "\ufd03 "
	}
	return fc(cBlue, prefix+acct)
}

func segGCloud(icons bool) string {
	project := os.Getenv("CLOUDSDK_CORE_PROJECT")
	if project == "" {
		project = os.Getenv("GCLOUD_PROJECT")
	}
	if project == "" {
		project = os.Getenv("GOOGLE_CLOUD_PROJECT")
	}
	if project == "" {
		return ""
	}
	prefix := "gcp "
	if icons {
		prefix = "\uf1a0 "
	}
	return fc(cGreen, prefix+project)
}

func segBattery(icons bool) string {
	switch runtime.GOOS {
	case "linux":
		for _, name := range []string{"BAT0", "BAT1", "battery"} {
			data, err := os.ReadFile(filepath.Join("/sys/class/power_supply", name, "capacity"))
			if err != nil {
				continue
			}
			pct, err := strconv.Atoi(strings.TrimSpace(string(data)))
			if err != nil {
				continue
			}
			statusData, _ := os.ReadFile(filepath.Join("/sys/class/power_supply", name, "status"))
			status := strings.TrimSpace(string(statusData))

			icon := "\uf240"
			color := cGreen
			switch {
			case pct <= 10:
				icon = "\uf244"
				color = cRed
			case pct <= 25:
				icon = "\uf243"
				color = cOrange
			case pct <= 50:
				icon = "\uf242"
				color = cYellow
			case pct <= 75:
				icon = "\uf241"
				color = cGreen
			}
			if status == "Charging" {
				icon = "\uf0e7"
			}
			if icons {
				return fc(color, icon+" "+strconv.Itoa(pct)+"%")
			}
			return fc(color, strconv.Itoa(pct)+"%")
		}
	case "darwin":
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		out, err := exec.CommandContext(ctx, "pmset", "-g", "batt").Output()
		if err != nil {
			return ""
		}
		for _, line := range strings.Split(string(out), "\n") {
			if !strings.Contains(line, "InternalBattery") {
				continue
			}
			idx := strings.Index(line, "%")
			if idx == -1 {
				continue
			}
			start := idx - 1
			for start >= 0 && line[start] >= '0' && line[start] <= '9' {
				start--
			}
			pct, err := strconv.Atoi(strings.TrimSpace(line[start+1 : idx]))
			if err != nil {
				continue
			}
			icon := "\uf240"
			color := cGreen
			switch {
			case pct <= 10:
				icon = "\uf244"
				color = cRed
			case pct <= 25:
				icon = "\uf243"
				color = cOrange
			case pct <= 50:
				icon = "\uf242"
				color = cYellow
			case pct <= 75:
				icon = "\uf241"
				color = cGreen
			}
			if strings.Contains(line, "charging") && !strings.Contains(line, "discharging") {
				icon = "\uf0e7"
			}
			if icons {
				return fc(color, icon+" "+strconv.Itoa(pct)+"%")
			}
			return fc(color, strconv.Itoa(pct)+"%")
		}
	}
	return ""
}

func segDuration(secs int) string {
	if secs < 3 {
		return ""
	}
	var s string
	switch {
	case secs >= 3600:
		s = fmt.Sprintf("%dh%dm", secs/3600, (secs%3600)/60)
	case secs >= 60:
		s = fmt.Sprintf("%dm%ds", secs/60, secs%60)
	default:
		s = fmt.Sprintf("%ds", secs)
	}
	return fc(cYellow, s)
}

func segTime() string {
	return fc(cGray, time.Now().Format("03:04:05 PM"))
}

func hasMarkerUp(cwd string, markers []string) bool {
	dir := cwd
	for {
		for _, m := range markers {
			if _, err := os.Stat(filepath.Join(dir, m)); err == nil {
				return true
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return false
		}
		dir = parent
	}
}
