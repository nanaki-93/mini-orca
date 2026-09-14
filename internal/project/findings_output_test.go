package project

import "testing"

func TestReviewSectionsAllowEmptyButRejectMalformedFindings(t *testing.T) {
	for _, output := range []string{"", " \n\t", "[]", "[ \n ]", `{"findings":[]}`} {
		t.Run("empty:"+output, func(t *testing.T) {
			performance, warning, err := ParsePerformanceFindings(output, "main.go", securitySource(), nil)
			if err != nil || performance == nil || len(performance) != 0 || warning != "" {
				t.Fatalf("performance=%+v warning=%q err=%v", performance, warning, err)
			}
			security, err := ParseSecurityFindings(output, securityIndexedFile(), securitySource())
			if err != nil || security == nil || len(security) != 0 {
				t.Fatalf("security=%+v err=%v", security, err)
			}
		})
	}
	for _, output := range []string{`{}`, `null`, `{"findings":null}`, `{"findings":[{}]}`, `[{}]`, `[] {}`, `{"findings":`, "No issues found"} {
		t.Run("invalid:"+output, func(t *testing.T) {
			if _, _, err := ParsePerformanceFindings(output, "main.go", securitySource(), nil); err == nil {
				t.Fatal("invalid performance response passed")
			}
			if _, err := ParseSecurityFindings(output, securityIndexedFile(), securitySource()); err == nil {
				t.Fatal("invalid security response passed")
			}
		})
	}
	indexed := securityIndexedFile()
	indexed.ContentHash = "wrong"
	if _, err := ParseSecurityFindings("", indexed, securitySource()); err == nil {
		t.Fatal("empty review bypassed source identity")
	}
}
