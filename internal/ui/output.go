package ui

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// Table represents a simple text table
type Table struct {
	Headers []string
	Rows    [][]string
}

// NewTable creates a new table
func NewTable(headers ...string) *Table {
	return &Table{
		Headers: headers,
		Rows:    make([][]string, 0),
	}
}

// AddRow adds a row to the table
func (t *Table) AddRow(cells ...string) {
	t.Rows = append(t.Rows, cells)
}

// Print prints the table to stdout
func (t *Table) Print() {
	if len(t.Rows) == 0 {
		return
	}

	// Calculate column widths
	widths := make([]int, len(t.Headers))
	for i, header := range t.Headers {
		widths[i] = len(header)
	}
	for _, row := range t.Rows {
		for i, cell := range row {
			if i < len(widths) && len(cell) > widths[i] {
				widths[i] = len(cell)
			}
		}
	}

	// Print header
	headerParts := make([]string, len(t.Headers))
	for i, header := range t.Headers {
		headerParts[i] = padRight(header, widths[i])
	}
	fmt.Println(strings.Join(headerParts, "  "))

	// Print rows
	for _, row := range t.Rows {
		rowParts := make([]string, len(row))
		for i, cell := range row {
			if i < len(widths) {
				rowParts[i] = padRight(cell, widths[i])
			} else {
				rowParts[i] = cell
			}
		}
		fmt.Println(strings.Join(rowParts, "  "))
	}
}

func padRight(s string, length int) string {
	if len(s) >= length {
		return s
	}
	return s + strings.Repeat(" ", length-len(s))
}

// PrintJSON prints data as JSON
func PrintJSON(v interface{}) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(v)
}
