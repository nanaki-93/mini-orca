package project

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const contextPolicyVersion = "1"

// ContextDecision explains whether a project file may enter an LLM prompt.
type ContextDecision struct {
	Path    string `json:"path"`
	Include bool   `json:"include"`
	Reason  string `json:"reason"`
}

// ContextPolicy is the single inclusion policy for imports and model prompts.
type ContextPolicy struct {
	root    string
	ignores []ignoreRule
	include []ignoreRule
	exclude []ignoreRule
}

type ignoreRule struct {
	pattern string
	regexp  *regexp.Regexp
}

type policyConfig struct {
	Include []string `json:"include"`
	Exclude []string `json:"exclude"`
}

func NewContextPolicy(root string) (*ContextPolicy, error) {
	canonical, err := CanonicalRoot(root)
	if err != nil {
		return nil, err
	}
	p := &ContextPolicy{root: canonical}
	for _, name := range []string{".gitignore", ".mini-orcaignore"} {
		rules, err := readIgnoreRules(filepath.Join(canonical, name))
		if err != nil {
			return nil, err
		}
		p.ignores = append(p.ignores, rules...)
	}
	configPath := filepath.Join(canonical, ".mini-orca", "context-policy.json")
	data, err := os.ReadFile(configPath)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("read context policy: %w", err)
	}
	if err == nil {
		var cfg policyConfig
		if err := json.Unmarshal(data, &cfg); err != nil {
			return nil, fmt.Errorf("parse context policy: %w", err)
		}
		p.include, err = compileRules(cfg.Include)
		if err != nil {
			return nil, err
		}
		p.exclude, err = compileRules(cfg.Exclude)
		if err != nil {
			return nil, err
		}
	}
	return p, nil
}

func (p *ContextPolicy) Version() string { return contextPolicyVersion }

func (p *ContextPolicy) Decide(relative string) ContextDecision {
	path := filepath.ToSlash(filepath.Clean(relative))
	if path == "." || strings.HasPrefix(path, "../") || filepath.IsAbs(relative) {
		return ContextDecision{Path: path, Reason: "unsafe path"}
	}
	if info, err := os.Lstat(filepath.Join(p.root, filepath.FromSlash(path))); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return ContextDecision{Path: path, Reason: "symlink is excluded from model context"}
	}
	name := strings.ToLower(filepath.Base(path))
	if secretFile(name) {
		return ContextDecision{Path: path, Reason: "secret or local configuration"}
	}
	if generatedOrLockFile(name) {
		return ContextDecision{Path: path, Reason: "generated, dependency, or lock file"}
	}
	if matches(p.exclude, path) {
		return ContextDecision{Path: path, Reason: "project context-policy exclude"}
	}
	if matches(p.include, path) {
		return ContextDecision{Path: path, Include: true, Reason: "project context-policy include"}
	}
	if matches(p.ignores, path) {
		return ContextDecision{Path: path, Reason: "ignored by .gitignore or .mini-orcaignore"}
	}
	return ContextDecision{Path: path, Include: true, Reason: "eligible source or text file"}
}

func secretFile(name string) bool {
	if name == ".env" || strings.HasPrefix(name, ".env.") {
		return true
	}
	if name == "config.yaml" || name == "config.yml" || strings.Contains(name, ".local.") {
		return true
	}
	for _, fragment := range []string{"credential", "secret", "token", "apikey", "api_key", "password"} {
		if strings.Contains(name, fragment) {
			return true
		}
	}
	for _, extension := range []string{".pem", ".key", ".crt", ".cer", ".der", ".p12", ".pfx"} {
		if strings.HasSuffix(name, extension) {
			return true
		}
	}
	return false
}

func generatedOrLockFile(name string) bool {
	if strings.Contains(name, ".min.") || strings.Contains(name, ".generated.") || strings.HasSuffix(name, "_generated.go") {
		return true
	}
	switch name {
	case "go.sum", "cargo.lock", "package-lock.json", "yarn.lock", "pnpm-lock.yaml", "composer.lock", "gemfile.lock":
		return true
	}
	return false
}

func readIgnoreRules(path string) ([]ignoreRule, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read ignore file: %w", err)
	}
	var patterns []string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") && !strings.HasPrefix(line, "!") {
			patterns = append(patterns, line)
		}
	}
	return compileRules(patterns)
}

func compileRules(patterns []string) ([]ignoreRule, error) {
	rules := make([]ignoreRule, 0, len(patterns))
	for _, pattern := range patterns {
		pattern = strings.TrimPrefix(filepath.ToSlash(strings.TrimSpace(pattern)), "/")
		if pattern == "" {
			continue
		}
		if strings.HasSuffix(pattern, "/") {
			pattern += "**"
		}
		expr := regexp.QuoteMeta(pattern)
		expr = strings.ReplaceAll(expr, "\\*\\*", ".*")
		expr = strings.ReplaceAll(expr, "\\*", "[^/]*")
		expr = strings.ReplaceAll(expr, "\\?", "[^/]")
		re, err := regexp.Compile("^(?:.*/)?" + expr + "$")
		if err != nil {
			return nil, fmt.Errorf("invalid context-policy pattern %q: %w", pattern, err)
		}
		rules = append(rules, ignoreRule{pattern: pattern, regexp: re})
	}
	return rules, nil
}

func matches(rules []ignoreRule, path string) bool {
	for _, rule := range rules {
		if rule.regexp.MatchString(path) {
			return true
		}
	}
	return false
}
