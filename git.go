package main

import (
	"bufio"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func gitBranch(cwd string) (branch string, repoDir string) {
	dir := cwd
	for {
		gitPath := filepath.Join(dir, ".git")
		info, err := os.Stat(gitPath)
		if err == nil {
			var headPath string
			if info.IsDir() {
				headPath = filepath.Join(gitPath, "HEAD")
			} else {
				data, err := os.ReadFile(gitPath)
				if err != nil {
					return "", ""
				}
				gitDir := strings.TrimSpace(strings.TrimPrefix(string(data), "gitdir: "))
				if !filepath.IsAbs(gitDir) {
					gitDir = filepath.Join(dir, gitDir)
				}
				headPath = filepath.Join(gitDir, "HEAD")
			}
			data, err := os.ReadFile(headPath)
			if err != nil {
				return "", ""
			}
			head := strings.TrimSpace(string(data))
			if strings.HasPrefix(head, "ref: refs/heads/") {
				return head[16:], dir
			}
			if len(head) >= 8 {
				return head[:8], dir
			}
			return "", ""
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", ""
		}
		dir = parent
	}
}

func gitDirty(repoDir string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "status", "--porcelain")
	cmd.Dir = repoDir
	out, err := cmd.Output()
	if err != nil {
		return ""
	}

	staged, modified, untracked := false, false, false
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) < 2 {
			continue
		}
		x, y := line[0], line[1]
		switch {
		case x == '?':
			untracked = true
		default:
			if x != ' ' {
				staged = true
			}
			if y != ' ' {
				modified = true
			}
		}
	}

	if !staged && !modified && !untracked {
		if basicTerminal() {
			return fc(cGreen, "clean")
		}
		return fc(cGreen, "✓")
	}

	var parts []string
	if staged {
		parts = append(parts, fc(cGreen, "+"))
	}
	if modified {
		parts = append(parts, fc(cRed, "!"))
	}
	if untracked {
		parts = append(parts, fc(cYellow, "?"))
	}
	return strings.Join(parts, "")
}
