package archgate

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

type hygieneSecretPattern struct {
	label   string
	pattern *regexp.Regexp
}

var hygieneSecretPatterns = []hygieneSecretPattern{
	{
		label:   "PEM private key",
		pattern: regexp.MustCompile(`-----BEGIN (?:RSA |EC |DSA |OPENSSH |PGP )?PRIVATE KEY-----`),
	},
	{
		label:   "AWS access key ID",
		pattern: regexp.MustCompile(`\bAKIA[0-9A-Z]{16}\b`),
	},
	{
		label:   "GitHub personal access token",
		pattern: regexp.MustCompile(`\b(?:ghp|gho|ghu|ghs|ghr)_[A-Za-z0-9]{36,}\b|\bgithub_pat_[A-Za-z0-9_]{40,}\b`),
	},
	{
		label:   "API secret key token",
		pattern: regexp.MustCompile(`\bsk-(?:proj-|ant-)?[A-Za-z0-9_-]{24,}\b|\bxox[baprs]-[A-Za-z0-9-]{16,}\b`),
	},
}

// CheckChangeSecretAndHygiene scans proposed edits for conflict markers and hardcoded secrets.
func CheckChangeSecretAndHygiene(edits []ChangeEdit) CheckResult {
	var violations []string
	for _, edit := range edits {
		violations = append(violations, scanEditHygieneViolations(edit)...)
	}
	if len(violations) > 0 {
		return CheckResult{
			Name:     "secret & hygiene gate",
			Required: true,
			State:    CheckFailed,
			Output:   strings.Join(violations, "\n"),
		}
	}
	return CheckResult{
		Name:     "secret & hygiene gate",
		Required: true,
		State:    CheckPassed,
		Output:   "No hardcoded secrets, private keys, or conflict markers detected.",
	}
}

func scanEditHygieneViolations(edit ChangeEdit) []string {
	slashPath := filepath.ToSlash(edit.Path)
	lines := strings.Split(edit.Content, "\n")
	var violations []string
	for idx, raw := range lines {
		lineNum := idx + 1
		if marker, ok := detectConflictMarker(raw); ok {
			violations = append(violations, fmt.Sprintf("%s:%d: unresolved Git merge conflict marker (%s)", slashPath, lineNum, marker))
		}
		for _, rule := range hygieneSecretPatterns {
			if rule.pattern.MatchString(raw) {
				violations = append(violations, fmt.Sprintf("%s:%d: potential leaked %s; remove hardcoded credentials before committing", slashPath, lineNum, rule.label))
			}
		}
	}
	return violations
}

func detectConflictMarker(line string) (string, bool) {
	trimmed := strings.TrimRight(line, "\r")
	switch {
	case strings.HasPrefix(trimmed, "<<<<<<< "):
		return "<<<<<<<", true
	case trimmed == "=======":
		return "=======", true
	case strings.HasPrefix(trimmed, ">>>>>>> "):
		return ">>>>>>>", true
	default:
		return "", false
	}
}

// HasOptionalGitleaks reports whether workspace has a .gitleaks.toml and gitleaks is installed.
func HasOptionalGitleaks(workspace string) bool {
	info, err := os.Stat(filepath.Join(workspace, ".gitleaks.toml"))
	if err != nil || info.IsDir() {
		return false
	}
	_, err = exec.LookPath("gitleaks")
	return err == nil
}
