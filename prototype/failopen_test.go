package main

import (
	"bytes"
	"strings"
	"testing"
)

// gitleaks 只約定 0（乾淨）與 1（有命中）。其他退出碼代表掃描本身出錯，
// 這時 stdout 就算是合法 JSON 也不能當結果——否則掃描失敗會被讀成「找到 0 筆」，
// commit 靜默放行。實測 exit 2 + "[]" 會讓含假 AWS key 的 commit 通過。
func TestRunGitleaksRejectsUnexpectedExitCode(t *testing.T) {
	cases := []struct {
		name   string
		script string
	}{
		{
			name:   "exit 2 配空陣列",
			script: "printf '[]\\n'\necho 'simulated scanner failure' >&2\nexit 2\n",
		},
		{
			// Secret 的內容不影響這個測試的斷言，用佔位字串即可——真實格式的
			// 假金鑰會觸發 GitHub push protection，把新增這種值的 commit 擋在推送之外。
			name:   "exit 2 配 findings",
			script: "printf '[{\"RuleID\":\"aws-access-token\",\"File\":\"secret.txt\",\"Secret\":\"REDACTED-TEST-VALUE\"}]\\n'\necho 'simulated scanner failure' >&2\nexit 2\n",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			repo := useFakeGitleaksScript(t, c.script)
			var stderr bytes.Buffer
			findings, err := runGitleaks(repo, &stderr, "detect", "--report-format", "json", "--report-path", "-")
			if err == nil {
				t.Fatalf("掃描失敗卻回報成功，findings = %#v", findings)
			}
			if !strings.Contains(err.Error(), "simulated scanner failure") {
				t.Errorf("錯誤訊息未帶上 gitleaks stderr：%q", err)
			}
		})
	}
}

// 補充規則載入不了時必須中止掃描，不能降級成只用 gitleaks 預設規則：
// 那樣只有補充規則抓得到的金鑰（Groq、Supabase secret 等）會整批放行，
// 而輸出仍是「找到 0 筆」。
func TestRunGitleaksFailsClosedWhenRulesUnavailable(t *testing.T) {
	repo := useFakeGitleaksScript(t, "printf '[]\\n'\nexit 0\n")
	// 讓暫存目錄無法建立，模擬規則檔寫不出來。
	t.Setenv("TMPDIR", "/dev/null/nonexistent")

	var stderr bytes.Buffer
	findings, err := runGitleaks(repo, &stderr, "detect", "--report-format", "json", "--report-path", "-")
	if err == nil {
		t.Fatalf("規則載入失敗卻繼續掃描，findings = %#v", findings)
	}
	if !strings.Contains(err.Error(), "補充規則") {
		t.Errorf("錯誤訊息沒說明是補充規則載入失敗：%q", err)
	}
	if !strings.Contains(err.Error(), "未降級") {
		t.Errorf("錯誤訊息沒點出這是刻意不降級：%q", err)
	}
}
