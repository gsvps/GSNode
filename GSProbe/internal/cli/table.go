package cli

import (
	"fmt"
	"io"
	"strings"
)

type tableAlign int

const (
	alignLeft tableAlign = iota
	alignRight
)

type tableColumn struct {
	title string
	align tableAlign
	min   int
	max   int
}

type termTable struct {
	cols []tableColumn
	rows [][]string
}

func newKVTable(labelWidth int) *termTable {
	return &termTable{
		cols: []tableColumn{
			{title: "", align: alignLeft, min: labelWidth, max: labelWidth},
			{title: "", align: alignLeft, min: 8},
		},
	}
}

func (t *termTable) addRow(cells ...string) {
	if len(cells) == 0 {
		return
	}
	row := make([]string, len(t.cols))
	for i := range t.cols {
		if i < len(cells) {
			row[i] = cells[i]
		}
	}
	t.rows = append(t.rows, row)
}

func (t *termTable) addKV(label, value string) {
	t.addRow(label, value)
}

func (t *termTable) calcWidths(maxW int) []int {
	widths := make([]int, len(t.cols))
	for i, col := range t.cols {
		widths[i] = col.min
		if col.title != "" {
			widths[i] = max(widths[i], visibleWidth(col.title))
		}
	}
	for _, row := range t.rows {
		for i, cell := range row {
			if i >= len(widths) {
				continue
			}
			widths[i] = max(widths[i], visibleWidth(cell))
		}
	}
	for i, col := range t.cols {
		if col.max > 0 && widths[i] > col.max {
			widths[i] = col.max
		}
	}
	if len(widths) == 0 {
		return widths
	}
	border := len(widths)*3 + 1
	total := sumInts(widths) + border
	if total <= maxW {
		if len(widths) >= 2 {
			widths[len(widths)-1] += maxW - total
		}
		return widths
	}
	extra := total - maxW
	for extra > 0 {
		shrunk := false
		for i := len(widths) - 1; i >= 0 && extra > 0; i-- {
			if widths[i] > t.cols[i].min {
				widths[i]--
				extra--
				shrunk = true
			}
		}
		if !shrunk {
			break
		}
	}
	return widths
}

func sumInts(v []int) int {
	n := 0
	for _, x := range v {
		n += x
	}
	return n
}

func padCell(text string, width int, align tableAlign) string {
	if visibleWidth(text) > width {
		text = truncateRunes(stripANSI(text), width)
	}
	pad := width - visibleWidth(text)
	if pad <= 0 {
		return text
	}
	if align == alignRight {
		return strings.Repeat(" ", pad) + text
	}
	return text + strings.Repeat(" ", pad)
}

func (t *termTable) borderLine(left, mid, right string, widths []int) string {
	parts := make([]string, len(widths))
	for i, w := range widths {
		parts[i] = strings.Repeat("─", w+2)
	}
	return left + strings.Join(parts, mid) + right
}

func (t *termTable) render(maxW int) []string {
	if len(t.rows) == 0 {
		return nil
	}
	widths := t.calcWidths(maxW)
	hasHeader := false
	for _, col := range t.cols {
		if col.title != "" {
			hasHeader = true
			break
		}
	}

	var out []string
	out = append(out, clampTableLine(t.borderLine("┌", "┬", "┐", widths), maxW))
	if hasHeader {
		header := make([]string, len(widths))
		for i, col := range t.cols {
			header[i] = padCell(col.title, widths[i], col.align)
		}
		out = append(out, clampTableLine(renderTableRow("│", "│", "│", header, widths, t.cols), maxW))
		out = append(out, clampTableLine(t.borderLine("├", "┼", "┤", widths), maxW))
	}
	for _, row := range t.rows {
		out = append(out, clampTableLine(renderTableRow("│", "│", "│", row, widths, t.cols), maxW))
	}
	out = append(out, clampTableLine(t.borderLine("└", "┴", "┘", widths), maxW))
	return out
}

func clampTableLine(line string, maxW int) string {
	if visibleWidth(line) <= maxW {
		return line
	}
	return truncateRunes(stripANSI(line), maxW)
}

func renderTableRow(left, mid, right string, cells []string, widths []int, cols []tableColumn) string {
	parts := make([]string, len(widths))
	for i := range widths {
		text := ""
		if i < len(cells) {
			text = cells[i]
		}
		align := alignLeft
		if i < len(cols) {
			align = cols[i].align
		}
		parts[i] = " " + padCell(text, widths[i], align) + " "
	}
	return left + strings.Join(parts, mid) + right
}

func tableLines(maxW int, table *termTable) []string {
	if table == nil {
		return nil
	}
	return table.render(maxW)
}

func appendHeading(lines []string, title string) []string {
	return append(lines, colorHeading("▸ "+title))
}

func appendTable(lines []string, width int, table *termTable) []string {
	if table == nil {
		return lines
	}
	return append(lines, tableLines(width, table)...)
}

func appendScoreLine(lines []string, width int, label string, score int) []string {
	return append(lines, padRight(fmt.Sprintf("%s %s", label, colorScore(score)), width))
}

func printTable(w io.Writer, maxW int, table *termTable) {
	for _, line := range table.render(maxW) {
		fmt.Fprintln(w, line)
	}
}

func printSectionTitle(w io.Writer, title string, score int) {
	fmt.Fprintf(w, "\n%s  %s\n", colorHeading(title), colorScore(score))
}

func printSubHeading(w io.Writer, title string) {
	fmt.Fprintf(w, "\n%s\n", colorHeading("▸ "+title))
}

func newMatrixTable(headers []string) *termTable {
	cols := make([]tableColumn, len(headers))
	for i, h := range headers {
		min := 8
		if i == 0 {
			min = 4
		}
		cols[i] = tableColumn{title: h, align: alignLeft, min: min}
	}
	return &termTable{cols: cols}
}
