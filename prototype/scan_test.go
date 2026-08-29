package main

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestRunGitleaksChecksVersionOnceAndAcceptsVPrefix(t *testing.T) {
	repo, versionLog := useFakeGitleaks(t, " v8.30.1 ")
	var stderr bytes.Buffer
	if _, err := runGitleaks(repo, &stderr, "detect", "--report-format", "json", "--report-path", "-"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(versionLog)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(string(data), "version\n"); got != 1 {
		t.Fatalf("gitleaks version 查詢次數 = %d, want 1", got)
	}
	if stderr.Len() != 0 {
		t.Fatalf("鎖定版本不應警告：%s", stderr.String())
	}
}

func TestRunGitleaksWarnsOnVersionMismatchAndContinues(t *testing.T) {
	repo, _ := useFakeGitleaks(t, "8.30.2")
	var stderr bytes.Buffer
	if _, err := runGitleaks(repo, &stderr, "detect", "--report-format", "json", "--report-path", "-"); err != nil {
		t.Fatalf("版本不符仍應繼續掃描：%v", err)
	}
	want := "警告：gitleaks 版本為 8.30.2，LeakBolt 鎖定的是 8.30.1。偵測結果可能與品質基準不同。\n"
	if stderr.String() != want {
		t.Fatalf("警告 = %q, want %q", stderr.String(), want)
	}
}

func useFakeGitleaks(t *testing.T, version string) (string, string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("測試使用 POSIX shell 假執行檔")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "gitleaks")
	versionLog := filepath.Join(dir, "version.log")
	script := "#!/bin/sh\n" +
		"if [ \"$1\" = version ]; then\n" +
		"  printf 'version\\n' >> \"$LEAKBOLT_VERSION_LOG\"\n" +
		"  printf '%s\\n' \"$LEAKBOLT_FAKE_VERSION\"\n" +
		"  exit 0\n" +
		"fi\n" +
		"printf '[]\\n'\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	t.Setenv("LEAKBOLT_VERSION_LOG", versionLog)
	t.Setenv("LEAKBOLT_FAKE_VERSION", version)
	return t.TempDir(), versionLog
}
