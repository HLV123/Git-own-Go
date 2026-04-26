package color

import (
	"fmt"
	"os"
	"runtime"
)

// Color codes
const (
	reset   = "\033[0m"
	bold    = "\033[1m"
	red     = "\033[31m"
	green   = "\033[32m"
	yellow  = "\033[33m"
	blue    = "\033[34m"
	magenta = "\033[35m"
	cyan    = "\033[36m"
	white   = "\033[37m"
)

// Enabled controls whether color output is used.
var Enabled = isColorSupported()

func isColorSupported() bool {
	if runtime.GOOS == "windows" {
		// Windows Terminal and modern PowerShell support ANSI
		// Check for NO_COLOR env var
		if os.Getenv("NO_COLOR") != "" {
			return false
		}
		// Check TERM or WT_SESSION (Windows Terminal)
		if os.Getenv("WT_SESSION") != "" || os.Getenv("TERM") != "" {
			return true
		}
		// Default enable for Windows — modern terminals handle it
		return true
	}
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	return true
}

func wrap(code, s string) string {
	if !Enabled {
		return s
	}
	return code + s + reset
}

func Bold(s string) string    { return wrap(bold, s) }
func Red(s string) string     { return wrap(red, s) }
func Green(s string) string   { return wrap(green, s) }
func Yellow(s string) string  { return wrap(yellow, s) }
func Blue(s string) string    { return wrap(blue, s) }
func Magenta(s string) string { return wrap(magenta, s) }
func Cyan(s string) string    { return wrap(cyan, s) }

// Hash prints a short 7-char hash in yellow.
func Hash(h string) string {
	if len(h) > 7 {
		return Yellow(h[:7])
	}
	return Yellow(h)
}

// HashFull prints full hash in yellow.
func HashFull(h string) string { return Yellow(h) }

// CommitLine formats a log line.
func CommitLine(hash, msg string) string {
	short := hash
	if len(hash) > 7 {
		short = hash[:7]
	}
	return fmt.Sprintf("%s %s", Yellow(short), msg)
}

// Added formats an added line (green +).
func Added(line string) string { return Green("+ " + line) }

// Removed formats a removed line (red -).
func Removed(line string) string { return Red("- " + line) }

// Header formats a section header.
func Header(s string) string { return Bold(Cyan(s)) }

// BranchCurrent formats the current branch marker.
func BranchCurrent(name string) string { return Green("* " + name) }

// BranchOther formats a non-current branch.
func BranchOther(name string) string { return "  " + name }

// Tag formats a tag name.
func TagName(name string) string { return Magenta(name) }

// Conflict formats a conflict marker.
func Conflict(s string) string { return Bold(Red(s)) }
