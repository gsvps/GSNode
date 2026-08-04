package cli

import (
	"os"
	"strconv"
	"strings"
	"unicode/utf8"
)

func terminalWidth() int {
	if c := strings.TrimSpace(os.Getenv("COLUMNS")); c != "" {
		if w, err := strconv.Atoi(c); err == nil && w >= 80 {
			return w
		}
	}
	return 136
}

func runeWidth(s string) int {
	return utf8.RuneCountInString(s)
}

func truncateRunes(s string, max int) string {
	if max <= 0 {
		return ""
	}
	if runeWidth(s) <= max {
		return s
	}
	if max == 1 {
		return "…"
	}
	out := make([]rune, 0, max)
	for _, r := range s {
		if len(out) >= max-1 {
			break
		}
		out = append(out, r)
	}
	return string(out) + "…"
}

func padRight(s string, width int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	w := visibleWidth(s)
	if w > width {
		return truncateRunes(stripANSI(s), width)
	}
	return s + strings.Repeat(" ", width-w)
}

func hyperlinkSupported() bool {
	if os.Getenv("GSNODE_NO_HYPERLINK") != "" || os.Getenv("NO_HYPERLINK") != "" {
		return false
	}
	if stat, err := os.Stdout.Stat(); err == nil {
		if (stat.Mode() & os.ModeCharDevice) != 0 {
			return true
		}
	}
	if os.Getenv("WT_SESSION") != "" || os.Getenv("TERM_PROGRAM") != "" {
		return true
	}
	return false
}

func hyperlink(url, label string) string {
	if url == "" {
		return label
	}
	if label == "" {
		label = url
	}
	if !hyperlinkSupported() {
		return label
	}
	return "\033]8;;" + url + "\033\\" + label + "\033]8;;\033\\"
}
