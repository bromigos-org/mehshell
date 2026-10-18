package main

import (
	"fmt"
	"os"
	"strings"
	"unicode/utf8"
)

const (
	cCyan    = 75
	cBlue    = 39
	cMagenta = 170
	cGreen   = 76
	cRed     = 196
	cYellow  = 220
	cOrange  = 208
	cGray    = 242
)

// The Linux virtual console uses a small bitmap font and an eight-colour palette.
// Keep this independent of saved options so the same config works in desktop shells.
func basicTerminal() bool {
	term := os.Getenv("TERM")
	return term == "linux" || term == "dumb"
}

func promptArrow() string {
	if basicTerminal() {
		return ">"
	}
	return "❯"
}

func fc(color int, s string) string {
	if s == "" {
		return ""
	}
	s = strings.ReplaceAll(s, "%", "%%")
	if os.Getenv("TERM") == "dumb" {
		return s
	}
	if os.Getenv("TERM") == "linux" {
		switch color {
		case cCyan:
			color = 6
		case cBlue:
			color = 4
		case cMagenta:
			color = 5
		case cGreen:
			color = 2
		case cRed:
			color = 1
		case cYellow, cOrange:
			color = 3
		default:
			color = 7
		}
	}
	return fmt.Sprintf("%%F{%d}%s%%f", color, s)
}

type seg struct {
	text  string
	order int
	bg    int
}

func stripColors(s string) string {
	for _, prefix := range []string{"%F{", "%K{"} {
		for {
			idx := strings.Index(s, prefix)
			if idx == -1 {
				break
			}
			end := strings.Index(s[idx:], "}")
			if end == -1 {
				break
			}
			s = s[:idx] + s[idx+end+1:]
		}
	}
	for _, esc := range []string{"%f", "%k"} {
		s = strings.ReplaceAll(s, esc, "")
	}
	s = strings.ReplaceAll(s, "%%", "%")
	return s
}

func escPercent(s string) string {
	return strings.ReplaceAll(s, "%", "%%")
}

func contrastFg(bg int) int {
	switch bg {
	case 178, 136, 220, 3, 76, 2, 208:
		return 0
	default:
		return 255
	}
}

func renderPowerlineLeft(segs []seg, rainbow bool) string {
	if len(segs) == 0 {
		return ""
	}
	var b strings.Builder
	if rainbow {
		for i, s := range segs {
			fg := contrastFg(s.bg)
			plain := stripColors(s.text)
			b.WriteString(fmt.Sprintf("%%K{%d}%%F{%d} %s ", s.bg, fg, escPercent(plain)))
			if i < len(segs)-1 {
				b.WriteString(fmt.Sprintf("%%F{%d}%%K{%d}\ue0b0", s.bg, segs[i+1].bg))
			} else {
				b.WriteString(fmt.Sprintf("%%f%%k%%F{%d}\ue0b0%%f", s.bg))
			}
		}
	} else {
		b.WriteString(fmt.Sprintf("%%K{%d}", 238))
		for i, s := range segs {
			b.WriteString(fmt.Sprintf(" %s ", s.text))
			if i < len(segs)-1 {
				b.WriteString(fmt.Sprintf("%%F{%d}\ue0b1%%f", 246))
			}
		}
		b.WriteString(fmt.Sprintf("%%f%%k%%F{%d}\ue0b0%%f", 238))
	}
	return b.String()
}

func renderPowerlineRight(segs []seg, rainbow bool) string {
	if len(segs) == 0 {
		return ""
	}
	var b strings.Builder
	if rainbow {
		for _, s := range segs {
			fg := contrastFg(s.bg)
			plain := stripColors(s.text)
			b.WriteString(fmt.Sprintf("%%F{%d}\ue0b2%%K{%d}%%F{%d} %s %%f", s.bg, s.bg, fg, escPercent(plain)))
		}
		b.WriteString("%k")
	} else {
		b.WriteString(fmt.Sprintf("%%F{%d}\ue0b2%%K{%d}", 238, 238))
		for i, s := range segs {
			b.WriteString(fmt.Sprintf(" %s ", s.text))
			if i < len(segs)-1 {
				b.WriteString(fmt.Sprintf("%%F{%d}\ue0b3%%f", 246))
			}
		}
		b.WriteString("%f%k")
	}
	return b.String()
}

func visibleWidth(s string) int {
	stripped := s
	for _, prefix := range []string{"%F{", "%K{"} {
		for {
			idx := strings.Index(stripped, prefix)
			if idx == -1 {
				break
			}
			end := strings.Index(stripped[idx:], "}")
			if end == -1 {
				break
			}
			stripped = stripped[:idx] + stripped[idx+end+1:]
		}
	}
	for _, esc := range []string{"%f", "%k"} {
		stripped = strings.ReplaceAll(stripped, esc, "")
	}
	stripped = strings.ReplaceAll(stripped, "%%", "%")
	return utf8.RuneCountInString(stripped)
}

func escShell(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "'", "\\'")
	return s
}

func joinSegs(segs []seg, sep string) string {
	parts := make([]string, len(segs))
	for i, s := range segs {
		parts[i] = s.text
	}
	return strings.Join(parts, sep)
}
