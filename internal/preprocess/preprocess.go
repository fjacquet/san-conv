// Package preprocess cleans raw, possibly terminal-captured configuration text
// before it is handed to a parser. SAN switch "show running-config" output is
// frequently captured from an interactive terminal session, so it carries
// pager prompts ("--More--"), ANSI/VT100 escape sequences, and stray carriage
// returns that glue config lines onto the prompt line. This package strips all
// of that, producing the clean line slice a parser expects.
package preprocess

import (
	"io"
	"regexp"
	"strings"
)

// ansiCSI matches an ANSI/VT100 CSI escape sequence: ESC '[' parameter-bytes
// (0x30-0x3F) intermediate-bytes (0x20-0x2F) final-byte (0x40-0x7E). This
// covers SGR colors ("\x1b[7m", "\x1b[0m"), erase-in-line ("\x1b[K"), cursor
// movement, etc. — everything observed in real captures and then some.
var ansiCSI = regexp.MustCompile("\x1b\\[[0-?]*[ -/]*[@-~]")

// pagerPrompt matches a line that is nothing but a terminal pager prompt:
// "--More--", " --More-- ", "------ More ------", case-insensitive. A line
// that merely contains the word "more" as part of real content does not match
// (the dashes are required on both sides).
var pagerPrompt = regexp.MustCompile(`(?i)^\s*-{2,}\s*more\s*-{2,}\s*$`)

// Clean reads all of r and returns it as cleaned logical lines, with no
// trailing newline on any element. It:
//   - strips ANSI/VT100 CSI escape sequences,
//   - normalizes "\r\n" and bare "\r" to line breaks (this un-glues lines that
//     a "--More--" prompt was prefixed onto),
//   - drops lines that are only a pager prompt.
//
// Blank lines are preserved (parsers already treat them as "continue current
// block"). The single trailing empty element produced when the input ends with
// a newline is dropped, matching bufio.Scanner's behavior. The returned slice
// is always non-nil. Clean returns an error only if reading r fails.
func Clean(r io.Reader) ([]string, error) {
	raw, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return cleanText(string(raw)), nil
}

// cleanText is the pure core of Clean, operating on an in-memory string.
func cleanText(s string) []string {
	s = ansiCSI.ReplaceAllString(s, "")
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")

	rawLines := strings.Split(s, "\n")
	// strings.Split appends a trailing "" when s ends with "\n"; drop it so the
	// output matches what bufio.Scanner produced for the same input.
	if n := len(rawLines); n > 0 && rawLines[n-1] == "" {
		rawLines = rawLines[:n-1]
	}

	lines := make([]string, 0, len(rawLines))
	for _, ln := range rawLines {
		if pagerPrompt.MatchString(ln) {
			continue
		}
		lines = append(lines, ln)
	}
	return lines
}
