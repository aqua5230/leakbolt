package main

import (
	"bytes"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

func TestNotifyBlockedCommitSkipsCI(t *testing.T) {
	called := false
	stubNativeDialog(t, func([]stagedFinding) (string, error) {
		called = true
		return acknowledgeDialogButton, nil
	})
	t.Setenv("CI", "true")

	notifyBlockedCommit(t.TempDir(), []stagedFinding{{Fingerprint: "1234567890abcdef"}}, &bytes.Buffer{})
	if called {
		t.Fatal("CI 非空時不應觸發原生對話框")
	}
}

func TestNotifyBlockedCommitSkipsNoGUIEnvironment(t *testing.T) {
	called := false
	stubNativeDialog(t, func([]stagedFinding) (string, error) {
		called = true
		return acknowledgeDialogButton, nil
	})
	t.Setenv("LEAKBOLT_NO_GUI", "1")

	notifyBlockedCommit(t.TempDir(), []stagedFinding{{Fingerprint: "1234567890abcdef"}}, &bytes.Buffer{})
	if called {
		t.Fatal("LEAKBOLT_NO_GUI 非空時不應觸發原生對話框")
	}
}

func TestNotifyBlockedCommitSkipsGitConfigFalse(t *testing.T) {
	repo := newNotifyTestRepo(t)
	mustRunGit(t, repo, "config", "leakbolt.gui", "false")
	called := false
	stubNativeDialog(t, func([]stagedFinding) (string, error) {
		called = true
		return acknowledgeDialogButton, nil
	})

	notifyBlockedCommit(repo, []stagedFinding{{Fingerprint: "1234567890abcdef"}}, &bytes.Buffer{})
	if called {
		t.Fatal("leakbolt.gui=false 時不應觸發原生對話框")
	}
}

func TestDialogFailureDoesNotChangeBlockedExitCode(t *testing.T) {
	repo := newNotifyTestRepo(t)
	stubNativeDialog(t, func([]stagedFinding) (string, error) {
		return "", errors.New("osascript failed")
	})
	var stdout, stderr bytes.Buffer
	findings := []Finding{{RuleID: "test-rule", File: "secret.txt", Secret: "must-not-appear"}}

	if code := finishStagedScan(repo, findings, &stdout, &stderr); code != 1 {
		t.Fatalf("osascript 失敗後 exit = %d, want 1", code)
	}
	if stderr.Len() != 0 {
		t.Fatalf("osascript 失敗應靜默，stderr = %q", stderr.String())
	}
	state, err := loadState(repo)
	if err != nil {
		t.Fatal(err)
	}
	want := fmt.Sprintf("暫存區掃描完成：找到 1 筆，未標記 1 筆。\n- 檔案 secret.txt｜規則 test-rule｜指紋 %s\ncommit 已中止。確認是誤報可執行 leakbolt allow <指紋前綴>。\n", shortFingerprint(state.Recent[0].Fingerprint))
	if stdout.String() != want {
		t.Fatalf("既有終端機輸出 = %q, want %q", stdout.String(), want)
	}
	if strings.Contains(stdout.String(), "must-not-appear") {
		t.Fatalf("終端機輸出洩漏原始命中：%q", stdout.String())
	}
}

func TestAllowDialogChoiceKeepsBlockedExitAndAllowsFingerprint(t *testing.T) {
	repo := newNotifyTestRepo(t)
	stubNativeDialog(t, func([]stagedFinding) (string, error) {
		return allowDialogButton, nil
	})
	var stdout, stderr bytes.Buffer
	findings := []Finding{{RuleID: "test-rule", File: "secret.txt", Secret: "test-secret"}}

	if code := finishStagedScan(repo, findings, &stdout, &stderr); code != 1 {
		t.Fatalf("放行後不應自動重跑 commit，exit = %d, want 1", code)
	}
	state, err := loadState(repo)
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Allowlist) != 1 {
		t.Fatalf("allowlist = %#v, want one entry", state.Allowlist)
	}
	want := "已將指紋 " + shortFingerprint(state.Allowlist[0].Fingerprint) + "（規則 test-rule）標記為誤報；請重新執行 commit。\n"
	if !strings.HasSuffix(stdout.String(), want) {
		t.Fatalf("放行說明 = %q, want suffix %q", stdout.String(), want)
	}
}

func TestAppleScriptStringLiteralEscapesSpecialCharacters(t *testing.T) {
	got := appleScriptStringLiteral("a\"b\\c\n\r\t")
	want := "\"a\\\"b\\\\c\\n\\r\\t\""
	if got != want {
		t.Fatalf("AppleScript literal = %q, want %q", got, want)
	}

	attack := "bad\" & do shell script \"touch /tmp/leakbolt-injected\" & \""
	script := blockedCommitDialogScript([]stagedFinding{{
		Finding:     Finding{File: attack, RuleID: "rule", Secret: "raw-secret"},
		Fingerprint: "1234567890abcdef",
	}})
	if strings.Contains(script, attack) {
		t.Fatalf("未轉義字串進入 AppleScript：%s", script)
	}
	if strings.Contains(script, "raw-secret") {
		t.Fatalf("AppleScript 不可包含原始命中：%s", script)
	}
}

func TestDialogWithMultipleFindingsHasNoAllowButton(t *testing.T) {
	findings := []stagedFinding{
		{Finding: Finding{File: "one.txt", RuleID: "rule-one"}, Fingerprint: "111111111111aaaa"},
		{Finding: Finding{File: "two.txt", RuleID: "rule-two"}, Fingerprint: "222222222222bbbb"},
	}
	script := blockedCommitDialogScript(findings)
	if strings.Contains(script, appleScriptStringLiteral(allowDialogButton)) {
		t.Fatalf("多筆命中不應有放行按鈕：%s", script)
	}
	want := "buttons {\"知道了\"} default button \"知道了\""
	if !strings.Contains(script, want) {
		t.Fatalf("多筆命中按鈕設定錯誤：%s", script)
	}
}

func TestSingleFindingUsesAcknowledgeAsDefaultButton(t *testing.T) {
	script := blockedCommitDialogScript([]stagedFinding{{Fingerprint: "1234567890abcdef"}})
	if !strings.Contains(script, "default button \"知道了\"") {
		t.Fatalf("預設按鈕必須是知道了：%s", script)
	}
	if strings.Contains(script, "default button \"標為誤報並放行\"") {
		t.Fatalf("放行按鈕不可是預設：%s", script)
	}
}

func TestDialogListsAtMostFiveFindings(t *testing.T) {
	var findings []stagedFinding
	for i := 1; i <= 6; i++ {
		findings = append(findings, stagedFinding{
			Finding:     Finding{File: "file-" + string(rune('0'+i)), RuleID: "rule"},
			Fingerprint: "1234567890123456",
		})
	}
	message := blockedCommitDialogMessage(findings)
	if got := strings.Count(message, "\n\n檔案："); got != 5 {
		t.Fatalf("列出 %d 筆，want 5：%q", got, message)
	}
	if !strings.Contains(message, "還有 1 筆") {
		t.Fatalf("缺少剩餘筆數：%q", message)
	}
}

func stubNativeDialog(t *testing.T, show func([]stagedFinding) (string, error)) {
	t.Helper()
	oldSupported := nativeDialogsSupported
	oldShow := showBlockedCommitDialog
	nativeDialogsSupported = func() bool { return true }
	showBlockedCommitDialog = show
	t.Cleanup(func() {
		nativeDialogsSupported = oldSupported
		showBlockedCommitDialog = oldShow
	})
	t.Setenv("CI", "")
	t.Setenv("LEAKBOLT_NO_GUI", "")
}

func newNotifyTestRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(t.TempDir(), "global.gitconfig"))
	t.Setenv("GIT_CONFIG_SYSTEM", filepath.Join(t.TempDir(), "system.gitconfig"))
	mustRunGit(t, repo, "init")
	return repo
}
