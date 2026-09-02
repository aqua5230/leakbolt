package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 全域保護裝好之後，repo-local 或 worktree 層級的 core.hooksPath 會蓋過 global，
// 讓那個 repo 靜默失去保護（husky init 就會設 local）。doctor 必須抓到並報異常——
// 回報「正常」等於宣告一個已經失效的防護還在，這是資安工具最不該有的行為。
func TestGlobalDoctorDetectsShadowedHooksPath(t *testing.T) {
	cases := []struct {
		name       string
		configArgs []string
		wantScope  string
	}{
		{
			name:       "husky 設的 repo-local 路徑",
			configArgs: []string{"config", "--local", "core.hooksPath", ".husky/_"},
			wantScope:  "local",
		},
		{
			name:       "worktree 層級路徑",
			configArgs: []string{"config", "--worktree", "core.hooksPath", ".worktree-hooks"},
			wantScope:  "worktree",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			isolateGlobalGitConfig(t)
			repo := testGitRepo(t)
			if _, err := installGlobalHook(); err != nil {
				t.Fatal(err)
			}
			if c.wantScope == "worktree" {
				mustRunGit(t, repo, "config", "extensions.worktreeConfig", "true")
			}
			mustRunGit(t, repo, c.configArgs...)

			var stdout bytes.Buffer
			code := runDoctor(repo, &stdout)
			output := stdout.String()

			if code == 0 {
				t.Fatalf("%s 蓋過全域保護，doctor 卻回報正常（exit 0）；輸出：%q", c.wantScope, output)
			}
			if !strings.Contains(output, "core.hooksPath ("+c.wantScope+")：異常") {
				t.Errorf("doctor 沒指出是 %s 蓋掉的；輸出：%q", c.wantScope, output)
			}
			if !strings.Contains(output, "不會被檢查") {
				t.Errorf("doctor 沒說明防護已失效；輸出：%q", output)
			}
		})
	}
}

// 被 husky 蓋掉之後跑 leakbolt install，檢查會寫進 .husky/pre-commit，
// commit 照樣被擋。這時 doctor 必須回報正常——保護還在卻報異常是假警報，
// 使用者會學會忽略 doctor 的輸出。
func TestGlobalDoctorAcceptsCoexistenceWithHusky(t *testing.T) {
	isolateGlobalGitConfig(t)
	repo := testGitRepo(t)
	if _, err := installGlobalHook(); err != nil {
		t.Fatal(err)
	}

	// 重現 npx husky init 的結果：.husky/_ 轉呼叫殼 + repo-local hooksPath。
	if err := os.MkdirAll(filepath.Join(repo, ".husky", "_"), 0o755); err != nil {
		t.Fatal(err)
	}
	shim := filepath.Join(repo, ".husky", "_", "pre-commit")
	mustWrite(t, shim, "#!/bin/sh\n. \"${0%/*}/../pre-commit\"\n")
	if err := os.Chmod(shim, 0o755); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(repo, ".husky", "pre-commit"), "npm test\n")
	mustRunGit(t, repo, "config", "--local", "core.hooksPath", ".husky/_")

	// 共存前：保護確實沒了，doctor 要報異常。
	var before bytes.Buffer
	if code := runDoctor(repo, &before); code == 0 {
		t.Fatalf("husky 蓋掉保護，doctor 卻回報正常；輸出：%q", before.String())
	}

	// leakbolt install 把檢查寫進 husky 的 hook。
	if _, err := installHook(repo, false); err != nil {
		t.Fatalf("被 husky 蓋掉後 install 失敗（應該要能寫進 .husky/pre-commit）：%v", err)
	}

	var after bytes.Buffer
	code := runDoctor(repo, &after)
	output := after.String()
	if strings.Contains(output, "蓋過全機保護") {
		t.Errorf("已與 husky 共存，doctor 仍報遮蔽；輸出：%q", output)
	}
	if strings.Contains(output, "hook 可達性：異常") {
		t.Errorf("hook 鏈上有 LeakBolt，可達性卻報異常；輸出：%q", output)
	}
	if code != 0 && !strings.Contains(output, "gitleaks") {
		t.Fatalf("共存狀態 doctor exit = %d，且與 gitleaks 無關；輸出：%q", code, output)
	}
}

// effectiveHookPath 必須反映 git 自己解析出的有效值。舊版自行按
// local→global→system 排序，漏掉 worktree scope，於是 worktree 設了 hooksPath 時
// 仍回傳全域路徑，讓可達性檢查誤判成正常。
func TestEffectiveHookPathHonoursWorktreeScope(t *testing.T) {
	isolateGlobalGitConfig(t)
	repo := testGitRepo(t)
	mustRunGit(t, repo, "config", "--global", "core.hooksPath", "/global/hooks")
	mustRunGit(t, repo, "config", "extensions.worktreeConfig", "true")
	mustRunGit(t, repo, "config", "--worktree", "core.hooksPath", ".worktree-hooks")

	got := effectiveHookPath(repo)
	want := filepath.Join(repo, ".worktree-hooks", "pre-commit")
	if got != want {
		t.Fatalf("effectiveHookPath = %q, want %q（worktree scope 被漏掉了）", got, want)
	}
}

// 只裝全域保護、沒有任何遮蔽時，doctor 要回報正常。
// 上面那個測試若因為誤判而永遠報異常，這裡會失敗。
func TestGlobalDoctorPassesWithoutShadowing(t *testing.T) {
	isolateGlobalGitConfig(t)
	repo := testGitRepo(t)
	if _, err := installGlobalHook(); err != nil {
		t.Fatal(err)
	}

	var stdout bytes.Buffer
	code := runDoctor(repo, &stdout)
	if strings.Contains(stdout.String(), "蓋過全域保護") {
		t.Fatalf("沒有遮蔽卻報了遮蔽：%q", stdout.String())
	}
	// gitleaks 不一定裝在測試機上，所以只斷言不是因為遮蔽而失敗。
	if code != 0 && !strings.Contains(stdout.String(), "gitleaks") {
		t.Fatalf("doctor exit = %d，且與 gitleaks 無關；輸出：%q", code, stdout.String())
	}
}
