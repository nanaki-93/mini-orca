package project

import (
	"os"
	"path/filepath"
	"strings"
)

type projectDetection struct {
	Type      string
	BuildFile string
}

var projectBuildMarkers = []projectDetection{
	{Type: "rust", BuildFile: "Cargo.toml"},
	{Type: "typescript", BuildFile: "package.json"},
	{Type: "typescript", BuildFile: "deno.json"},
	{Type: "python", BuildFile: "pyproject.toml"},
	{Type: "python", BuildFile: "requirements.txt"},
	{Type: "java", BuildFile: "pom.xml"},
	{Type: "cpp", BuildFile: "CMakeLists.txt"},
	{Type: "swift", BuildFile: "Package.swift"},
	{Type: "ruby", BuildFile: "Gemfile"},
	{Type: "php", BuildFile: "composer.json"},
	{Type: "elixir", BuildFile: "mix.exs"},
	{Type: "dart", BuildFile: "pubspec.yaml"},
	{Type: "zig", BuildFile: "build.zig"},
	{Type: "make", BuildFile: "Makefile"},
}

func detectProject(root string) projectDetection {
	if projectFileExists(root, "go.mod") {
		return projectDetection{Type: "go", BuildFile: "go.mod"}
	}
	if projectFileExists(root, "build.gradle.kts") || projectFileExists(root, "build.gradle") {
		return detectGradleProject(root)
	}
	for _, marker := range projectBuildMarkers {
		if projectFileExists(root, marker.BuildFile) {
			return marker
		}
	}
	return projectDetection{Type: "unknown"}
}

func detectGradleProject(root string) projectDetection {
	buildFile := "build.gradle"
	if projectFileExists(root, "build.gradle.kts") {
		buildFile = "build.gradle.kts"
	}
	if projectDirectoryExists(root, filepath.Join("src", "main", "kotlin")) || gradleBuildDeclaresKotlin(root, buildFile) {
		return projectDetection{Type: "kotlin", BuildFile: buildFile}
	}
	return projectDetection{Type: "java", BuildFile: buildFile}
}

func gradleBuildDeclaresKotlin(root, buildFile string) bool {
	data, err := os.ReadFile(filepath.Join(root, buildFile))
	return err == nil && (strings.Contains(string(data), "kotlin(") || strings.Contains(string(data), "org.jetbrains.kotlin"))
}

func projectFileExists(root, name string) bool {
	info, err := os.Stat(filepath.Join(root, name))
	return err == nil && !info.IsDir()
}

func projectDirectoryExists(root, name string) bool {
	info, err := os.Stat(filepath.Join(root, name))
	return err == nil && info.IsDir()
}
