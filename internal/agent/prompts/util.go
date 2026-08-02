package prompts

// joinLines joins lines with double newlines for readability.
func joinLines(lines []string) string {
	result := ""
	for i, line := range lines {
		if i > 0 {
			result += "\n"
		}
		result += line
	}
	return result
}

// promptBuilder is a simple helper for building multi-line prompt strings.
type promptBuilder struct {
	lines []string
}

// AppendLine adds a line to the coder prompt builder.
func (b *promptBuilder) AppendLine(line string) {
	b.lines = append(b.lines, line)
}

// String returns the built prompt as a single string.
func (b *promptBuilder) String() string {
	return joinLines(b.lines)
}
