package main

import (
	"fmt"
	"os"
	"strings"
	"unicode"
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
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return '?'
		}
		return r
	}, s)
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
	var plain strings.Builder
	for _, token := range promptTokens(s) {
		plain.WriteString(token.text)
	}
	return plain.String()
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
	_, cells := promptCells(s)
	width := 0
	for _, cell := range cells {
		width += cell.width
	}
	return width
}

type promptToken struct {
	raw, text string
	cell      int
}

// Parse only the formatting emitted by this program. In particular, %%F{2} is
// literal filename text, not a colour escape embedded in an escaped percent.
func promptTokens(s string) []promptToken {
	var tokens []promptToken
	for len(s) > 0 {
		n, plain := 0, ""
		if strings.HasPrefix(s, "%%") {
			n, plain = 2, "%"
		} else if strings.HasPrefix(s, "%f") || strings.HasPrefix(s, "%k") {
			n = 2
		} else if strings.HasPrefix(s, "%F{") || strings.HasPrefix(s, "%K{") {
			if end := strings.IndexByte(s, '}'); end >= 0 {
				n = end + 1
			}
		}
		if n == 0 {
			_, n = utf8.DecodeRuneInString(s)
			plain = s[:n]
		}
		tokens = append(tokens, promptToken{raw: s[:n], text: plain, cell: -1})
		s = s[n:]
	}
	return tokens
}

type promptCell struct {
	text  string
	width int
}

// Keep a base and its marks together when cropping. Emoji modifiers, flags and
// joined emoji occupy the same two-cell cluster on graphical terminals.
func promptCells(s string) ([]promptToken, []promptCell) {
	tokens := promptTokens(s)
	var cells []promptCell
	joinNext, regional := false, false
	for i := range tokens {
		if tokens[i].text == "" {
			continue
		}
		r, _ := utf8.DecodeRuneInString(tokens[i].text)
		width := runeCellWidth(r)
		flag := r >= 0x1f1e6 && r <= 0x1f1ff
		modifier := r >= 0x1f3fb && r <= 0x1f3ff
		attach := len(cells) > 0 && (width == 0 || joinNext || modifier || flag && regional)
		if attach {
			last := &cells[len(cells)-1]
			last.text += tokens[i].text
			if joinNext && width > last.width {
				last.width = width
			}
			if r == 0xfe0f || r == 0x20e3 || modifier || flag {
				last.width = max(last.width, 2)
			}
		} else {
			cells = append(cells, promptCell{text: tokens[i].text, width: width})
		}
		tokens[i].cell = len(cells) - 1
		joinNext = r == 0x200d
		regional = flag && !regional
	}
	return tokens, cells
}

func runeCellWidth(r rune) int {
	if unicode.IsControl(r) || unicode.In(r, unicode.Mn, unicode.Me, unicode.Cf) || r >= 0x1160 && r <= 0x11ff {
		return 0
	}
	lo, hi := 0, len(wideCells)
	for lo < hi {
		mid := (lo + hi) / 2
		if r < wideCells[mid][0] {
			hi = mid
		} else if r > wideCells[mid][1] {
			lo = mid + 1
		} else {
			return 2
		}
	}
	return 1
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

// Unicode 17 East_Asian_Width W/F ranges. Ambiguous characters use one cell.
// Source: https://www.unicode.org/Public/17.0.0/ucd/EastAsianWidth.txt
//
// UNICODE LICENSE V3
// COPYRIGHT AND PERMISSION NOTICE
// Copyright (c) 1991-2026 Unicode, Inc.
//
// NOTICE TO USER: Carefully read the following legal agreement. BY
// DOWNLOADING, INSTALLING, COPYING OR OTHERWISE USING DATA FILES, AND/OR
// SOFTWARE, YOU UNEQUIVOCALLY ACCEPT, AND AGREE TO BE BOUND BY, ALL OF THE
// TERMS AND CONDITIONS OF THIS AGREEMENT. IF YOU DO NOT AGREE, DO NOT
// DOWNLOAD, INSTALL, COPY, DISTRIBUTE OR USE THE DATA FILES OR SOFTWARE.
//
// Permission is hereby granted, free of charge, to any person obtaining a
// copy of data files and any associated documentation (the "Data Files") or
// software and any associated documentation (the "Software") to deal in the
// Data Files or Software without restriction, including without limitation
// the rights to use, copy, modify, merge, publish, distribute, and/or sell
// copies of the Data Files or Software, and to permit persons to whom the
// Data Files or Software are furnished to do so, provided that either (a)
// this copyright and permission notice appear with all copies of the Data
// Files or Software, or (b) this copyright and permission notice appear in
// associated Documentation.
//
// THE DATA FILES AND SOFTWARE ARE PROVIDED "AS IS", WITHOUT WARRANTY OF ANY
// KIND, EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF
// MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT OF
// THIRD PARTY RIGHTS.
//
// IN NO EVENT SHALL THE COPYRIGHT HOLDER OR HOLDERS INCLUDED IN THIS NOTICE
// BE LIABLE FOR ANY CLAIM, OR ANY SPECIAL INDIRECT OR CONSEQUENTIAL DAMAGES,
// OR ANY DAMAGES WHATSOEVER RESULTING FROM LOSS OF USE, DATA OR PROFITS,
// WHETHER IN AN ACTION OF CONTRACT, NEGLIGENCE OR OTHER TORTIOUS ACTION,
// ARISING OUT OF OR IN CONNECTION WITH THE USE OR PERFORMANCE OF THE DATA
// FILES OR SOFTWARE.
//
// Except as contained in this notice, the name of a copyright holder shall
// not be used in advertising or otherwise to promote the sale, use or other
// dealings in these Data Files or Software without prior written
// authorization of the copyright holder.
var wideCells = [...][2]rune{
	{0x1100, 0x115f},
	{0x231a, 0x231b},
	{0x2329, 0x232a},
	{0x23e9, 0x23ec},
	{0x23f0, 0x23f0},
	{0x23f3, 0x23f3},
	{0x25fd, 0x25fe},
	{0x2614, 0x2615},
	{0x2630, 0x2637},
	{0x2648, 0x2653},
	{0x267f, 0x267f},
	{0x268a, 0x268f},
	{0x2693, 0x2693},
	{0x26a1, 0x26a1},
	{0x26aa, 0x26ab},
	{0x26bd, 0x26be},
	{0x26c4, 0x26c5},
	{0x26ce, 0x26ce},
	{0x26d4, 0x26d4},
	{0x26ea, 0x26ea},
	{0x26f2, 0x26f3},
	{0x26f5, 0x26f5},
	{0x26fa, 0x26fa},
	{0x26fd, 0x26fd},
	{0x2705, 0x2705},
	{0x270a, 0x270b},
	{0x2728, 0x2728},
	{0x274c, 0x274c},
	{0x274e, 0x274e},
	{0x2753, 0x2755},
	{0x2757, 0x2757},
	{0x2795, 0x2797},
	{0x27b0, 0x27b0},
	{0x27bf, 0x27bf},
	{0x2b1b, 0x2b1c},
	{0x2b50, 0x2b50},
	{0x2b55, 0x2b55},
	{0x2e80, 0x2e99},
	{0x2e9b, 0x2ef3},
	{0x2f00, 0x2fd5},
	{0x2ff0, 0x303e},
	{0x3041, 0x3096},
	{0x3099, 0x30ff},
	{0x3105, 0x312f},
	{0x3131, 0x318e},
	{0x3190, 0x31e5},
	{0x31ef, 0x321e},
	{0x3220, 0x3247},
	{0x3250, 0xa48c},
	{0xa490, 0xa4c6},
	{0xa960, 0xa97c},
	{0xac00, 0xd7a3},
	{0xf900, 0xfaff},
	{0xfe10, 0xfe19},
	{0xfe30, 0xfe52},
	{0xfe54, 0xfe66},
	{0xfe68, 0xfe6b},
	{0xff01, 0xff60},
	{0xffe0, 0xffe6},
	{0x16fe0, 0x16fe4},
	{0x16ff0, 0x16ff6},
	{0x17000, 0x18cd5},
	{0x18cff, 0x18d1e},
	{0x18d80, 0x18df2},
	{0x1aff0, 0x1aff3},
	{0x1aff5, 0x1affb},
	{0x1affd, 0x1affe},
	{0x1b000, 0x1b122},
	{0x1b132, 0x1b132},
	{0x1b150, 0x1b152},
	{0x1b155, 0x1b155},
	{0x1b164, 0x1b167},
	{0x1b170, 0x1b2fb},
	{0x1d300, 0x1d356},
	{0x1d360, 0x1d376},
	{0x1f004, 0x1f004},
	{0x1f0cf, 0x1f0cf},
	{0x1f18e, 0x1f18e},
	{0x1f191, 0x1f19a},
	{0x1f200, 0x1f202},
	{0x1f210, 0x1f23b},
	{0x1f240, 0x1f248},
	{0x1f250, 0x1f251},
	{0x1f260, 0x1f265},
	{0x1f300, 0x1f320},
	{0x1f32d, 0x1f335},
	{0x1f337, 0x1f37c},
	{0x1f37e, 0x1f393},
	{0x1f3a0, 0x1f3ca},
	{0x1f3cf, 0x1f3d3},
	{0x1f3e0, 0x1f3f0},
	{0x1f3f4, 0x1f3f4},
	{0x1f3f8, 0x1f43e},
	{0x1f440, 0x1f440},
	{0x1f442, 0x1f4fc},
	{0x1f4ff, 0x1f53d},
	{0x1f54b, 0x1f54e},
	{0x1f550, 0x1f567},
	{0x1f57a, 0x1f57a},
	{0x1f595, 0x1f596},
	{0x1f5a4, 0x1f5a4},
	{0x1f5fb, 0x1f64f},
	{0x1f680, 0x1f6c5},
	{0x1f6cc, 0x1f6cc},
	{0x1f6d0, 0x1f6d2},
	{0x1f6d5, 0x1f6d8},
	{0x1f6dc, 0x1f6df},
	{0x1f6eb, 0x1f6ec},
	{0x1f6f4, 0x1f6fc},
	{0x1f7e0, 0x1f7eb},
	{0x1f7f0, 0x1f7f0},
	{0x1f90c, 0x1f93a},
	{0x1f93c, 0x1f945},
	{0x1f947, 0x1f9ff},
	{0x1fa70, 0x1fa7c},
	{0x1fa80, 0x1fa8a},
	{0x1fa8e, 0x1fac6},
	{0x1fac8, 0x1fac8},
	{0x1facd, 0x1fadc},
	{0x1fadf, 0x1faea},
	{0x1faef, 0x1faf8},
	{0x20000, 0x2fffd},
	{0x30000, 0x3fffd},
}
