package main

import (
	"path"
	"strings"
)

// 測試資產路徑的誤報濾除。
//
// 實測 golang/go（4215 commits）歷史掃描 257 筆全數落在測試資產，gin-gonic/gin
// 4 筆亦同：測試檔裡名為 key 的變數、RFC 範例字串、testdata/ 下的測試向量與
// 自簽私鑰。真洩漏 0 筆。這類命中不做處理會在 install 當下製造誤報疲勞。
//
// 不寫成 rules/leakbolt.toml 的 [[allowlists]]：那是全域套用，會連自家補充
// 規則（Groq、Supabase secret 等）在測試檔裡的命中一併關掉。測試檔裡出現真的
// 供應商金鑰仍是洩漏，必須照抓。
//
// 因此只濾掉 gitleaks 預設規則中高熵猜測型的那幾條，且僅限測試路徑。
var noisyTestRules = map[string]bool{
	"generic-api-key":     true,
	"square-access-token": true,
	"private-key":         true,
}

// isTestAssetPath 判斷路徑是否為測試資產。
func isTestAssetPath(file string) bool {
	if file == "" {
		return false
	}
	normalized := strings.ToLower(strings.ReplaceAll(file, "\\", "/"))
	base := path.Base(normalized)

	if strings.HasSuffix(base, "_test.go") ||
		strings.HasSuffix(base, ".test.js") || strings.HasSuffix(base, ".test.ts") ||
		strings.HasSuffix(base, ".spec.js") || strings.HasSuffix(base, ".spec.ts") ||
		strings.HasPrefix(base, "test_") {
		return true
	}

	for _, segment := range strings.Split(normalized, "/") {
		switch segment {
		case "testdata", "fixtures", "__fixtures__", "__tests__", "__mocks__", "test", "tests", "spec":
			return true
		}
	}
	return false
}

// filterTestAssetFindings 移除測試路徑上的高噪音命中，並回傳濾掉的筆數。
func filterTestAssetFindings(findings []Finding) ([]Finding, int) {
	kept := make([]Finding, 0, len(findings))
	filtered := 0
	for _, finding := range findings {
		if noisyTestRules[finding.RuleID] && isTestAssetPath(finding.File) {
			filtered++
			continue
		}
		kept = append(kept, finding)
	}
	return kept, filtered
}
