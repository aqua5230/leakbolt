package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func isolateGlobalGitConfig(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(home, "global.gitconfig"))
	t.Setenv("GIT_CONFIG_SYSTEM", filepath.Join(home, "system.gitconfig"))
	return home
}

func TestGlobalHookChainsFailingRepositoryHook(t *testing.T) {
	home := isolateGlobalGitConfig(t)
	repo := testGitRepo(t)
	repoHook := filepath.Join(repo, ".git", "hooks", "pre-commit")
	mustWrite(t, repoHook, "#!/bin/sh\necho repository-hook-ran >&2\nexit 23\n")
	if err := os.Chmod(repoHook, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := installGlobalHook(); err != nil {
		t.Fatal(err)
	}
	globalHook := filepath.Join(home, ".leakbolt", "hooks", "pre-commit")
	direct := exec.Command(shInterpreter, globalHook)
	direct.Dir = repo
	directOutput, directErr := direct.CombinedOutput()
	var directExitErr *exec.ExitError
	if !errors.As(directErr, &directExitErr) || directExitErr.ExitCode() != 23 {
		t.Fatalf("全域 hook exit = %v，預期傳遞 repo hook 的 23；output=%s", directErr, directOutput)
	}
	mustWrite(t, filepath.Join(repo, "staged.txt"), "staged\n")
	mustRunGit(t, repo, "add", "staged.txt")

	cmd := exec.Command("git", "commit", "-m", "must fail")
	cmd.Dir = repo
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("repo hook 失敗時 commit 必須中止，output=%s", output)
	}
	if !strings.Contains(string(output), "repository-hook-ran") {
		t.Fatalf("repo hook 未執行：%s", output)
	}
	if _, statErr := os.Stat(globalHook); statErr != nil {
		t.Fatalf("全域 hook 未建立：%v", statErr)
	}
}

func TestInstallGlobalRefusesOccupiedHooksPath(t *testing.T) {
	isolateGlobalGitConfig(t)
	repo := testGitRepo(t)
	mustRunGit(t, repo, "config", "--global", "core.hooksPath", "/other/tool/hooks")

	var stdout, stderr bytes.Buffer
	if code := runInstall([]string{"--global"}, &stdout, &stderr); code != 2 {
		t.Fatalf("install --global exit = %d, want 2; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "/other/tool/hooks") || !strings.Contains(stderr.String(), "不會覆寫") {
		t.Fatalf("衝突訊息不完整：%q", stderr.String())
	}
	value, set, err := gitConfig(repo, "global")
	if err != nil || !set || value != "/other/tool/hooks" {
		t.Fatalf("既有 core.hooksPath 被改動：value=%q set=%v err=%v", value, set, err)
	}
}

func TestUninstallGlobalOnlyUnsetsLeakBoltPath(t *testing.T) {
	t.Run("unset own value", func(t *testing.T) {
		isolateGlobalGitConfig(t)
		hookPath, err := installGlobalHook()
		if err != nil {
			t.Fatal(err)
		}
		var stdout, stderr bytes.Buffer
		if code := runUninstall([]string{"--global"}, &stdout, &stderr); code != 0 {
			t.Fatalf("uninstall --global exit = %d; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
		}
		if _, set, err := gitConfig("", "global"); err != nil || set {
			t.Fatalf("LeakBolt 的 core.hooksPath 未解除：set=%v err=%v", set, err)
		}
		if _, err := os.Stat(hookPath); err != nil {
			t.Fatalf("解除後 hook 檔案應保留：%v", err)
		}
	})

	t.Run("refuse other value", func(t *testing.T) {
		isolateGlobalGitConfig(t)
		repo := testGitRepo(t)
		mustRunGit(t, repo, "config", "--global", "core.hooksPath", "/other/tool/hooks")
		var stdout, stderr bytes.Buffer
		if code := runUninstall([]string{"--global"}, &stdout, &stderr); code != 2 {
			t.Fatalf("uninstall --global exit = %d, want 2; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
		}
		value, set, err := gitConfig(repo, "global")
		if err != nil || !set || value != "/other/tool/hooks" {
			t.Fatalf("別人的 core.hooksPath 被改動：value=%q set=%v err=%v", value, set, err)
		}
	})
}

func TestDoctorRecognizesGlobalModeWithoutRepositoryState(t *testing.T) {
	isolateGlobalGitConfig(t)
	repo := testGitRepo(t)
	if _, err := installGlobalHook(); err != nil {
		t.Fatal(err)
	}

	var output bytes.Buffer
	_ = runDoctor(repo, &output)
	if !strings.Contains(output.String(), "保護模式：全域模式") {
		t.Fatalf("doctor 未辨識全域模式：%s", output.String())
	}
	if !strings.Contains(output.String(), "core.hooksPath (global)：正常") {
		t.Fatalf("doctor 將 LeakBolt 全域路徑判成異常：%s", output.String())
	}
	if strings.Contains(output.String(), "找不到 state.json") {
		t.Fatalf("全域模式不應要求 repo state.json：%s", output.String())
	}
}
