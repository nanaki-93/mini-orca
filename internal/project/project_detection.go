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

func detectProject(root string) projectDetection {
	switch {
	case projectFileExists(root, "go.mod"):
		return projectDetection{Type: "go", BuildFile: "go.mod"}
	case projectFileExists(root, "build.gradle.kts") || projectFileExists(root, "build.gradle"):
		return detectGradleProject(root)
	case projectFileExists(root, "Cargo.toml"):
		return projectDetection{Type: "rust", BuildFile: "Cargo.toml"}
	case projectFileExists(root, "package.json"):
		return projectDetection{Type: "typescript", BuildFile: "package.json"}
	case projectFileExists(root, "pyproject.toml"):
		return projectDetection{Type: "python", BuildFile: "pyproject.toml"}
	case projectFileExists(root, "requirements.txt"):
		return projectDetection{Type: "python", BuildFile: "requirements.txt"}
	default:
		return projectDetection{Type: "unknown"}
	}
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
