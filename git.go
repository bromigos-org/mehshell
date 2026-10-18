package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode"
)

const gitTimeout = 150 * time.Millisecond
const gitOutputLimit = 2 << 20

type gitRepo struct{ root, dir, branch, worktree string }

// HEAD, gitdir and operation markers are small local files, never shell input.
func gitReadFile(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() || info.Size() > 8192 {
		return "", errors.New("unsupported Git metadata")
	}
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, 8193))
	if err != nil || len(data) > 8192 {
		return "", errors.New("unreadable Git metadata")
	}
	return strings.TrimSpace(string(data)), nil
}

func findGitRepo(cwd string) gitRepo {
	dir, err := filepath.Abs(cwd)
	if err != nil {
		return gitRepo{}
	}
	for depth := 0; depth < 256; depth++ {
		gitDir := filepath.Join(dir, ".git")
		info, err := os.Stat(gitDir)
		if err == nil {
			if !info.IsDir() {
				text, err := gitReadFile(gitDir)
				if err != nil || !strings.HasPrefix(text, "gitdir: ") {
					return gitRepo{}
				}
				gitDir = strings.TrimPrefix(text, "gitdir: ")
				if !filepath.IsAbs(gitDir) {
					gitDir = filepath.Join(dir, gitDir)
				}
			}
			head, err := gitReadFile(filepath.Join(gitDir, "HEAD"))
			if err != nil {
				return gitRepo{root: dir, dir: gitDir, branch: "git"}
			}
			branch := strings.TrimPrefix(head, "ref: refs/heads/")
			if branch == head && len(head) >= 8 {
				branch = head[:8]
			}
			repo := gitRepo{root: dir, dir: gitDir, branch: branch}
			if common, err := gitReadFile(filepath.Join(gitDir, "commondir")); err == nil && common != "" {
				repo.worktree = filepath.Base(dir)
			}
			return repo
		}
		if !os.IsNotExist(err) {
			return gitRepo{}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return gitRepo{}
}

func gitBranch(cwd string) (branch string, repoDir string) {
	repo := findGitRepo(cwd)
	return repo.branch, repo.root
}

type gitStatus struct {
	branch, oid                                           string
	staged, modified, untracked, conflicts, ahead, behind int
	known, upstream, aheadKnown, unborn                   bool
}

// Porcelain v2 -z keeps spaces, newlines and rename source paths unambiguous.
func parseGitStatus(output []byte) gitStatus {
	var state gitStatus
	if len(output) == 0 || output[len(output)-1] != 0 {
		return state
	}
	records := bytes.Split(output[:len(output)-1], []byte{0})
	for index := 0; index < len(records); index++ {
		line := string(records[index])
		switch {
		case strings.HasPrefix(line, "# branch.head "):
			state.branch = strings.TrimPrefix(line, "# branch.head ")
		case strings.HasPrefix(line, "# branch.oid "):
			state.oid = strings.TrimPrefix(line, "# branch.oid ")
			state.unborn = state.oid == "(initial)"
		case strings.HasPrefix(line, "# branch.upstream "):
			state.upstream = true
		case strings.HasPrefix(line, "# branch.ab "):
			fields := strings.Fields(strings.TrimPrefix(line, "# branch.ab "))
			if len(fields) != 2 || !strings.HasPrefix(fields[0], "+") || !strings.HasPrefix(fields[1], "-") {
				return gitStatus{}
			}
			ahead, errA := strconv.Atoi(fields[0][1:])
			behind, errB := strconv.Atoi(fields[1][1:])
			if errA != nil || errB != nil || ahead < 0 || behind < 0 {
				return gitStatus{}
			}
			state.ahead, state.behind, state.aheadKnown = ahead, behind, true
		case strings.HasPrefix(line, "# "):
			// Git explicitly permits additional header fields.
		case strings.HasPrefix(line, "? "):
			state.untracked++
		case strings.HasPrefix(line, "! "):
			// Ignored paths do not make the worktree dirty.
		case strings.HasPrefix(line, "u "):
			if len(strings.SplitN(line, " ", 11)) != 11 {
				return gitStatus{}
			}
			state.conflicts++
		case strings.HasPrefix(line, "1 "), strings.HasPrefix(line, "2 "):
			count := 9
			if line[0] == '2' {
				count = 10
			}
			fields := strings.SplitN(line, " ", count)
			if len(fields) != count || len(fields[1]) != 2 {
				return gitStatus{}
			}
			if fields[1][0] != '.' {
				state.staged++
			}
			if fields[1][1] != '.' {
				state.modified++
			}
			if line[0] == '2' {
				index++ // The next NUL-delimited record is the original pathname.
				if index >= len(records) || len(records[index]) == 0 {
					return gitStatus{}
				}
			}
		default:
			return gitStatus{}
		}
	}
	state.known = state.branch != "" && state.oid != ""
	return state
}

type gitOutput struct {
	buffer bytes.Buffer
	cancel context.CancelFunc
}

func (output *gitOutput) Len() int      { return output.buffer.Len() }
func (output *gitOutput) Bytes() []byte { return output.buffer.Bytes() }

func (output *gitOutput) Write(data []byte) (int, error) {
	if len(data) > gitOutputLimit-output.Len() {
		output.cancel()
		return 0, errors.New("Git status exceeded the prompt budget")
	}
	return output.buffer.Write(data)
}

func gitEnvironment() []string {
	var env []string
	for _, item := range os.Environ() {
		if !strings.HasPrefix(item, "GIT_") && !strings.HasPrefix(item, "LC_ALL=") {
			env = append(env, item)
		}
	}
	return append(env, "LC_ALL=C", "GIT_OPTIONAL_LOCKS=0", "GIT_TERMINAL_PROMPT=0", "GIT_NO_LAZY_FETCH=1", "GIT_ALLOW_PROTOCOL=")
}

func readGitStatus(repoDir string) gitStatus {
	ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "--no-optional-locks", "-c", "core.fsmonitor=false", "-c", "core.untrackedCache=false", "-c", "status.submoduleSummary=false", "status", "--porcelain=v2", "--branch", "--ahead-behind", "-z", "--untracked-files=all", "--ignore-submodules=none")
	cmd.Dir, cmd.Env = repoDir, gitEnvironment()
	cmd.WaitDelay = 20 * time.Millisecond
	output := gitOutput{cancel: cancel}
	cmd.Stdout, cmd.Stderr = &output, io.Discard
	if err := cmd.Run(); err != nil || ctx.Err() != nil {
		return gitStatus{}
	}
	return parseGitStatus(output.Bytes())
}

func gitOperation(gitDir string) string {
	for _, marker := range []struct{ path, name string }{
		{"rebase-merge", "rebase"}, {"rebase-apply", "rebase"}, {"MERGE_HEAD", "merge"}, {"CHERRY_PICK_HEAD", "cherry-pick"}, {"REVERT_HEAD", "revert"},
	} {
		if _, err := os.Stat(filepath.Join(gitDir, marker.path)); err == nil {
			return marker.name
		} else if !os.IsNotExist(err) {
			return "operation?"
		}
	}
	return ""
}

func gitStatusText(state gitStatus, icons bool) string {
	if !state.known {
		return fc(cYellow, "status?")
	}
	var parts []string
	if state.upstream && !state.aheadKnown {
		parts = append(parts, fc(cYellow, "upstream?"))
	}
	for _, count := range []struct {
		value, color int
		label, icon  string
	}{
		{state.ahead, cCyan, "ahead:", "↑"}, {state.behind, cYellow, "behind:", "↓"},
		{state.staged, cGreen, "+", "+"}, {state.modified, cRed, "!", "!"},
		{state.untracked, cYellow, "?", "?"}, {state.conflicts, cRed, "conflict:", "conflict:"},
	} {
		if count.value > 0 {
			label := count.label
			if icons && !basicTerminal() {
				label = count.icon
			}
			parts = append(parts, fc(count.color, label+strconv.Itoa(count.value)))
		}
	}
	if state.staged+state.modified+state.untracked+state.conflicts == 0 {
		label := "clean"
		if icons && !basicTerminal() {
			label = "✓"
		}
		parts = append(parts, fc(cGreen, label))
	}
	return strings.Join(parts, " ")
}

func gitDirty(repoDir string) string { return gitStatusText(readGitStatus(repoDir), !basicTerminal()) }

func gitLabel(text string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return '?'
		}
		return r
	}, text)
}

func formatGitSegment(cwd string, icons bool) string {
	repo := findGitRepo(cwd)
	if repo.root == "" {
		return ""
	}
	state := readGitStatus(repo.root)
	branch := repo.branch
	if state.known {
		branch = state.branch
		if branch == "(detached)" {
			branch = fmt.Sprintf("detached@%.8s", state.oid)
		}
	}
	parts := []string{fc(cMagenta, gitLabel(branch))}
	if icons && !basicTerminal() {
		parts[0] = fc(cBlue, "\uf126 ") + parts[0]
	}
	if repo.worktree != "" {
		parts = append(parts, fc(cCyan, "wt:"+gitLabel(repo.worktree)))
	}
	if operation := gitOperation(repo.dir); operation != "" {
		parts = append(parts, fc(cOrange, operation))
	}
	if state.unborn {
		parts = append(parts, fc(cGray, "new"))
	}
	parts = append(parts, gitStatusText(state, icons))
	return strings.Join(parts, " ")
}
