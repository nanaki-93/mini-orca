package project

import (
	"os/exec"
	"path/filepath"
	"strings"
)

// ImpactReference is advisory index metadata; it never adds source to a model request.
type ImpactReference struct {
	Path       string `json:"path"`
	Symbol     string `json:"symbol,omitempty"`
	Confidence string `json:"confidence"`
	Reason     string `json:"reason"`
}

type ImpactPreview struct {
	TargetPath   string            `json:"target_path"`
	TargetSymbol string            `json:"target_symbol,omitempty"`
	References   []ImpactReference `json:"references"`
}

// BuildImpactPreview uses only deterministic imports and declaration names.
func BuildImpactPreview(index *ProjectIndex, targetPath, targetSymbol string) ImpactPreview {
	preview := ImpactPreview{TargetPath: targetPath, TargetSymbol: targetSymbol, References: []ImpactReference{}}
	if index == nil {
		return preview
	}
	targetDir := filepath.ToSlash(filepath.Dir(targetPath))
	for _, file := range index.Files {
		if file.Path == targetPath {
			continue
		}
		for _, imported := range file.Imports {
			if targetDir != "." && strings.HasSuffix(strings.TrimSuffix(imported, "/"), targetDir) {
				preview.References = append(preview.References, ImpactReference{Path: file.Path, Confidence: "medium", Reason: "imports the target package"})
				break
			}
		}
	}
	return preview
}

type GitStatus struct {
	Available bool   `json:"available"`
	Branch    string `json:"branch,omitempty"`
	FileState string `json:"file_state,omitempty"`
	DiffState string `json:"diff_state,omitempty"`
}

// ReadGitStatus runs fixed read-only Git commands only.
func ReadGitStatus(root, targetPath string) GitStatus {
	inside, err := runGit(root, "rev-parse", "--is-inside-work-tree")
	if err != nil || strings.TrimSpace(inside) != "true" {
		return GitStatus{}
	}
	branch, _ := runGit(root, "branch", "--show-current")
	status, _ := runGit(root, "status", "--porcelain", "--", targetPath)
	diff, _ := runGit(root, "diff", "--name-only", "--", targetPath)
	diffState := "clean"
	if strings.TrimSpace(diff) != "" {
		diffState = "modified"
	}
	return GitStatus{Available: true, Branch: strings.TrimSpace(branch), FileState: strings.TrimSpace(status), DiffState: diffState}
}

func runGit(root string, args ...string) (string, error) {
	command := exec.Command("git", append([]string{"-C", root}, args...)...)
	output, err := command.Output()
	return string(output), err
}
