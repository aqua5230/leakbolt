package main

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// 在暫存目錄放一支假的 leakbolt，並把它擺到 PATH 最前面。
func fakeLeakboltOnPATH(t *testing.T, script string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("測試使用 POSIX shell 假執行檔")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "leakbolt"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+restrictedPATHWithoutLeakbolt)
}

func doctorOutputWithFakeLeakbolt(t *testing.T, script string) string {
	t.Helper()
	fakeLeakboltOnPATH(t, script)
	repo := testGitRepo(t)
	t.Setenv("GIT_CONFIG_SYSTEM", filepath.Join(t.TempDir(), "system.gitconfig"))
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(t.TempDir(), "global.gitconfig"))
	if err := writeScriptHook(filepath.Join(repo, ".git", "hooks", "pre-commit"), false); err != nil {
		t.Fatal(err)
	}
	if err := recordInstallation(repo, "script"); err != nil {
		t.Fatal(err)
	}

	var output bytes.Buffer
	runDoctor(repo, &output)
	return output.String()
}

// 補上版本識別以前的舊版執行檔不認得 --version。先前 doctor 只看檔案在不在，
// 會把這種「原始碼修過但沒重裝」的機器回報成正常。
func TestDoctorFlagsInstalledBinaryWithoutVersionSupport(t *testing.T) {
	output := doctorOutputWithFakeLeakbolt(t, "#!/bin/sh\necho '未知子指令：--version' >&2\nexit 2\n")
	if !strings.Contains(output, "leakbolt 執行檔：異常") {
		t.Fatalf("舊版執行檔應判異常：%s", output)
	}
	if !strings.Contains(output, "認不得 --version") {
		t.Fatalf("doctor 未說明原因：%s", output)
	}
}

func TestDoctorReportsInstalledBinaryVersion(t *testing.T) {
	output := doctorOutputWithFakeLeakbolt(t, "#!/bin/sh\necho 'leakbolt 9.9.9 (abc1234)，鎖定 gitleaks 8.30.1'\n")
	if !strings.Contains(output, "leakbolt 執行檔：正常") {
		t.Fatalf("能回報版本的執行檔應判正常：%s", output)
	}
	if !strings.Contains(output, "leakbolt 9.9.9 (abc1234)") {
		t.Fatalf("doctor 應印出 PATH 上那支自己報的版本：%s", output)
	}
}
