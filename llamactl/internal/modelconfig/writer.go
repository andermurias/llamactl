package modelconfig

import "strings"

// WriteCmd applies updates (flag→value) to a multiline cmd string.
// Existing flags are updated in-place (preserving line order and all other flags).
// Flags not yet present are appended at the end.
func WriteCmd(cmd string, updates map[string]string) string {
	lines := strings.Split(cmd, "\n")
	applied := make(map[string]bool)

	for i := 0; i < len(lines); i++ {
		tok := strings.TrimSpace(lines[i])
		if !strings.HasPrefix(tok, "-") {
			continue
		}

		// Inline flag: "-ngl 99"
		if parts := strings.SplitN(tok, " ", 2); len(parts) == 2 {
			flag := parts[0]
			if val, ok := updates[flag]; ok {
				lines[i] = lineIndent(lines[i]) + flag + " " + val
				applied[flag] = true
			}
			continue
		}

		// Flag-only line: check if next line is a value
		flag := tok
		if i+1 < len(lines) && !strings.HasPrefix(strings.TrimSpace(lines[i+1]), "-") {
			if val, ok := updates[flag]; ok {
				lines[i+1] = lineIndent(lines[i+1]) + val
				applied[flag] = true
			}
			i++ // skip value line
		}
		// Boolean flag — nothing to update
	}

	// Append flags not yet present in the cmd
	for flag, val := range updates {
		if !applied[flag] {
			lines = append(lines, flag+" "+val)
		}
	}

	return strings.Join(lines, "\n")
}

// lineIndent returns the leading whitespace of a line.
func lineIndent(line string) string {
	return line[:len(line)-len(strings.TrimLeft(line, " \t"))]
}
