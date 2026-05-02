package modelconfig

import "strings"

// ParseCmd extracts flag→value pairs from a multiline cmd string.
// Lines starting with "-" are treated as flags. The following line is the
// value if it does not itself start with "-". Boolean flags (no value) get "".
//
// Handles both inline form ("-ngl 99") and next-line form ("-ngl\n99").
func ParseCmd(cmd string) map[string]string {
	lines := splitCmdLines(cmd)
	result := make(map[string]string)
	for i := 0; i < len(lines); i++ {
		tok := lines[i]
		if !strings.HasPrefix(tok, "-") {
			continue
		}
		// Inline value: "-ngl 99" on one line
		if parts := strings.SplitN(tok, " ", 2); len(parts) == 2 {
			result[parts[0]] = strings.TrimSpace(parts[1])
			continue
		}
		// Next-line value
		if i+1 < len(lines) && !strings.HasPrefix(lines[i+1], "-") {
			result[tok] = lines[i+1]
			i++ // consumed
		} else {
			result[tok] = "" // boolean flag
		}
	}
	return result
}

// splitCmdLines trims whitespace and returns non-empty tokens.
func splitCmdLines(cmd string) []string {
	raw := strings.Split(cmd, "\n")
	out := make([]string, 0, len(raw))
	for _, l := range raw {
		if t := strings.TrimSpace(l); t != "" {
			out = append(out, t)
		}
	}
	return out
}
