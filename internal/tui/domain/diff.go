package domain

import (
	"path/filepath"
	"strings"

	"github.com/nanaki-93/mini-orca/v2/internal/tui/client"
)

// DiffHunk represents a contiguous group of changed lines with surrounding context.
type DiffHunk struct {
	Index    int
	OldStart int
	OldEnd   int
	NewStart int
	NewEnd   int
	Lines    []client.DiffLine
}

// FileDiffHunk pairs a DiffHunk with its owning FileChange metadata across a multi-file proposal.
type FileDiffHunk struct {
	GlobalIndex   int
	FileIndex     int
	FileTotal     int
	Path          string
	Summary       string
	FileAdded     int
	FileRemoved   int
	FileHunkIndex int
	FileHunkTotal int
	Hunk          DiffHunk
}

// SplitDiffRow represents a paired side-by-side diff row (before on left, after on right).
type SplitDiffRow struct {
	OldLine int
	OldKind string
	OldText string
	NewLine int
	NewKind string
	NewText string
}

// UnifiedDiffRow represents a display row in the multi-file unified diff stream.
type UnifiedDiffRow struct {
	IsHeader      bool
	HunkGlobalIdx int
	FileIndex     int
	HeaderText    string
	Line          client.DiffLine
}

// IsAddedLine reports whether a diff line kind represents an insertion.
func IsAddedLine(kind string) bool {
	return kind == "add" || kind == "added" || kind == "+"
}

// IsRemovedLine reports whether a diff line kind represents a deletion.
func IsRemovedLine(kind string) bool {
	return kind == "delete" || kind == "del" || kind == "removed" || kind == "-"
}

// SplitDiffIntoHunks groups diff lines into hunks with up to 2 lines of surrounding context.
func SplitDiffIntoHunks(lines []client.DiffLine) []DiffHunk {
	changed := changedLineIndices(lines)
	if len(changed) == 0 {
		return nil
	}
	ranges := mergeHunkRanges(changed, len(lines))
	out := make([]DiffHunk, 0, len(ranges))
	for i, r := range ranges {
		slice := lines[r[0] : r[1]+1]
		oldStart, oldEnd := lineBounds(slice, true)
		newStart, newEnd := lineBounds(slice, false)
		out = append(out, DiffHunk{
			Index:    i,
			OldStart: oldStart,
			OldEnd:   oldEnd,
			NewStart: newStart,
			NewEnd:   newEnd,
			Lines:    slice,
		})
	}
	return out
}

func changedLineIndices(lines []client.DiffLine) []int {
	var indices []int
	for idx, line := range lines {
		if IsAddedLine(line.Kind) || IsRemovedLine(line.Kind) {
			indices = append(indices, idx)
		}
	}
	return indices
}

func mergeHunkRanges(changed []int, total int) [][2]int {
	var ranges [][2]int
	for _, idx := range changed {
		start := max(0, idx-2)
		end := min(total-1, idx+2)
		if len(ranges) == 0 || start > ranges[len(ranges)-1][1]+1 {
			ranges = append(ranges, [2]int{start, end})
			continue
		}
		if end > ranges[len(ranges)-1][1] {
			ranges[len(ranges)-1][1] = end
		}
	}
	return ranges
}

func lineBounds(slice []client.DiffLine, old bool) (int, int) {
	first, last := 0, 0
	for _, line := range slice {
		num := line.NewLine
		if old {
			num = line.OldLine
		}
		if num <= 0 {
			continue
		}
		if first == 0 {
			first = num
		}
		last = num
	}
	if first == 0 {
		return 1, 1
	}
	return first, last
}

// RejectHunkFromDiff reconstructs the proposed file content with the specified hunk reverted.
func RejectHunkFromDiff(allLines []client.DiffLine, hunk DiffHunk) string {
	hunkRange := findHunkSliceBounds(allLines, hunk)
	var kept []string
	for idx, line := range allLines {
		inHunk := idx >= hunkRange[0] && idx <= hunkRange[1]
		if inHunk {
			if !IsAddedLine(line.Kind) {
				kept = append(kept, line.Text)
			}
		} else if !IsRemovedLine(line.Kind) {
			kept = append(kept, line.Text)
		}
	}
	return strings.Join(kept, "\n")
}

// RejectHunkLines returns an updated diff line slice with the specified hunk reverted to context.
func RejectHunkLines(allLines []client.DiffLine, hunk DiffHunk) []client.DiffLine {
	hunkRange := findHunkSliceBounds(allLines, hunk)
	out := make([]client.DiffLine, 0, len(allLines))
	for idx, line := range allLines {
		if idx >= hunkRange[0] && idx <= hunkRange[1] {
			if IsAddedLine(line.Kind) {
				continue
			}
			if IsRemovedLine(line.Kind) {
				line.Kind = "context"
				line.NewLine = line.OldLine
			}
		}
		out = append(out, line)
	}
	return out
}

func findHunkSliceBounds(allLines []client.DiffLine, hunk DiffHunk) [2]int {
	if len(hunk.Lines) == 0 || len(allLines) == 0 {
		return [2]int{-1, -1}
	}
	target := hunk.Lines[0]
	for i := 0; i+len(hunk.Lines) <= len(allLines); i++ {
		if allLines[i] == target {
			return [2]int{i, i + len(hunk.Lines) - 1}
		}
	}
	return [2]int{-1, -1}
}

// ClassifyDiffScope returns added, removed, and the precision scope badge for a change session.
func ClassifyDiffScope(changes []client.FileChange) (added int, removed int, badge string) {
	for _, fc := range changes {
		for _, line := range fc.Diff.Lines {
			if IsAddedLine(line.Kind) {
				added++
			} else if IsRemovedLine(line.Kind) {
				removed++
			}
		}
	}
	delta := added + removed
	switch {
	case delta == 0:
		return added, removed, "IDLE"
	case delta <= 60:
		return added, removed, "SURGICAL"
	case delta <= 200:
		return added, removed, "FOCUSED"
	default:
		return added, removed, "BROAD"
	}
}

// EffectiveSessionChanges returns all modified files in the proposal that contain actual line additions or removals.
func EffectiveSessionChanges(change *client.ChangeSession) []client.FileChange {
	if change == nil || len(change.Changes) == 0 {
		return nil
	}
	out := make([]client.FileChange, 0, len(change.Changes))
	for _, fc := range change.Changes {
		if add, rem := countFileDiffLines(fc.Diff.Lines); add > 0 || rem > 0 {
			out = append(out, fc)
		}
	}
	return out
}

// CollectChangeHunks extracts all hunks across all touched files in a ChangeSession.
func CollectChangeHunks(changes []client.FileChange) []FileDiffHunk {
	if len(changes) == 0 {
		return nil
	}
	type touchedFile struct {
		fc      client.FileChange
		hunks   []DiffHunk
		added   int
		removed int
	}
	touched := make([]touchedFile, 0, len(changes))
	for _, fc := range changes {
		hunks := SplitDiffIntoHunks(fc.Diff.Lines)
		if len(hunks) == 0 {
			continue
		}
		fAdd, fRem := countFileDiffLines(fc.Diff.Lines)
		touched = append(touched, touchedFile{
			fc:      fc,
			hunks:   hunks,
			added:   fAdd,
			removed: fRem,
		})
	}
	var out []FileDiffHunk
	for fIdx, tf := range touched {
		for hIdx, h := range tf.hunks {
			out = append(out, FileDiffHunk{
				GlobalIndex:   len(out),
				FileIndex:     fIdx,
				FileTotal:     len(touched),
				Path:          tf.fc.Path,
				Summary:       tf.fc.Summary,
				FileAdded:     tf.added,
				FileRemoved:   tf.removed,
				FileHunkIndex: hIdx,
				FileHunkTotal: len(tf.hunks),
				Hunk:          h,
			})
		}
	}
	return out
}

func countFileDiffLines(lines []client.DiffLine) (added int, removed int) {
	for _, dl := range lines {
		if IsAddedLine(dl.Kind) {
			added++
		} else if IsRemovedLine(dl.Kind) {
			removed++
		}
	}
	return added, removed
}

// BuildSplitDiffRows pairs consecutive removed and added lines into side-by-side comparison rows.
func BuildSplitDiffRows(lines []client.DiffLine) []SplitDiffRow {
	var rows []SplitDiffRow
	for i := 0; i < len(lines); {
		dl := lines[i]
		if !IsRemovedLine(dl.Kind) && !IsAddedLine(dl.Kind) {
			rows = append(rows, SplitDiffRow{
				OldLine: dl.OldLine,
				OldKind: "context",
				OldText: dl.Text,
				NewLine: dl.NewLine,
				NewKind: "context",
				NewText: dl.Text,
			})
			i++
			continue
		}
		var dels, adds []client.DiffLine
		for i < len(lines) && (IsRemovedLine(lines[i].Kind) || IsAddedLine(lines[i].Kind)) {
			if IsRemovedLine(lines[i].Kind) {
				dels = append(dels, lines[i])
			} else {
				adds = append(adds, lines[i])
			}
			i++
		}
		rows = appendPairedChangeRows(rows, dels, adds)
	}
	return rows
}

func appendPairedChangeRows(rows []SplitDiffRow, dels, adds []client.DiffLine) []SplitDiffRow {
	count := max(len(dels), len(adds))
	for k := 0; k < count; k++ {
		var r SplitDiffRow
		if k < len(dels) {
			r.OldLine = dels[k].OldLine
			r.OldKind = "delete"
			r.OldText = dels[k].Text
		}
		if k < len(adds) {
			r.NewLine = adds[k].NewLine
			r.NewKind = "add"
			r.NewText = adds[k].Text
		}
		rows = append(rows, r)
	}
	return rows
}

func FirstChangedRowForMode(hunk DiffHunk, mode string) int {
	if mode == "split" {
		for idx, r := range BuildSplitDiffRows(hunk.Lines) {
			if r.OldKind == "delete" || r.NewKind == "add" {
				return idx
			}
		}
		return 0
	}
	for idx, dl := range hunk.Lines {
		if IsAddedLine(dl.Kind) || IsRemovedLine(dl.Kind) {
			return idx
		}
	}
	return 0
}

var testNameSuffixes = []string{
	"_test.go", "_test.py", "test.java", "tests.java",
	"test.kt", "test.rs", "test.rb", "test.php", "test.cs",
}

// IsTestFilePath reports whether a project-relative path matches the daemon's Stage 2 test-path rules.
func IsTestFilePath(path string) bool {
	clean := filepath.ToSlash(strings.TrimSpace(path))
	base := strings.ToLower(filepath.Base(clean))
	if strings.HasPrefix(base, "test_") || strings.Contains(base, ".test.") || strings.Contains(base, ".spec.") {
		return true
	}
	for _, suf := range testNameSuffixes {
		if strings.HasSuffix(base, suf) {
			return true
		}
	}
	return hasTestDirSegment(clean)
}

func hasTestDirSegment(clean string) bool {
	for _, seg := range strings.Split(strings.ToLower(clean), "/") {
		switch seg {
		case "test", "tests", "__tests__", "spec", "specs":
			return true
		}
	}
	return false
}

// DeriveCompanionTestPath returns a canonical companion test file path for a source path.
func DeriveCompanionTestPath(path string) string {
	clean := filepath.ToSlash(strings.TrimSpace(path))
	if clean == "" || IsTestFilePath(clean) {
		return clean
	}
	dir := filepath.ToSlash(filepath.Dir(clean))
	base := filepath.Base(clean)
	ext := filepath.Ext(base)
	stem := strings.TrimSuffix(base, ext)
	if stem == "" {
		stem = base
	}
	testBase := companionTestBase(stem, ext)
	if dir == "." || dir == "" {
		return testBase
	}
	return dir + "/" + testBase
}

func companionTestBase(stem, ext string) string {
	switch strings.ToLower(ext) {
	case ".go":
		return stem + "_test.go"
	case ".py":
		return "test_" + stem + ".py"
	case ".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs":
		return stem + ".test" + ext
	default:
		return stem + "_test.go"
	}
}

// EnsureChangeTestPaths deduplicates captured paths, caps scope at 8 files, and guarantees companion test paths.
func EnsureChangeTestPaths(paths []string, index *client.ProjectIndex) []string {
	var clean []string
	for _, raw := range paths {
		p := filepath.ToSlash(strings.TrimSpace(raw))
		if p != "" && !ContainsString(clean, p) && len(clean) < 8 {
			clean = append(clean, p)
		}
	}
	if len(clean) == 0 && index != nil && len(index.Files) > 0 {
		clean = append(clean, index.Files[0].Path)
	}
	if len(clean) == 0 {
		return clean
	}
	return appendCompanionTestPaths(clean)
}

func appendCompanionTestPaths(clean []string) []string {
	for _, src := range append([]string(nil), clean...) {
		if IsTestFilePath(src) || hasTestPathInDir(clean, filepath.ToSlash(filepath.Dir(src))) {
			continue
		}
		testPath := DeriveCompanionTestPath(src)
		if testPath == "" || ContainsString(clean, testPath) {
			continue
		}
		if len(clean) < 8 {
			clean = append(clean, testPath)
		} else if !hasAnyTestPath(clean) {
			clean[7] = testPath
		}
	}
	return clean
}

func hasTestPathInDir(paths []string, dir string) bool {
	for _, p := range paths {
		if IsTestFilePath(p) && filepath.ToSlash(filepath.Dir(p)) == dir {
			return true
		}
	}
	return false
}

func hasAnyTestPath(paths []string) bool {
	for _, p := range paths {
		if IsTestFilePath(p) {
			return true
		}
	}
	return false
}
