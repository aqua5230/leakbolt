package main

import (
	"bufio"
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestProviderForRuleIDKnownAndUnknown(t *testing.T) {
	provider, ok := providerForRuleID("leakbolt-groq-api-key")
	if !ok || provider.Name != "Groq" {
		t.Fatalf("provider = %+v, ok = %v, want Groq", provider, ok)
	}
	if _, ok := providerForRuleID("does-not-exist"); ok {
		t.Fatal("未知規則不應該有對應的處理指引")
	}
}

func TestConfirmYesAcceptsYAndYes(t *testing.T) {
	cases := map[string]bool{"y\n": true, "yes\n": true, "Y\n": true, "n\n": false, "\n": false}
	for input, want := range cases {
		scanner := bufio.NewScanner(strings.NewReader(input))
		if got := confirmYes(scanner); got != want {
			t.Fatalf("confirmYes(%q) = %v, want %v", input, got, want)
		}
	}
}

func TestGuideFingerprintRejectsShortPrefix(t *testing.T) {
	repo := newNotifyTestRepo(t)
	var stdout, stderr bytes.Buffer
	if code := guideFingerprint(repo, "short", &stdout, &stderr); code != 2 {
		t.Fatalf("exit = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "指紋前綴至少需要 8 個字元") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestGuideFingerprintUnknownRuleIDPrintsGenericGuidance(t *testing.T) {
	t.Setenv("LEAKBOLT_NO_GUI", "1")
	repo := newNotifyTestRepo(t)
	seedGuideFinding(t, repo, "generic-api-key", "some-secret")
	prefix := recentFingerprintPrefix(t, repo)

	var stdout, stderr bytes.Buffer
	if code := guideFingerprint(repo, prefix, &stdout, &stderr); code != 0 {
		t.Fatalf("exit = %d, want 0：%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "沒有內建的處理指引") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestGuideFingerprintDeclineSkipsBrowserButStillRescans(t *testing.T) {
	t.Setenv("LEAKBOLT_NO_GUI", "1")
	repo := newNotifyTestRepo(t)
	useFakeGitleaksScript(t, "printf '[]\\n'\nexit 0\n")
	seedGuideFinding(t, repo, "leakbolt-groq-api-key", "still-here-secret")
	prefix := recentFingerprintPrefix(t, repo)

	stubGuidePrompt(t, "n\n\n")
	called := false
	stubOpenBrowser(t, func(string) error { called = true; return nil })

	var stdout, stderr bytes.Buffer
	if code := guideFingerprint(repo, prefix, &stdout, &stderr); code != 0 {
		t.Fatalf("exit = %d, want 0：%s", code, stderr.String())
	}
	if called {
		t.Fatal("使用者拒絕時不應開啟瀏覽器")
	}
	if !strings.Contains(stdout.String(), "沒辦法確認金鑰本身有沒有真的被撤銷") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestGuideFingerprintConfirmOpensBrowserWithRevokeURL(t *testing.T) {
	t.Setenv("LEAKBOLT_NO_GUI", "1")
	repo := newNotifyTestRepo(t)
	useFakeGitleaksScript(t, "printf '[]\\n'\nexit 0\n")
	seedGuideFinding(t, repo, "leakbolt-groq-api-key", "still-here-secret")
	prefix := recentFingerprintPrefix(t, repo)

	stubGuidePrompt(t, "y\n\n")
	var openedURL string
	stubOpenBrowser(t, func(url string) error { openedURL = url; return nil })

	var stdout, stderr bytes.Buffer
	if code := guideFingerprint(repo, prefix, &stdout, &stderr); code != 0 {
		t.Fatalf("exit = %d, want 0：%s", code, stderr.String())
	}
	want := providersByRuleID["leakbolt-groq-api-key"].RevokeURL
	if openedURL != want {
		t.Fatalf("開啟的網址 = %q, want %q", openedURL, want)
	}
}

func TestGuideFingerprintOpenBrowserFailureFallsBackToPrintingURL(t *testing.T) {
	t.Setenv("LEAKBOLT_NO_GUI", "1")
	repo := newNotifyTestRepo(t)
	useFakeGitleaksScript(t, "printf '[]\\n'\nexit 0\n")
	seedGuideFinding(t, repo, "leakbolt-groq-api-key", "still-here-secret")
	prefix := recentFingerprintPrefix(t, repo)

	stubGuidePrompt(t, "y\n\n")
	stubOpenBrowser(t, func(string) error { return errors.New("開不了") })

	var stdout, stderr bytes.Buffer
	if code := guideFingerprint(repo, prefix, &stdout, &stderr); code != 0 {
		t.Fatalf("exit = %d, want 0：%s", code, stderr.String())
	}
	want := providersByRuleID["leakbolt-groq-api-key"].RevokeURL
	if !strings.Contains(stderr.String(), want) {
		t.Fatalf("開瀏覽器失敗時應在 stderr 印出網址讓使用者自行前往，stderr = %q", stderr.String())
	}
}

func TestGuideFingerprintStillPresentAfterRescanReportsBlocked(t *testing.T) {
	t.Setenv("LEAKBOLT_NO_GUI", "1")
	repo := newNotifyTestRepo(t)
	useFakeGitleaksScript(t, `printf '[{"RuleID":"leakbolt-groq-api-key","File":"still.env","Secret":"still-here-secret"}]\n'`+"\nexit 1\n")
	seedGuideFinding(t, repo, "leakbolt-groq-api-key", "still-here-secret")
	prefix := recentFingerprintPrefix(t, repo)

	stubGuidePrompt(t, "n\n\n")
	stubOpenBrowser(t, func(string) error { return nil })

	var stdout, stderr bytes.Buffer
	if code := guideFingerprint(repo, prefix, &stdout, &stderr); code != 1 {
		t.Fatalf("命中還在時 exit = %d, want 1：%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "還在暫存區裡") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func seedGuideFinding(t *testing.T, repo, ruleID, secret string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	findings := []Finding{{RuleID: ruleID, File: "seed.env", Secret: secret}}
	if code := finishStagedScan(repo, findings, &stdout, &stderr); code != 1 {
		t.Fatalf("種子掃描 exit = %d, want 1：stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}

func recentFingerprintPrefix(t *testing.T, repo string) string {
	t.Helper()
	state, err := loadState(repo)
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Recent) == 0 {
		t.Fatal("state.Recent 是空的，種子掃描沒有寫入任何命中")
	}
	return state.Recent[0].Fingerprint[:8]
}

func stubGuidePrompt(t *testing.T, input string) {
	t.Helper()
	old := guideStdin
	guideStdin = strings.NewReader(input)
	t.Cleanup(func() { guideStdin = old })
}

func stubOpenBrowser(t *testing.T, fn func(string) error) {
	t.Helper()
	old := openBrowser
	openBrowser = fn
	t.Cleanup(func() { openBrowser = old })
}
