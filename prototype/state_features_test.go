package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestFingerprintIsStableAndSalted(t *testing.T) {
	first := &State{Salt: strings.Repeat("01", 32)}
	second := &State{Salt: strings.Repeat("02", 32)}
	one, err := fingerprint(first, "test-rule", "test-secret")
	if err != nil {
		t.Fatal(err)
	}
	again, err := fingerprint(first, "test-rule", "test-secret")
	if err != nil {
		t.Fatal(err)
	}
	two, err := fingerprint(second, "test-rule", "test-secret")
	if err != nil {
		t.Fatal(err)
	}
	if one != again {
		t.Fatal("相同輸入必須產生相同指紋")
	}
	if one == two {
		t.Fatal("不同 salt 必須產生不同指紋")
	}
}

func TestStateNeverStoresRawSecret(t *testing.T) {
	repo := testRepo(t)
	state, err := newState()
	if err != nil {
		t.Fatal(err)
	}
	secret := "not-a-real-secret-1f710b4f"
	if _, err := prepareStagedFindings(state, []Finding{{RuleID: "test-rule", Secret: secret}}); err != nil {
		t.Fatal(err)
	}
	if err := writeState(repo, state); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(statePath(repo))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(data, []byte(secret)) {
		t.Fatalf("state.json 不可包含原始 secret：%s", data)
	}
	info, err := os.Stat(statePath(repo))
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS == "windows" {
		// Windows 的 os.Chmod 只能切換唯讀屬性，無法設定 POSIX 0600。
		if !info.Mode().IsRegular() {
			t.Fatalf("state.json 必須是一般檔案，mode = %v", info.Mode())
		}
		return
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("state.json 權限 = %o，want 0600", info.Mode().Perm())
	}
}

func TestAllowValidatesPrefixAndSkipsMarkedFinding(t *testing.T) {
	repo := testRepo(t)
	state := &State{Salt: strings.Repeat("03", 32)}
	findings := []Finding{{RuleID: "test-rule", Secret: "not-a-real-secret-2d4b3b2b"}}
	visible, err := prepareStagedFindings(state, findings)
	if err != nil {
		t.Fatal(err)
	}
	if len(visible) != 1 {
		t.Fatalf("visible = %d, want 1", len(visible))
	}
	if err := writeState(repo, state); err != nil {
		t.Fatal(err)
	}
	if _, _, err := allowRecentFingerprint(repo, "short"); err == nil {
		t.Fatal("短於 8 字元的前綴必須失敗")
	}
	entry, alreadyAllowed, err := allowRecentFingerprint(repo, visible[0].Fingerprint[:8])
	if err != nil {
		t.Fatal(err)
	}
	if alreadyAllowed || entry.Fingerprint != visible[0].Fingerprint {
		t.Fatalf("allow 結果 = %#v, already=%v", entry, alreadyAllowed)
	}
	state, err = loadState(repo)
	if err != nil {
		t.Fatal(err)
	}
	visible, err = prepareStagedFindings(state, findings)
	if err != nil {
		t.Fatal(err)
	}
	if len(visible) != 0 {
		t.Fatalf("已標記命中仍被輸出：%#v", visible)
	}
}

func TestAllowRejectsAmbiguousPrefix(t *testing.T) {
	repo := testRepo(t)
	state := &State{
		Salt: strings.Repeat("04", 32),
		Recent: []RecentFinding{
			{Fingerprint: "aaaaaaaa11111111111111111111111111111111111111111111111111111111", RuleID: "one"},
			{Fingerprint: "aaaaaaaa22222222222222222222222222222222222222222222222222222222", RuleID: "two"},
		},
	}
	if err := writeState(repo, state); err != nil {
		t.Fatal(err)
	}
	if _, _, err := allowRecentFingerprint(repo, "aaaaaaaa"); err == nil {
		t.Fatal("比對到多筆的前綴必須失敗")
	}
}

func TestUntrackUntrackedFileDoesNothing(t *testing.T) {
	repo := testGitRepo(t)
	mustWrite(t, filepath.Join(repo, ".env"), "TOKEN=not-a-real-secret\n")
	tracked, _, err := untrackFile(repo, ".env")
	if err != nil {
		t.Fatal(err)
	}
	if tracked {
		t.Fatal("未追蹤檔案不可回報已處理")
	}
	if _, err := os.Stat(filepath.Join(repo, ".env")); err != nil {
		t.Fatalf("工作目錄檔案不可被刪除：%v", err)
	}
}

func TestUntrackRemovesIndexKeepsFileAndUpdatesIgnore(t *testing.T) {
	repo := testGitRepo(t)
	path := filepath.Join(repo, ".env")
	mustWrite(t, path, "TOKEN=not-a-real-secret\n")
	mustRunGit(t, repo, "add", ".env")

	tracked, _, err := untrackFile(repo, ".env")
	if err != nil {
		t.Fatal(err)
	}
	if !tracked {
		t.Fatal("追蹤檔案應完成處理")
	}
	stillTracked, err := gitTracked(repo, ".env")
	if err != nil {
		t.Fatal(err)
	}
	if stillTracked {
		t.Fatal("檔案仍在 index")
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "TOKEN=not-a-real-secret\n" {
		t.Fatalf("工作目錄檔案應保留，data=%q err=%v", data, err)
	}
	ignore, err := os.ReadFile(filepath.Join(repo, ".gitignore"))
	if err != nil || !gitignoreContains(ignore, ".env") {
		t.Fatalf(".gitignore 未補上 .env，data=%q err=%v", ignore, err)
	}
	backups, err := filepath.Glob(filepath.Join(stateDirectory(repo), "backup", "*", ".env"))
	if err != nil || len(backups) != 1 {
		t.Fatalf("備份數量 = %d, err=%v", len(backups), err)
	}
}

func TestDoctorDetectsChangedHooksPath(t *testing.T) {
	repo := testGitRepo(t)
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(t.TempDir(), "global.gitconfig"))
	if err := writeScriptHook(filepath.Join(repo, ".git", "hooks", "pre-commit"), false); err != nil {
		t.Fatal(err)
	}
	if err := recordInstallation(repo, "script"); err != nil {
		t.Fatal(err)
	}
	mustRunGit(t, repo, "config", "core.hooksPath", ".other-hooks")
	var output bytes.Buffer
	if code := runDoctor(repo, &output); code != 1 {
		t.Fatalf("doctor exit = %d, output=%s", code, output.String())
	}
	if !strings.Contains(output.String(), "core.hooksPath (local)：異常") {
		t.Fatalf("doctor 未偵測 hooksPath 改動：%s", output.String())
	}
}

func TestDoctorDetectsChangedSystemHooksPath(t *testing.T) {
	repo := testGitRepo(t)
	systemConfig := filepath.Join(t.TempDir(), "system.gitconfig")
	t.Setenv("GIT_CONFIG_SYSTEM", systemConfig)
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(t.TempDir(), "global.gitconfig"))
	if err := writeScriptHook(filepath.Join(repo, ".git", "hooks", "pre-commit"), false); err != nil {
		t.Fatal(err)
	}
	if err := recordInstallation(repo, "script"); err != nil {
		t.Fatal(err)
	}
	mustRunGit(t, repo, "config", "--file", systemConfig, "core.hooksPath", ".system-hooks")

	var output bytes.Buffer
	if code := runDoctor(repo, &output); code != 1 {
		t.Fatalf("doctor exit = %d, output=%s", code, output.String())
	}
	if !strings.Contains(output.String(), "core.hooksPath (system)：異常") {
		t.Fatalf("doctor 未偵測 system hooksPath 改動：%s", output.String())
	}
}

func TestDoctorReportsUnrecordedSystemHooksPathForOldState(t *testing.T) {
	repo := testGitRepo(t)
	t.Setenv("GIT_CONFIG_SYSTEM", filepath.Join(t.TempDir(), "system.gitconfig"))
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(t.TempDir(), "global.gitconfig"))
	if err := writeScriptHook(filepath.Join(repo, ".git", "hooks", "pre-commit"), false); err != nil {
		t.Fatal(err)
	}
	if err := recordInstallation(repo, "script"); err != nil {
		t.Fatal(err)
	}
	state, err := loadState(repo)
	if err != nil {
		t.Fatal(err)
	}
	state.Install.SystemHooksPathDigest = ""
	if err := writeState(repo, state); err != nil {
		t.Fatal(err)
	}

	var output bytes.Buffer
	if code := runDoctor(repo, &output); code != 1 {
		t.Fatalf("doctor exit = %d, output=%s", code, output.String())
	}
	want := "core.hooksPath (system)：未記錄（舊版安裝記錄，請重新執行 leakbolt install）"
	if !strings.Contains(output.String(), want) {
		t.Fatalf("doctor 未回報舊版 state：%s", output.String())
	}
}

func TestDoctorReportsLeakboltDisabled(t *testing.T) {
	repo := testGitRepo(t)
	t.Setenv("GIT_CONFIG_SYSTEM", filepath.Join(t.TempDir(), "system.gitconfig"))
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(t.TempDir(), "global.gitconfig"))
	if err := writeScriptHook(filepath.Join(repo, ".git", "hooks", "pre-commit"), false); err != nil {
		t.Fatal(err)
	}
	if err := recordInstallation(repo, "script"); err != nil {
		t.Fatal(err)
	}
	mustRunGit(t, repo, "config", "hooks.leakbolt", "false")

	var output bytes.Buffer
	if code := runDoctor(repo, &output); code != 1 {
		t.Fatalf("doctor exit = %d, output=%s", code, output.String())
	}
	if !strings.Contains(output.String(), "LeakBolt 啟用狀態：異常") {
		t.Fatalf("doctor 未回報 LeakBolt 停用：%s", output.String())
	}
}

func testRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	mustMkdir(t, filepath.Join(repo, ".git"))
	return repo
}

func testGitRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	mustRunGit(t, repo, "init")
	mustRunGit(t, repo, "config", "user.email", "leakbolt-test@example.invalid")
	mustRunGit(t, repo, "config", "user.name", "LeakBolt Test")
	return repo
}

func mustRunGit(t *testing.T, repo string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = repo
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, output)
	}
}

func TestStateDirectoryInWorktree(t *testing.T) {
	// git worktree 裡 .git 是檔案不是目錄，直接拼 <repo>/.git 會開檔失敗。
	// 狀態檔要跟 hook 一樣放在共用的 git 目錄下。
	repo := t.TempDir()
	run := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run(repo, "init", "-q", ".")
	run(repo, "config", "user.email", "t@t")
	run(repo, "config", "user.name", "t")
	if err := os.WriteFile(filepath.Join(repo, "a.txt"), []byte("hi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(repo, "add", "a.txt")
	run(repo, "commit", "-qm", "init")

	wt := filepath.Join(t.TempDir(), "wt")
	run(repo, "worktree", "add", "-q", "-b", "wtbranch", wt)

	// 真的要能在 worktree 裡建立與寫入狀態（原本的 bug 是在這裡開檔失敗：
	// worktree 的 .git 是檔案不是目錄，拼出 <repo>/.git/leakbolt 會開檔失敗）
	state, err := loadOrCreateState(wt)
	if err != nil {
		t.Fatalf("worktree 建立狀態失敗：%v", err)
	}
	if err := writeState(wt, state); err != nil {
		t.Fatalf("worktree 寫入狀態失敗：%v", err)
	}

	// 目錄建好了才解得開符號連結（macOS 的 /var 通往 /private/var）
	resolve := func(p string) string {
		if r, err := filepath.EvalSymlinks(p); err == nil {
			return r
		}
		return p
	}
	if mainDir, wtDir := resolve(stateDirectory(repo)), resolve(stateDirectory(wt)); mainDir != wtDir {
		t.Errorf("worktree 應與主 repo 共用狀態目錄：主=%q worktree=%q", mainDir, wtDir)
	}

	// worktree 寫的狀態，主 repo 要讀得到
	if _, err := loadState(repo); err != nil {
		t.Fatalf("主 repo 應該讀得到 worktree 寫的狀態：%v", err)
	}
}
