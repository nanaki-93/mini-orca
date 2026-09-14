package project

// Label a range only when one innermost declaration contains all of it. A range
// spanning declarations or with equally narrow, conflicting labels stays at
// file/line scope. Names never come from model guesses on the current prompt.
func performanceSymbolForRange(symbols []SymbolInfo, start, end int) string {
	name := ""
	width := -1
	for _, symbol := range symbols {
		if symbol.Name == "" || symbol.StartLine > start || symbol.EndLine < end {
			continue
		}
		span := symbol.EndLine - symbol.StartLine
		switch {
		case width < 0 || span < width:
			name, width = symbol.Name, span
		case span == width && symbol.Name != name:
			name = ""
		}
	}
	return name
}
