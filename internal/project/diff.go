package project

import "strings"

type diffMatch struct{ old, new int }

// BuildUnifiedDiff aligns unchanged lines between edits and preserves both source line numbers.
func BuildUnifiedDiff(path, original, composed string) UnifiedDiff {
	before, after := strings.Split(original, "\n"), strings.Split(composed, "\n")
	if original == "" {
		before = nil
	}
	if composed == "" {
		after = nil
	}
	matches := appendDiffMatches(nil, before, after, 0, 0)
	matches = append(matches, diffMatch{len(before), len(after)})
	lines := make([]DiffLine, 0, len(before)+len(after))
	oldLine, newLine := 0, 0
	for _, match := range matches {
		for oldLine < match.old {
			lines = append(lines, DiffLine{Kind: "removed", OldLine: oldLine + 1, Text: before[oldLine]})
			oldLine++
		}
		for newLine < match.new {
			lines = append(lines, DiffLine{Kind: "added", NewLine: newLine + 1, Text: after[newLine]})
			newLine++
		}
		if oldLine < len(before) && newLine < len(after) {
			lines = append(lines, DiffLine{Kind: "context", OldLine: oldLine + 1, NewLine: newLine + 1, Text: before[oldLine]})
			oldLine++
			newLine++
		}
	}
	return UnifiedDiff{OldPath: path, NewPath: path, Lines: lines}
}

// Hirschberg's LCS keeps memory linear for full-file proposals. Common edges
// avoid recomputing the unchanged start and end of each partition.
func appendDiffMatches(matches []diffMatch, before, after []string, old, next int) []diffMatch {
	for len(before) > 0 && len(after) > 0 && before[0] == after[0] {
		matches = append(matches, diffMatch{old, next})
		before, after, old, next = before[1:], after[1:], old+1, next+1
	}
	suffix := 0
	for suffix < len(before) && suffix < len(after) && before[len(before)-1-suffix] == after[len(after)-1-suffix] {
		suffix++
	}
	a, b := before[:len(before)-suffix], after[:len(after)-suffix]
	if len(a) == 1 {
		for j, line := range b {
			if a[0] == line {
				matches = append(matches, diffMatch{old, next + j})
				break
			}
		}
	} else if len(a) > 1 && len(b) > 0 {
		middle := len(a) / 2
		split := diffSplit(a, b, middle)
		matches = appendDiffMatches(matches, a[:middle], b[:split], old, next)
		matches = appendDiffMatches(matches, a[middle:], b[split:], old+middle, next+split)
	}
	for i := 0; i < suffix; i++ {
		matches = append(matches, diffMatch{old + len(a) + i, next + len(b) + i})
	}
	return matches
}

func diffSplit(before, after []string, middle int) int {
	left, right := diffLengths(before[:middle], after, false), diffLengths(before[middle:], after, true)
	split, best := 0, -1
	for j := range left {
		if length := left[j] + right[len(after)-j]; length > best {
			split, best = j, length
		}
	}
	return split
}

func diffLengths(before, after []string, reverse bool) []int {
	lengths := make([]int, len(after)+1)
	for i := range before {
		diagonal := 0
		for j := range after {
			a, b := i, j
			if reverse {
				a, b = len(before)-1-i, len(after)-1-j
			}
			previous := lengths[j+1]
			if before[a] == after[b] {
				lengths[j+1] = diagonal + 1
			} else {
				lengths[j+1] = max(lengths[j], lengths[j+1])
			}
			diagonal = previous
		}
	}
	return lengths
}
