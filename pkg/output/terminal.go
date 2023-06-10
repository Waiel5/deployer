package output

import (
	"fmt"
	"os"
	"strings"

	"github.com/fatih/color"
)

// Terminal handles formatted console output with colors and spinners.
type Terminal struct {
	verbose bool
}

// NewTerminal creates a new terminal output handler.
func NewTerminal(verbose bool) *Terminal {
	return &Terminal{verbose: verbose}
}

// Header prints a bold header line.
func (t *Terminal) Header(format string, args ...interface{}) {
	bold := color.New(color.Bold, color.FgWhite)
	msg := fmt.Sprintf(format, args...)
	bold.Fprintf(os.Stderr, "\n%s\n", msg)
	fmt.Fprintf(os.Stderr, "%s\n\n", strings.Repeat("─", len(msg)))
}

// Info prints an informational message.
func (t *Terminal) Info(format string, args ...interface{}) {
	cyan := color.New(color.FgCyan)
	cyan.Fprintf(os.Stderr, "  ℹ ")
	fmt.Fprintf(os.Stderr, format+"\n", args...)
}

// Success prints a success message.
func (t *Terminal) Success(format string, args ...interface{}) {
	green := color.New(color.FgGreen)
	green.Fprintf(os.Stderr, "  ✓ ")
	fmt.Fprintf(os.Stderr, format+"\n", args...)
}

// Warn prints a warning message.
func (t *Terminal) Warn(format string, args ...interface{}) {
	yellow := color.New(color.FgYellow)
	yellow.Fprintf(os.Stderr, "  ⚠ ")
	fmt.Fprintf(os.Stderr, format+"\n", args...)
}

// Error prints an error message.
func (t *Terminal) Error(format string, args ...interface{}) {
	red := color.New(color.FgRed)
	red.Fprintf(os.Stderr, "  ✗ ")
	fmt.Fprintf(os.Stderr, format+"\n", args...)
}

// Debug prints a debug message (only when verbose is enabled).
func (t *Terminal) Debug(format string, args ...interface{}) {
	if !t.verbose {
		return
	}
	dim := color.New(color.FgWhite, color.Faint)
	dim.Fprintf(os.Stderr, "  … ")
	fmt.Fprintf(os.Stderr, format+"\n", args...)
}

// StartSpinner prints a "working" indicator for a long-running operation.
func (t *Terminal) StartSpinner(format string, args ...interface{}) {
	blue := color.New(color.FgBlue)
	blue.Fprintf(os.Stderr, "  ● ")
	fmt.Fprintf(os.Stderr, format+"\n", args...)
}

// StopSpinner prints the completion of a long-running operation.
func (t *Terminal) StopSpinner(format string, args ...interface{}) {
	green := color.New(color.FgGreen)
	green.Fprintf(os.Stderr, "  ✓ ")
	fmt.Fprintf(os.Stderr, format+"\n", args...)
}

// Table prints data in a formatted ASCII table.
func (t *Terminal) Table(headers []string, rows [][]string) {
	if len(rows) == 0 {
		t.Info("No data to display")
		return
	}

	// Calculate column widths
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = len(h)
	}
	for _, row := range rows {
		for i, cell := range row {
			if i < len(widths) && len(cell) > widths[i] {
				widths[i] = len(cell)
			}
		}
	}

	// Build separator
	sepParts := make([]string, len(widths))
	for i, w := range widths {
		sepParts[i] = strings.Repeat("─", w+2)
	}
	separator := "┼" + strings.Join(sepParts, "┼") + "┼"
	topBorder := "┌" + strings.Join(sepParts, "┬") + "┐"
	bottomBorder := "└" + strings.Join(sepParts, "┴") + "┘"

	// Print header
	fmt.Fprintln(os.Stderr, topBorder)
	headerCells := make([]string, len(headers))
	for i, h := range headers {
		headerCells[i] = fmt.Sprintf(" %-*s ", widths[i], h)
	}
	bold := color.New(color.Bold)
	fmt.Fprint(os.Stderr, "│")
	for _, cell := range headerCells {
		bold.Fprint(os.Stderr, cell)
		fmt.Fprint(os.Stderr, "│")
	}
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, separator)

	// Print rows
	for _, row := range rows {
		fmt.Fprint(os.Stderr, "│")
		for i, cell := range row {
			if i < len(widths) {
				// Colorize status cells
				formatted := fmt.Sprintf(" %-*s ", widths[i], cell)
				switch cell {
				case "RUNNING":
					color.New(color.FgGreen).Fprint(os.Stderr, formatted)
				case "STOPPED":
					color.New(color.FgRed).Fprint(os.Stderr, formatted)
				case "UNREACHABLE":
					color.New(color.FgYellow).Fprint(os.Stderr, formatted)
				case "NOT FOUND":
					color.New(color.FgWhite, color.Faint).Fprint(os.Stderr, formatted)
				default:
					fmt.Fprint(os.Stderr, formatted)
				}
				fmt.Fprint(os.Stderr, "│")
			}
		}
		fmt.Fprintln(os.Stderr)
	}

	fmt.Fprintln(os.Stderr, bottomBorder)
	fmt.Fprintf(os.Stderr, "\n  %d service(s) across %d row(s)\n\n",
		countUnique(rows, 0), len(rows))
}

func countUnique(rows [][]string, col int) int {
	seen := make(map[string]bool)
	for _, row := range rows {
		if col < len(row) {
			seen[row[col]] = true
		}
	}
	return len(seen)
}
