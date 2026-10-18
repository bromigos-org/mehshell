package main

import (
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func fixtureGit(t *testing.T, dir string, args ...string) (string, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	prefix := []string{"-c", "core.hooksPath=/dev/null", "-c", "commit.gpgsign=false", "-c", "user.name=Prompt Test", "-c", "user.email=prompt@example.invalid"}
	command := exec.CommandContext(ctx, "git", append(prefix, args...)...)
	command.Dir = dir
	command.Env = append(gitEnvironment(), "GIT_CONFIG_NOSYSTEM=1")
	output, err := command.CombinedOutput()
	return strings.TrimSpace(string(output)), err
}

func mustGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	output, err := fixtureGit(t, dir, args...)
	if err != nil {
		t.Fatalf("fixture git %v: %v: %s", args, err, output)
	}
	return output
}

func gitFile(t *testing.T, root, name, text string) {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
}

func gitFixture(t *testing.T, commit bool) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is required for repository fixtures")
	}
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("TERM", "dumb")
	repo := t.TempDir()
	mustGit(t, repo, "init", "--initial-branch=main")
	if commit {
		gitFile(t, repo, "tracked", "base\n")
		mustGit(t, repo, "add", "tracked")
		mustGit(t, repo, "commit", "-m", "base")
	}
	return repo
}

func TestGitUnbornAndClean(t *testing.T) {
	repo := gitFixture(t, false)
	state := readGitStatus(repo)
	if !state.known || !state.unborn || state.branch != "main" {
		t.Fatalf("unborn: %+v", state)
	}
	if got := formatGitSegment(repo, false); got != "main new clean" {
		t.Fatalf("unborn prompt: %q", got)
	}
	gitFile(t, repo, "first", "first\n")
	mustGit(t, repo, "add", "first")
	mustGit(t, repo, "commit", "-m", "first")
	if got := formatGitSegment(repo, false); got != "main clean" {
		t.Fatalf("clean prompt: %q", got)
	}
}

func TestGitCountsFilesWithNewlinesAndStagedRename(t *testing.T) {
	repo := gitFixture(t, true)
	gitFile(t, repo, "tracked", "staged\n")
	mustGit(t, repo, "add", "tracked")
	gitFile(t, repo, "tracked", "modified after staging\n")
	gitFile(t, repo, "extra", "staged extra\n")
	mustGit(t, repo, "add", "extra")
	gitFile(t, repo, "untracked/a\n? fake-status", "one")
	gitFile(t, repo, "untracked/second", "two")
	state := readGitStatus(repo)
	if !state.known || state.staged != 2 || state.modified != 1 || state.untracked != 2 || state.conflicts != 0 {
		t.Fatalf("counts: %+v", state)
	}
	if got := formatGitSegment(repo, false); got != "main +2 !1 ?2" {
		t.Fatalf("dirty prompt: %q", got)
	}
	mustGit(t, repo, "add", "-A")
	mustGit(t, repo, "commit", "-m", "files")
	mustGit(t, repo, "mv", "untracked/a\n? fake-status", "renamed\n? another")
	state = readGitStatus(repo)
	if !state.known || state.staged != 1 || state.untracked != 0 || state.modified != 0 {
		t.Fatalf("rename source must not be parsed as a status record: %+v", state)
	}
}

func TestGitAheadBehindUsesOnlyLocalUpstream(t *testing.T) {
	repo := gitFixture(t, true)
	mustGit(t, repo, "branch", "upstream")
	mustGit(t, repo, "branch", "--set-upstream-to=upstream")
	gitFile(t, repo, "ahead", "local")
	mustGit(t, repo, "add", "ahead")
	mustGit(t, repo, "commit", "-m", "ahead")
	mustGit(t, repo, "checkout", "upstream")
	gitFile(t, repo, "behind", "upstream")
	mustGit(t, repo, "add", "behind")
	mustGit(t, repo, "commit", "-m", "behind")
	mustGit(t, repo, "checkout", "main")
	state := readGitStatus(repo)
	if !state.known || !state.upstream || !state.aheadKnown || state.ahead != 1 || state.behind != 1 {
		t.Fatalf("divergence: %+v", state)
	}
	if got := formatGitSegment(repo, false); got != "main ahead:1 behind:1 clean" {
		t.Fatalf("upstream prompt: %q", got)
	}
}

func TestGitLinkedWorktreeAndDetachedHead(t *testing.T) {
	repo := gitFixture(t, true)
	worktree := filepath.Join(t.TempDir(), "feature-tree")
	mustGit(t, repo, "worktree", "add", "-b", "feature", worktree)
	if got := formatGitSegment(worktree, false); got != "feature wt:feature-tree clean" {
		t.Fatalf("worktree prompt: %q", got)
	}
	gitFile(t, worktree, "nested/file", "dirty")
	branch, root := gitBranch(filepath.Join(worktree, "nested"))
	if branch != "feature" || root != worktree {
		t.Fatalf("nested worktree discovery: %q %q", branch, root)
	}
	mustGit(t, worktree, "checkout", "--detach")
	got := formatGitSegment(worktree, false)
	if !strings.HasPrefix(got, "detached@") || !strings.Contains(got, " wt:feature-tree ?1") {
		t.Fatalf("detached worktree: %q", got)
	}
	if strings.Contains(formatGitSegment(repo, false), "wt:") {
		t.Fatal("main checkout incorrectly marked as linked")
	}
}

func TestGitDistinctConflictsAndOperations(t *testing.T) {
	for _, operation := range []string{"merge", "rebase", "cherry-pick"} {
		t.Run(operation, func(t *testing.T) {
			repo := gitFixture(t, true)
			mustGit(t, repo, "checkout", "-b", "topic")
			gitFile(t, repo, "tracked", "topic\n")
			mustGit(t, repo, "commit", "-am", "topic")
			mustGit(t, repo, "checkout", "main")
			gitFile(t, repo, "tracked", "main\n")
			mustGit(t, repo, "commit", "-am", "main")
			if _, err := fixtureGit(t, repo, operation, "topic"); err == nil {
				t.Fatal("expected fixture conflict")
			}
			state := readGitStatus(repo)
			if !state.known || state.conflicts != 1 || state.staged != 0 || state.modified != 0 {
				t.Fatalf("distinct conflict count: %+v", state)
			}
			found := findGitRepo(repo)
			if got := gitOperation(found.dir); got != operation {
				t.Fatalf("operation: %q", got)
			}
			got := formatGitSegment(repo, false)
			if !strings.Contains(got, " "+operation+" ") || !strings.Contains(got, "conflict:1") || strings.Contains(got, "clean") {
				t.Fatalf("conflict prompt: %q", got)
			}
		})
	}
}

func TestGitReadDoesNotRefreshIndexOrRunFSMonitor(t *testing.T) {
	repo := gitFixture(t, true)
	index := filepath.Join(repo, ".git", "index")
	before, err := os.ReadFile(index)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(index)
	if err != nil {
		t.Fatal(err)
	}
	gitFile(t, repo, "tracked", "changed after index\n")
	// A shell hook would touch this fixture marker if status accidentally ran it.
	marker := filepath.Join(repo, "hook-ran")
	hook := filepath.Join(t.TempDir(), "fsmonitor")
	if err := os.WriteFile(hook, []byte("#!/bin/sh\ntouch '"+marker+"'\n"), 0700); err != nil {
		t.Fatal(err)
	}
	mustGit(t, repo, "config", "core.fsmonitor", hook)
	if state := readGitStatus(repo); !state.known || state.modified != 1 {
		t.Fatalf("read: %+v", state)
	}
	after, err := os.ReadFile(index)
	if err != nil {
		t.Fatal(err)
	}
	afterInfo, err := os.Stat(index)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) || !info.ModTime().Equal(afterInfo.ModTime()) {
		t.Fatal("prompt read refreshed the Git index")
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("prompt ran fsmonitor hook")
	}
}

func TestGitInheritedRepositoryAndTraceVariablesAreIgnored(t *testing.T) {
	repo := gitFixture(t, true)
	trace := filepath.Join(t.TempDir(), "trace")
	t.Setenv("GIT_DIR", filepath.Join(t.TempDir(), "wrong"))
	t.Setenv("GIT_INDEX_FILE", filepath.Join(repo, "wrong-index"))
	t.Setenv("GIT_TRACE", trace)
	if got := formatGitSegment(repo, false); got != "main clean" {
		t.Fatalf("inherited git environment affected local prompt: %q", got)
	}
	if _, err := os.Stat(trace); !os.IsNotExist(err) {
		t.Fatal("Git wrote a trace file")
	}
	for _, required := range []string{"GIT_ALLOW_PROTOCOL=", "GIT_NO_LAZY_FETCH=1", "GIT_OPTIONAL_LOCKS=0"} {
		if !strings.Contains(strings.Join(gitEnvironment(), "\n"), required) {
			t.Fatalf("missing guard %s", required)
		}
	}
}

func TestGitTimeoutAndCommandFailureAreUnknown(t *testing.T) {
	repo := gitFixture(t, true)
	bin := t.TempDir()
	git := filepath.Join(bin, "git")
	t.Setenv("PATH", bin)
	for _, script := range []string{"#!/bin/sh\nexec /bin/sleep 2\n", "#!/bin/sh\nexit 1\n", "#!/bin/sh\nexit 0\n"} {
		if err := os.WriteFile(git, []byte(script), 0700); err != nil {
			t.Fatal(err)
		}
		start := time.Now()
		got := formatGitSegment(repo, false)
		if got != "main status?" {
			t.Fatalf("failed/timed-out/empty status looked clean: %q", got)
		}
		if time.Since(start) > time.Second {
			t.Fatal("Git timeout did not bound the prompt")
		}
	}
}

func TestGitOutputBudgetAndMalformedPorcelain(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	output := gitOutput{cancel: cancel}
	if _, err := output.Write(make([]byte, gitOutputLimit+1)); err == nil || ctx.Err() == nil || output.Len() != 0 {
		t.Fatal("output limit did not cancel without retaining excess data")
	}
	// Hide WriterTo so io.Copy takes the same ReaderFrom/Write path as a pipe.
	output = gitOutput{cancel: cancel}
	reader := struct{ io.Reader }{bytes.NewReader(make([]byte, gitOutputLimit+1))}
	if _, err := io.Copy(&output, reader); err == nil || output.Len() > gitOutputLimit {
		t.Fatal("stream copy bypassed the output limit")
	}
	header := "# branch.oid abcdef1234\x00# branch.head main\x00"
	for _, tail := range []string{"1 broken\x00", "# branch.ab +? -2\x00", "? truncated", "2 R. N... 100644 100644 100644 a b R100 destination\x00", "u malformed\x00"} {
		if state := parseGitStatus([]byte(header + tail)); state.known {
			t.Fatalf("malformed status accepted: %q", tail)
		}
	}
	state := parseGitStatus([]byte(header + "# branch.upstream origin/main\x00"))
	if got := gitStatusText(state, false); stripColors(got) != "upstream? clean" {
		t.Fatalf("missing upstream comparison: %q", got)
	}
}

func TestGitPlainTerminalAndOutsideRepository(t *testing.T) {
	repo := gitFixture(t, true)
	t.Setenv("TERM", "linux")
	got := formatGitSegment(repo, true)
	for _, char := range got {
		if char > 127 {
			t.Fatalf("console glyph: %q", got)
		}
	}
	if !strings.Contains(got, "clean") {
		t.Fatalf("console clean label: %q", got)
	}
	if got := formatGitSegment(t.TempDir(), true); got != "" {
		t.Fatalf("outside repository: %q", got)
	}
	if got := gitLabel("tree\x1b\nname"); got != "tree??name" {
		t.Fatalf("control characters in label: %q", got)
	}
}
