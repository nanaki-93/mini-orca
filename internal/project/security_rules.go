package project

import "github.com/nanaki-93/mini-orca/v2/internal/project/securityrules"

const SecurityGoRuleSetVersion = securityrules.SecurityGoRuleSetVersion

type SecurityRuleScanResult = securityrules.SecurityRuleScanResult

// ScanGoSecurityRules applies the intentionally small, source-only security
// ruleset to one indexed Go file. It does not resolve packages or execute code.
func ScanGoSecurityRules(indexed IndexFile, source string) (SecurityRuleScanResult, error) {
	if !validSecurityIndex(indexed) || indexed.Language != "Go" || indexed.Binary {
		return SecurityRuleScanResult{}, ErrSecurityRulesUnavailable
	}
	if len(source) > SecurityMaxSourceBytes || contentHash([]byte(source)) != indexed.ContentHash {
		return SecurityRuleScanResult{}, ErrRevisionConflict
	}
	return securityrules.ScanGoFile(indexed, source)
}
