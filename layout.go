package main

import (
	"sort"
	"strings"
)

// layoutPrompt leaves the last terminal cell unused so the following newline
// cannot trigger an extra wrapped line. It never changes the caller's segments.
func layoutPrompt(left, right []seg, columns int, style string) string {
	if columns <= 1 {
		return ""
	}
	budget := min(columns-1, 4095)
	left, right = copySegments(left), copySegments(right)
	if basicTerminal() {
		style = "console"
	}
	line, fits := fitPrompt(left, right, budget, style)
	if fits {
		return line
	}

	// Compress labels before removing useful context. A path keeps its final
	// components; a branch keeps both its beginning and its status suffix.
	for i := range left {
		limit := 0
		switch left[i].order {
		case 1:
			limit = max(16, budget/2)
		case 2:
			limit = max(12, budget/3)
		}
		if limit > 0 && visibleWidth(left[i].text) > limit {
			left[i].text = shortenSegment(left[i], limit)
			if line, fits = fitPrompt(left, right, budget, style); fits {
				return line
			}
		}
	}

	// Clock, battery, timing and language versions go before active environments.
	// Negative orders are session/status warnings and are never optional.
	for _, item := range []struct {
		right bool
		order int
	}{
		{true, 10}, {true, 8}, {true, 9},
		{false, 8}, {false, 7}, {false, 6}, {false, 5}, {false, 4}, {false, 3},
		{true, 4}, {true, 3}, {true, 7}, {true, 6}, {true, 5},
		{true, 0}, {true, 1}, {true, 2}, {false, 0},
	} {
		if item.right {
			right = withoutOrder(right, item.order)
		} else {
			left = withoutOrder(left, item.order)
		}
		if line, fits = fitPrompt(left, right, budget, style); fits {
			return line
		}
	}

	// Decorations cost space too. Keep the configured style whenever it fits,
	// then use plain text for the smallest layouts instead of wrapping.
	if line, fits = fitPrompt(left, right, budget, "console"); fits {
		return line
	}
	compactBase := copySegments(left)
	for _, item := range []struct{ order, minimum int }{{2, 4}, {1, 8}} {
		left = shrinkOrder(left, right, budget, item.order, item.minimum)
		if line, fits = fitPrompt(left, right, budget, "console"); fits {
			return line
		}
	}
	left = withoutOrder(compactBase, 2)
	left = shrinkOrder(left, right, budget, 1, 1)
	if line, fits = fitPrompt(left, right, budget, "console"); fits {
		return line
	}

	// At very small sizes, session warnings outrank branch and brand labels.
	// Retain the current directory whenever any space remains after warnings.
	var essential []seg
	for sideIndex, side := range [][]seg{left, right} {
		for _, item := range side {
			if item.order < 0 || item.order == 1 && sideIndex == 0 {
				essential = append(essential, item)
			}
		}
	}
	sort.SliceStable(essential, func(i, j int) bool { return essential[i].order < essential[j].order })
	return shortenEnd(joinSegs(essential, " "), budget)
}

func shrinkOrder(left, right []seg, budget, order, minimum int) []seg {
	for i := range left {
		if left[i].order != order {
			continue
		}
		excess := promptWidth(left, right, "console") - budget
		width := visibleWidth(left[i].text)
		if excess > 0 && width > minimum {
			left[i].text = shortenSegment(left[i], max(minimum, width-excess))
		}
	}
	return left
}

func copySegments(items []seg) []seg {
	result := make([]seg, 0, len(items))
	for _, item := range items {
		if item.text != "" {
			result = append(result, item)
		}
	}
	sort.SliceStable(result, func(i, j int) bool { return result[i].order < result[j].order })
	return result
}

func withoutOrder(items []seg, order int) []seg {
	for i := 0; i < len(items); {
		if items[i].order == order {
			items = append(items[:i], items[i+1:]...)
		} else {
			i++
		}
	}
	return items
}

func promptSides(left, right []seg, style string) (string, string) {
	switch style {
	case "classic", "rainbow":
		return renderPowerlineLeft(left, style == "rainbow"), renderPowerlineRight(right, style == "rainbow")
	case "console":
		return joinSegs(left, " "), joinSegs(right, "  ")
	default:
		return fc(cCyan, "╭─") + " " + joinSegs(left, " "), joinSegs(right, "  ")
	}
}

func promptWidth(left, right []seg, style string) int {
	l, r := promptSides(left, right, style)
	width := visibleWidth(l) + visibleWidth(r)
	if l != "" && r != "" {
		width++
	}
	return width
}

func fitPrompt(left, right []seg, budget int, style string) (string, bool) {
	l, r := promptSides(left, right, style)
	lw, rw := visibleWidth(l), visibleWidth(r)
	gap := 0
	if l != "" && r != "" {
		gap = 1
	}
	if lw+rw+gap > budget {
		return "", false
	}
	if r == "" {
		return l, true
	}
	return l + strings.Repeat(" ", budget-lw-rw) + r, true
}

func omission() string {
	if basicTerminal() {
		return "..."
	}
	return "…"
}

// cropCells omits complete character clusters, leaving every colour escape in
// place. A retained Git status therefore keeps its colour after branch cropping.
func cropCells(tokens []promptToken, first, last int, mark string) string {
	var out strings.Builder
	marked := false
	for _, token := range tokens {
		if token.cell < 0 || token.cell < first || token.cell >= last {
			out.WriteString(token.raw)
		} else if !marked {
			out.WriteString(mark)
			marked = true
		}
	}
	return out.String()
}

func shortenSegment(item seg, width int) string {
	if visibleWidth(item.text) <= width {
		return item.text
	}
	tokens, cells := promptCells(item.text)
	mark := omission()
	if width < visibleWidth(mark) {
		mark = strings.Repeat(".", max(0, width))
	}
	room := max(0, width-visibleWidth(mark))
	if item.order == 1 {
		// Keep complete path components whenever the last component fits.
		start, used := len(cells), 0
		for start > 0 && used+cells[start-1].width <= room {
			start--
			used += cells[start].width
		}
		for i := start; i < len(cells); i++ {
			if cells[i].text == "/" {
				start = i
				break
			}
		}
		return cropCells(tokens, 0, start, mark)
	}
	// Keep the branch start and the suffix that carries dirty/ahead/behind counts.
	first, used := 0, 0
	for first < len(cells) && used+cells[first].width <= room/2 {
		used += cells[first].width
		first++
	}
	last := len(cells)
	for last > first && used+cells[last-1].width <= room {
		last--
		used += cells[last].width
	}
	return cropCells(tokens, first, last, mark)
}

func shortenEnd(text string, width int) string {
	if visibleWidth(text) <= width {
		return text
	}
	tokens, cells := promptCells(text)
	end, used := 0, 0
	for end < len(cells) && used+cells[end].width <= width {
		used += cells[end].width
		end++
	}
	return cropCells(tokens, end, len(cells), "")
}

// fitPromptText keeps the suffix (including the input mark) for the input line.
// Callers should omit whole optional labels before using this last-resort crop.
func fitPromptText(text string, width int) string {
	if width <= 0 {
		return ""
	}
	if visibleWidth(text) <= width {
		return text
	}
	tokens, cells := promptCells(text)
	start, used := len(cells), 0
	for start > 0 && used+cells[start-1].width <= width {
		start--
		used += cells[start].width
	}
	return cropCells(tokens, 0, start, "")
}
