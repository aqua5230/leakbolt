package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestDetectHookTarget(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(t *testing.T, repo string)
		localOnly bool
		wantKind  string
		wantPath  string
		tracked   bool
	}{
		{name: "no manager", wantKind: "script", wantPath: ".git/hooks/pre-commit"},
		{name: "husky", wantKind: "script", wantPath: ".husky/pre-commit", tracked: true, setup: func(t *testing.T, repo string) { mustMkdir(t, filepath.Join(repo, ".husky")) }},
		{name: "lefthook yml", wantKind: "lefthook", wantPath: "lefthook.yml", tracked: true, setup: func(t *testing.T, repo string) { mustWrite(t, filepath.Join(repo, "lefthook.yml"), "") }},
		{name: "lefthook yaml", wantKind: "lefthook", wantPath: "lefthook.yaml", tracked: true, setup: func(t *testing.T, repo string) { mustWrite(t, filepath.Join(repo, "lefthook.yaml"), "") }},
		{name: "pre commit", wantKind: "precommit", wantPath: ".pre-commit-config.yaml", tracked: true, setup: func(t *testing.T, repo string) {
			mustWrite(t, filepath.Join(repo, ".pre-commit-config.yaml"), "repos: []\n")
		}},
		{name: "local only wins", localOnly: true, wantKind: "script", wantPath: ".git/hooks/pre-commit", setup: func(t *testing.T, repo string) { mustMkdir(t, filepath.Join(repo, ".husky")) }},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repo := t.TempDir()
			mustMkdir(t, filepath.Join(repo, ".git", "hooks"))
			if test.setup != nil {
				test.setup(t, repo)
			}
			target, err := detectHookTarget(repo, test.localOnly)
			if err != nil {
				t.Fatal(err)
			}
			if target.Kind != test.wantKind || target.Path != filepath.Join(repo, test.wantPath) || target.VersionControlled != test.tracked {
				t.Fatalf("target = %#v", target)
			}
		})
	}
}

func TestAppendScriptHookIsIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pre-commit")
	if err := writeScriptHook(path); err != nil {
		t.Fatal(err)
	}
	if err := writeScriptHook(path); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "#!/bin/sh\n# Added by LeakBolt\n"+hookGuard {
		t.Fatalf("unexpected hook:\n%s", content)
	}
}

func TestAppendLefthookAddsToPreCommitBlock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "lefthook.yml")
	mustWrite(t, path, "pre-push:\n  commands:\n    existing:\n      run: echo push\npre-commit:\n  commands:\n    existing:\n      run: echo commit\n")
	if err := appendLefthook(path); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := "pre-push:\n  commands:\n    existing:\n      run: echo push\npre-commit:\n  commands:\n    leakbolt:\n      run: |\n        if command -v leakbolt >/dev/null 2>&1; then\n          leakbolt scan --staged || exit 1\n        fi\n    existing:\n      run: echo commit\n"
	if string(content) != want {
		t.Fatalf("unexpected lefthook config:\n%s", content)
	}
}

func TestAppendPreCommitConfigExpandsEmptyRepos(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".pre-commit-config.yaml")
	mustWrite(t, path, "repos: []\n")
	if err := appendPreCommitConfig(path); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(content[:len("repos:\n")]) != "repos:\n" || !containsGuard(string(content)) {
		t.Fatalf("unexpected pre-commit config:\n%s", content)
	}
}

func containsGuard(content string) bool {
	return strings.Contains(content, "if command -v leakbolt >/dev/null 2>&1; then") &&
		strings.Contains(content, "leakbolt scan --staged || exit 1")
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestIsHuskyHooksPath(t *testing.T) {
	repo := t.TempDir()
	mustMkdir(t, filepath.Join(repo, ".husky"))

	cases := []struct {
		name  string
		value string
		want  bool
	}{
		{"husky 相對路徑", filepath.Join(".husky", "_"), true},
		{"husky 絕對路徑", filepath.Join(repo, ".husky", "_"), true},
		{"husky 帶結尾斜線", filepath.Join(".husky", "_") + string(filepath.Separator), true},
		{"別的工具佔用", filepath.Join(".githooks"), false},
		{"空值", "", false},
	}
	for _, c := range cases {
		if got := isHuskyHooksPath(repo, c.value); got != c.want {
			t.Errorf("%s: isHuskyHooksPath(%q) = %v, want %v", c.name, c.value, got, c.want)
		}
	}

	// 沒有 .husky 目錄時，就算 core.hooksPath 指向 .husky/_ 也不算 husky
	bare := t.TempDir()
	if isHuskyHooksPath(bare, filepath.Join(".husky", "_")) {
		t.Error("沒有 .husky 目錄時不應判定為 husky")
	}
}

func TestWriteScriptHookRunsFirst(t *testing.T) {
	// husky 預設的 pre-commit 內容會先失敗，我們的檢查必須排在它前面才會被執行。
	dir := t.TempDir()
	path := filepath.Join(dir, "pre-commit")
	mustWrite(t, path, "#!/usr/bin/env sh\nnpm test\n")

	if err := writeScriptHook(path); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)

	if !strings.HasPrefix(got, "#!/usr/bin/env sh\n") {
		t.Errorf("原本的 shebang 應保留在第一行，實際：%q", got)
	}
	guardAt := strings.Index(got, "leakbolt scan --staged")
	npmAt := strings.Index(got, "npm test")
	if guardAt < 0 || npmAt < 0 {
		t.Fatalf("兩段內容都要在，實際：%q", got)
	}
	if guardAt > npmAt {
		t.Errorf("LeakBolt 檢查必須排在 npm test 之前，實際：%q", got)
	}

	// 重複安裝不應該重複寫入
	if err := writeScriptHook(path); err != nil {
		t.Fatal(err)
	}
	again, _ := os.ReadFile(path)
	if strings.Count(string(again), "leakbolt scan --staged") != 1 {
		t.Errorf("重複安裝不應重複寫入，實際：%q", string(again))
	}
}

func TestScriptHookDoesNotKillOtherChecks(t *testing.T) {
	// 守衛不能用 exit 0 離開腳本——那會把隊友的其他檢查一起關掉。
	// .husky/pre-commit 是進版控的檔案，沒裝 leakbolt 的隊友也會執行到它。
	dir := t.TempDir()
	path := filepath.Join(dir, "pre-commit")
	mustWrite(t, path, "#!/bin/sh\nnpm test\n")
	if err := writeScriptHook(path); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	if strings.Contains(got, "|| exit 0") {
		t.Errorf("守衛不可用 || exit 0（會結束整個腳本），實際：%q", got)
	}
	if !strings.Contains(got, "if command -v leakbolt") {
		t.Errorf("守衛應為 if 區塊，實際：%q", got)
	}
	if !strings.Contains(got, "npm test") {
		t.Errorf("原本的檢查必須保留，實際：%q", got)
	}
}

func TestVerifyHookReachableIgnoresExecBitOnWindows(t *testing.T) {
	// 這個測試鎖住的是判斷邏輯的形狀，不是 Windows 上的實際行為
	// （在 macOS/Linux 上跑不到那條分支）。真正的驗證要在 Windows 機器上跑
	// scripts/windows_matrix.ps1。
	dir := t.TempDir()
	repo := dir
	mustMkdir(t, filepath.Join(repo, ".git", "hooks"))
	path := filepath.Join(repo, ".git", "hooks", "pre-commit")
	mustWrite(t, path, "#!/bin/sh\n")
	if err := os.Chmod(path, 0o644); err != nil { // 存在但沒有執行位元
		t.Fatal(err)
	}

	_, ok := verifyHookReachable(repo)
	if runtime.GOOS == "windows" {
		if !ok {
			t.Error("Windows 上不該用執行位元判斷可達性")
		}
		return
	}
	if ok {
		t.Error("非 Windows 上，沒有執行位元就該判定不可達")
	}
}
