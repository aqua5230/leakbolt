package main

import "testing"

func TestIsTestAssetPath(t *testing.T) {
	cases := []struct {
		file string
		want bool
	}{
		{"src/crypto/internal/hpke/testdata/rfc9180-vectors.json", true},
		{"src/crypto/x509/pkcs8_test.go", true},
		{"testdata/certificate/key.pem", true},
		{"src\\debug\\buildinfo\\testdata\\go117\\go117.base64", true},
		{"app/__tests__/auth.js", true},
		{"lib/auth.spec.ts", true},
		{"tests/conftest.py", true},
		{"api/test_client.py", true},
		{"TestData/keys.json", true},

		{"src/config.go", false},
		{"lib/latest/config.json", false},
		{"internal/protest/run.go", false},
		{"contests/entry.env", false},
		{".env", false},
		{"", false},
	}
	for _, c := range cases {
		if got := isTestAssetPath(c.file); got != c.want {
			t.Errorf("isTestAssetPath(%q) = %v, want %v", c.file, got, c.want)
		}
	}
}

func TestFilterTestAssetFindings(t *testing.T) {
	findings := []Finding{
		{RuleID: "generic-api-key", File: "context_test.go"},
		{RuleID: "square-access-token", File: "src/debug/buildinfo/testdata/go117/go117.base64"},
		{RuleID: "private-key", File: "testdata/certificate/key.pem"},
		// 測試路徑，但屬於自家補充規則：真的供應商金鑰，必須留下
		{RuleID: "leakbolt-groq-api-key", File: "api/client_test.go"},
		// 高噪音規則，但不在測試路徑：必須留下
		{RuleID: "generic-api-key", File: "src/config.go"},
	}
	kept, filtered := filterTestAssetFindings(findings)
	if filtered != 3 {
		t.Fatalf("filtered = %d, want 3", filtered)
	}
	if len(kept) != 2 {
		t.Fatalf("kept = %d, want 2", len(kept))
	}
	if kept[0].RuleID != "leakbolt-groq-api-key" || kept[1].File != "src/config.go" {
		t.Errorf("留下的不是預期的兩筆：%+v", kept)
	}
}
