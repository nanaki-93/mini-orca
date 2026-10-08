package project

import (
	"path/filepath"
	"slices"
	"sort"
	"strings"
)

type InstructionPreset struct {
	ID       string   `json:"id"`
	Label    string   `json:"label"`
	Category string   `json:"category"`
	Content  string   `json:"content"`
	Reason   string   `json:"reason,omitempty"`
	Evidence []string `json:"evidence,omitempty"`
}

// SuggestInstructionPresets matches curated guidance to indexed facts in the
// target directory. Current context policy still applies if the index is older.
func SuggestInstructionPresets(root, target string, files []IndexFile) ([]InstructionPreset, error) {
	if _, err := ResolveWritePath(root, target); err != nil {
		return nil, err
	}
	policy, err := NewContextPolicy(root)
	if err != nil {
		return nil, err
	}
	scope := filepath.ToSlash(filepath.Dir(target))
	evidence := map[string][]string{}
	for _, file := range files {
		if file.Binary || !policy.Decide(file.Path).Include || (scope != "." && !strings.HasPrefix(file.Path, scope+"/")) {
			continue
		}
		for _, signal := range instructionSignals(file) {
			evidence[signal] = append(evidence[signal], file.Path)
		}
	}
	for signal, paths := range evidence {
		sort.Strings(paths)
		paths = slices.Compact(paths)
		if len(paths) > 3 {
			paths = paths[:3]
		}
		evidence[signal] = paths
	}
	presets := []InstructionPreset{}
	for _, rule := range projectInstructionRules() {
		if paths := evidence[rule.signal]; len(paths) > 0 {
			preset := rule.preset
			preset.Reason = instructionReasons[rule.signal]
			preset.Evidence = paths
			presets = append(presets, preset)
		}
	}
	presets = append(presets, generalInstructionPresets()...)
	categories := []string{instructionProject, instructionBuild, instructionCode, instructionTests, instructionSecurity, instructionUI, instructionDocs, instructionWorkflow}
	sort.SliceStable(presets, func(i, j int) bool {
		return slices.Index(categories, presets[i].Category) < slices.Index(categories, presets[j].Category)
	})
	return presets, nil
}

func instructionSignals(file IndexFile) []string {
	signal := file.Language
	switch signal {
	case "TypeScript":
		signal = "JavaScript"
	case "Kotlin", "Java":
		signal = "JVM"
	case "HTML", "CSS", "SCSS":
		signal = "UI"
	}
	signals := []string{signal}
	if file.Language == "TypeScript" {
		signals = append(signals, "TypeScript")
	}
	if ext := strings.ToLower(filepath.Ext(file.Path)); ext == ".tsx" || ext == ".jsx" {
		signals = append(signals, "UI")
	}
	signals = append(signals, instructionBuildSignals(file.Path)...)
	signals = append(signals, instructionImportSignals(file)...)
	return signals
}

var instructionReasons = map[string]string{
	"Go":         "Go source or module manifests in this scope",
	"JavaScript": "JavaScript, TypeScript or Node package manifests in this scope",
	"TypeScript": "TypeScript source or configuration in this scope",
	"UI":         "UI source in this scope", "React": "React imports in this scope",
	"Python": "Python source or dependency manifests in this scope",
	"JVM":    "Kotlin, Java or JVM build manifests in this scope",
	"Gradle": "Gradle build files in this scope", "Rust": "Rust source or Cargo manifests in this scope",
	"Shell": "Shell scripts in this scope", "Markdown": "Markdown files in this scope",
	"HTTP": "Go net/http imports in this scope", "SQL": "SQL files or Go database/sql imports in this scope",
	"CLI":       "Go command-line argument library imports in this scope",
	"Desktop":   "Desktop host manifests or Wails imports in this scope",
	"Container": "Dockerfiles in this scope", "CI": "GitHub Actions workflows in this scope",
}

var instructionBuildFiles = map[string][]string{
	"go.mod": {"Go"}, "go.work": {"Go"}, "package.json": {"JavaScript"},
	"tsconfig.json": {"JavaScript", "TypeScript"}, "pyproject.toml": {"Python"},
	"requirements.txt": {"Python"}, "Cargo.toml": {"Rust"},
	"build.gradle": {"JVM", "Gradle"}, "build.gradle.kts": {"JVM", "Gradle"},
	"pom.xml": {"JVM"}, "wails.json": {"Desktop"}, "tauri.conf.json": {"Desktop"},
	"Dockerfile": {"Container"},
}

func instructionBuildSignals(path string) []string {
	signals := append([]string(nil), instructionBuildFiles[filepath.Base(path)]...)
	if strings.Contains("/"+path, "/.github/workflows/") && (strings.HasSuffix(path, ".yml") || strings.HasSuffix(path, ".yaml")) {
		signals = append(signals, "CI")
	}
	return signals
}

var instructionGoImports = map[string]string{
	"net/http": "HTTP", "database/sql": "SQL", "flag": "CLI",
	"github.com/spf13/cobra": "CLI", "github.com/urfave/cli/v2": "CLI",
	"github.com/wailsapp/wails/v2": "Desktop",
}

func instructionImportSignals(file IndexFile) []string {
	var signals []string
	for _, name := range file.Imports {
		if file.Language == "Go" && instructionGoImports[name] != "" {
			signals = append(signals, instructionGoImports[name])
		}
		if file.Language == "TypeScript" && (strings.Contains(name, "'react'") || strings.Contains(name, `"react"`)) {
			signals = append(signals, "React", "UI")
		}
	}
	return signals
}
