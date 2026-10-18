package main

import (
	"reflect"
	"strings"
	"testing"
)

func TestTerminalCellWidths(t *testing.T) {
	t.Setenv("TERM", "xterm-kitty")
	for _, fixture := range []struct {
		text  string
		width int
	}{
		{"cafe\u0301", 4}, {"\u0301", 0}, {"项目/日本語", 11},
		{"ＡＢＣ", 6}, {"☁", 1}, {"☁\ufe0f", 2}, {"👩\u200d💻", 2},
		{"👍🏽", 2}, {"🇺🇸", 2}, {"1\ufe0f\u20e3", 2},
		{"\ue0b0", 1}, {"50%F{2}%f", 9},
	} {
		if got := visibleWidth(fc(cBlue, fixture.text)); got != fixture.width {
			t.Errorf("%q: width %d, want %d", fixture.text, got, fixture.width)
		}
		if got := stripColors(fc(cBlue, fixture.text)); got != fixture.text {
			t.Errorf("literal text changed: %q -> %q", fixture.text, got)
		}
	}
}

func TestControlCharactersCannotBreakPromptLines(t *testing.T) {
	t.Setenv("TERM", "xterm-kitty")
	got := stripColors(fc(cBlue, "repo\nbranch\t\x1b[31m\u0085"))
	if got != "repo?branch??[31m?" {
		t.Fatalf("control bytes were not made printable: %q", got)
	}
}

func TestWideLayoutKeepsStylesAndSegments(t *testing.T) {
	t.Setenv("TERM", "xterm-kitty")
	left := []seg{{text: fc(cGreen, "bromigOS"), order: 0, bg: 24}, {text: fc(cBlue, "project"), order: 1, bg: 31}}
	right := []seg{{text: fc(cGray, "12:34"), order: 10, bg: 238}}
	for _, style := range []string{"lean", "classic", "rainbow", "console"} {
		line := layoutPrompt(left, right, 120, style)
		if visibleWidth(line) != 119 || !strings.Contains(stripColors(line), "project") || !strings.HasSuffix(stripColors(line), "12:34") && style != "classic" && style != "rainbow" {
			t.Fatalf("%s wide layout: %q", style, line)
		}
		if (style == "classic" || style == "rainbow") && !strings.ContainsRune(line, '\ue0b0') {
			t.Fatalf("%s lost its powerline style: %q", style, line)
		}
	}
}

func TestClockGoesBeforeActiveEnvironment(t *testing.T) {
	t.Setenv("TERM", "xterm-kitty")
	line := layoutPrompt([]seg{{text: "project", order: 1}}, []seg{{text: "env:production", order: 0}, {text: "12:34", order: 10}}, 28, "lean")
	plain := stripColors(line)
	if !strings.Contains(plain, "env:production") || strings.Contains(plain, "12:34") {
		t.Fatalf("clock displaced active environment: %q", plain)
	}
}

func TestPathAndBranchKeepIdentityAndStatusWhenShortened(t *testing.T) {
	t.Setenv("TERM", "xterm-kitty")
	path := seg{text: fc(cBlue, "~/work/a-very-long-parent-folder/project/src"), order: 1}
	if got := stripColors(shortenSegment(path, 16)); got != "…/project/src" {
		t.Fatalf("lost current path components: %q", got)
	}
	branch := seg{text: fc(cMagenta, "feature/a-very-long-branch") + " " + fc(cRed, "!3"), order: 2}
	got := shortenSegment(branch, 16)
	if visibleWidth(got) > 16 || !strings.Contains(stripColors(got), "feature") || !strings.HasSuffix(stripColors(got), "!3") || !strings.Contains(got, fc(cRed, "!3")) {
		t.Fatalf("lost branch identity or coloured status: %q", got)
	}
}

func TestNarrowLayoutRetainsSessionDirectoryAndGame(t *testing.T) {
	t.Setenv("TERM", "xterm-kitty")
	left := []seg{{text: fc(cRed, "ROOT"), order: -2}, {text: "bromigOS", order: 0}, {text: "project", order: 1}, {text: "feature/xx", order: 2}, {text: "go 1.25", order: 5}}
	right := []seg{{text: "GAME", order: -1}, {text: "12:34", order: 10}}
	line := stripColors(layoutPrompt(left, right, 22, "rainbow"))
	for _, required := range []string{"ROOT", "project", "GAME"} {
		if !strings.Contains(line, required) {
			t.Fatalf("lost %s in narrow layout: %q", required, line)
		}
	}
	if strings.Contains(line, "12:34") || strings.Contains(line, "go 1.25") {
		t.Fatalf("optional context survived over essential identity: %q", line)
	}
}

func TestLayoutsNeverWrapOrMutateInputs(t *testing.T) {
	for _, terminal := range []string{"linux", "dumb", "xterm-kitty"} {
		t.Setenv("TERM", terminal)
		left := []seg{{text: fc(cRed, "SSH user@example"), order: -2}, {text: fc(cGreen, "bromigOS"), order: 0}, {text: fc(cBlue, "~/项目/cafe\u0301/very-long-directory"), order: 1}, {text: fc(cMagenta, "feature/50%F{2}extra-long-branch") + fc(cRed, " !2"), order: 2}, {text: "go 1.25", order: 5}}
		right := []seg{{text: "GAME", order: -1}, {text: "production", order: 2}, {text: "12:34", order: 10}}
		beforeLeft, beforeRight := append([]seg(nil), left...), append([]seg(nil), right...)
		for _, style := range []string{"lean", "classic", "rainbow", "console"} {
			for columns := 1; columns <= 180; columns++ {
				line := layoutPrompt(left, right, columns, style)
				if visibleWidth(line) >= columns || strings.ContainsAny(line, "\r\n\t\x1b") {
					t.Fatalf("%s %s %d columns wrapped: %q (width %d)", terminal, style, columns, line, visibleWidth(line))
				}
				if basicTerminal() && strings.ContainsAny(line, "…╭─\ue0b0\ue0b2") {
					t.Fatalf("font-dependent decoration in basic terminal: %q", line)
				}
			}
		}
		if !reflect.DeepEqual(left, beforeLeft) || !reflect.DeepEqual(right, beforeRight) {
			t.Fatal("layout changed caller's segment slices")
		}
	}
}

func TestCroppingKeepsCombiningMarksPercentTextAndPromptMark(t *testing.T) {
	t.Setenv("TERM", "xterm-kitty")
	if got := stripColors(shortenEnd(fc(cBlue, "e\u0301项目"), 1)); got != "e\u0301" {
		t.Fatalf("split combining cluster: %q", got)
	}
	if got := stripColors(fitPromptText(fc(cRed, "prefix ")+fc(cGreen, ">"), 1)); got != ">" {
		t.Fatalf("input marker missing: %q", got)
	}
	if got := fitPromptText(fc(cRed, "x"), 0); got != "" {
		t.Fatalf("zero-column crop: %q", got)
	}
	if got := stripColors(shortenEnd(fc(cBlue, "%%F{2}"), 3)); got != "%%F" {
		t.Fatalf("literal percent parsed as formatting: %q", got)
	}
}

func TestAbsurdColumnCountIsBounded(t *testing.T) {
	t.Setenv("TERM", "xterm-kitty")
	line := layoutPrompt([]seg{{text: "project", order: 1}}, []seg{{text: "time", order: 10}}, int(^uint(0)>>1), "lean")
	if visibleWidth(line) != 4095 {
		t.Fatalf("unexpected bounded width: %d", visibleWidth(line))
	}
}
