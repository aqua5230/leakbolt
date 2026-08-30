package main

import (
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

func gitRepositoryRoot(dir string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	cmd.Dir = dir
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("目前目錄不在 git repo 內；請在目標 repo 執行 leakbolt")
	}
	return strings.TrimSpace(string(output)), nil
}

func gitConfig(repo, scope string) (string, bool, error) {
	cmd := exec.Command("git", "config", "--"+scope, "--get", "core.hooksPath")
	cmd.Dir = repo
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err == nil {
		return strings.TrimSpace(string(output)), true, nil
	}
	if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
		return "", false, nil
	}
	return "", false, fmt.Errorf("讀取 git config --%s core.hooksPath 失敗：%s", scope, strings.TrimSpace(stderr.String()))
}

func gitConfigBool(repo, key string) (string, bool, error) {
	cmd := exec.Command("git", "config", "--bool", "--get", key)
	cmd.Dir = repo
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err == nil {
		return strings.TrimSpace(string(output)), true, nil
	}
	if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
		return "", false, nil
	}
	return "", false, fmt.Errorf("讀取 git config --bool --get %s 失敗：%s", key, strings.TrimSpace(stderr.String()))
}

func setGlobalHooksPath(path string) error {
	cmd := exec.Command("git", "config", "--global", "core.hooksPath", path)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("設定 git config --global core.hooksPath 失敗：%s", strings.TrimSpace(stderr.String()))
	}
	return nil
}

func unsetGlobalHooksPath() error {
	cmd := exec.Command("git", "config", "--global", "--unset", "core.hooksPath")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("解除 git config --global core.hooksPath 失敗：%s", strings.TrimSpace(stderr.String()))
	}
	return nil
}

// gitCommonDir 回傳這個 repo 共用的 git 目錄。
//
// 不可以自己把路徑拼成 <repo>/.git —— 在 git worktree 裡 .git 是一個檔案而不是目錄，
// 拼出來的路徑會變成「往檔案裡面找目錄」，開檔直接失敗。
// worktree 共用主 repo 的 .git/hooks，狀態檔也該放在同一個共用目錄下。
func gitCommonDir(repo string) string {
	cmd := exec.Command("git", "rev-parse", "--git-common-dir")
	cmd.Dir = repo
	output, err := cmd.Output()
	if err != nil {
		return filepath.Join(repo, ".git")
	}
	dir := strings.TrimSpace(string(output))
	if dir == "" {
		return filepath.Join(repo, ".git")
	}
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(repo, dir)
	}
	return dir
}
