package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func runUntrack(repo, input string, stdout io.Writer) int {
	tracked, backupPath, err := untrackFile(repo, input)
	if err != nil {
		fmt.Fprintf(stdout, "移出版控失敗：%v\n", err)
		return 2
	}
	if !tracked {
		fmt.Fprintln(stdout, "這個檔案目前未被 git 追蹤；不需處理。")
		return 0
	}
	fmt.Fprintln(stdout, "已移出版控並補上 .gitignore。")
	fmt.Fprintln(stdout, "移出版控不等於修好。這個檔案若曾進入 git 歷史，裡面的密鑰必須去供應商端作廢。")
	// 備份是原始內容的明文副本，使用者有權知道它在哪、怎麼刪掉。
	fmt.Fprintf(stdout, "已備份原始檔到 %s（明文，權限 0600，不進版控）。\n", backupPath)
	fmt.Fprintln(stdout, "確認不需要還原後，可用 leakbolt purge-backups 刪除全部備份。")
	return 0
}

// untrackFile 回傳 false 表示檔案原本就未被追蹤。
func untrackFile(repo, input string) (bool, string, error) {
	relative, absolute, err := repoFilePath(repo, input)
	if err != nil {
		return false, "", err
	}
	if strings.ContainsAny(relative, "\r\n") {
		return false, "", fmt.Errorf("不支援檔名含換行字元")
	}
	tracked, err := gitTracked(repo, relative)
	if err != nil || !tracked {
		return tracked, "", err
	}
	info, err := os.Lstat(absolute)
	if err != nil {
		return false, "", fmt.Errorf("讀取檔案失敗：%w", err)
	}
	if !info.Mode().IsRegular() {
		return false, "", fmt.Errorf("只支援一般檔案")
	}

	gitignore := filepath.Join(repo, ".gitignore")
	originalIgnore, ignoreExists, err := readGitignore(gitignore)
	if err != nil {
		return false, "", err
	}
	backupPath, err := backupTrackedFile(repo, relative, absolute)
	if err != nil {
		return false, "", fmt.Errorf("備份失敗：%w", err)
	}
	indexInfo, err := gitIndexInfo(repo, relative)
	if err != nil {
		return false, "", fmt.Errorf("讀取 index 失敗：%w", err)
	}

	indexChanged := false
	ignoreChanged := false
	if err := gitRun(repo, nil, "rm", "--cached", "--", relative); err != nil {
		return false, "", fmt.Errorf("git rm --cached 失敗：%w", err)
	}
	indexChanged = true
	if !gitignoreContains(originalIgnore, relative) {
		// os.WriteFile 失敗時仍可能已截斷檔案，先標記才能確實還原。
		ignoreChanged = true
		if err := writeGitignore(gitignore, originalIgnore, relative); err != nil {
			return false, "", rollbackUntrack(repo, relative, indexInfo, indexChanged, gitignore, originalIgnore, ignoreExists, ignoreChanged, "更新 .gitignore", err)
		}
	}
	stillTracked, err := gitTracked(repo, relative)
	if err != nil {
		return false, "", rollbackUntrack(repo, relative, indexInfo, indexChanged, gitignore, originalIgnore, ignoreExists, ignoreChanged, "確認 index", err)
	}
	if stillTracked {
		return false, "", rollbackUntrack(repo, relative, indexInfo, indexChanged, gitignore, originalIgnore, ignoreExists, ignoreChanged, "確認 index", fmt.Errorf("檔案仍在 index"))
	}
	return true, backupPath, nil
}

func repoFilePath(repo, input string) (string, string, error) {
	if input == "" {
		return "", "", fmt.Errorf("檔案路徑不可空白")
	}
	absolute := input
	if !filepath.IsAbs(absolute) {
		absolute = filepath.Join(repo, input)
	}
	absolute = filepath.Clean(absolute)
	relative, err := filepath.Rel(repo, absolute)
	if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", "", fmt.Errorf("檔案必須在目前 repo 內")
	}
	return filepath.ToSlash(relative), absolute, nil
}

func gitTracked(repo, relative string) (bool, error) {
	err := gitRun(repo, nil, "ls-files", "--error-unmatch", "--", relative)
	if err == nil {
		return true, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
		return false, nil
	}
	return false, fmt.Errorf("git ls-files 失敗：%w", err)
}

func gitIndexInfo(repo, relative string) ([]byte, error) {
	cmd := exec.Command("git", "ls-files", "--stage", "--", relative)
	cmd.Dir = repo
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	if len(output) == 0 {
		return nil, fmt.Errorf("找不到 index 項目")
	}
	return output, nil
}

func gitRun(repo string, input io.Reader, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = repo
	cmd.Stdin = input
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message != "" {
			return fmt.Errorf("%w：%s", err, message)
		}
		return err
	}
	return nil
}

func backupTrackedFile(repo, relative, source string) (string, error) {
	stamp := time.Now().UTC().Format("20060102T150405.000000000Z")
	if err := os.MkdirAll(stateDirectory(repo), 0o700); err != nil {
		return "", err
	}
	if err := os.Chmod(stateDirectory(repo), 0o700); err != nil {
		return "", err
	}
	destination := filepath.Join(stateDirectory(repo), "backup", stamp, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(destination), 0o700); err != nil {
		return "", err
	}
	if err := os.Chmod(filepath.Dir(destination), 0o700); err != nil {
		return "", err
	}
	in, err := os.Open(source)
	if err != nil {
		return "", err
	}
	defer in.Close()
	out, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return "", err
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		return "", copyErr
	}
	if closeErr != nil {
		return "", closeErr
	}
	return destination, nil
}

func readGitignore(path string) ([]byte, bool, error) {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, false, fmt.Errorf(".gitignore 不可為 symbolic link")
	}
	data, err := os.ReadFile(path)
	return data, true, err
}

func gitignoreContains(data []byte, path string) bool {
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSuffix(line, "\r") == path {
			return true
		}
	}
	return false
}

func writeGitignore(path string, original []byte, entry string) error {
	updated := append([]byte(nil), original...)
	if len(updated) > 0 && updated[len(updated)-1] != '\n' {
		updated = append(updated, '\n')
	}
	updated = append(updated, entry...)
	updated = append(updated, '\n')
	return os.WriteFile(path, updated, 0o644)
}

func rollbackUntrack(repo, relative string, indexInfo []byte, indexChanged bool, gitignore string, originalIgnore []byte, ignoreExists, ignoreChanged bool, step string, cause error) error {
	var rollbackErrors []string
	if indexChanged {
		if err := gitRun(repo, bytes.NewReader(indexInfo), "update-index", "--index-info"); err != nil {
			rollbackErrors = append(rollbackErrors, "index: "+err.Error())
		}
	}
	if ignoreChanged {
		var err error
		if ignoreExists {
			err = os.WriteFile(gitignore, originalIgnore, 0o644)
		} else {
			err = os.Remove(gitignore)
		}
		if err != nil {
			rollbackErrors = append(rollbackErrors, ".gitignore: "+err.Error())
		}
	}
	if len(rollbackErrors) > 0 {
		return fmt.Errorf("%s 失敗：%v；回滾也失敗：%s", step, cause, strings.Join(rollbackErrors, "; "))
	}
	return fmt.Errorf("%s 失敗：%w；已回滾 index 與 .gitignore", step, cause)
}
