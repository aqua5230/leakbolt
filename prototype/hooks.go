package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

func hookGuard(versionControlled bool) string {
	guard := "if [ \"$(git config --bool --get hooks.leakbolt)\" = \"false\" ]; then\n" +
		"  echo \"LeakBolt：已由 git config hooks.leakbolt false 停用，這次 commit 未檢查密鑰。\" >&2\n" +
		"elif command -v leakbolt >/dev/null 2>&1; then\n" +
		"  leakbolt scan --staged || exit 1\n"
	if versionControlled {
		return guard +
			"elif [ -f \"$(git rev-parse --git-common-dir)/leakbolt/state.json\" ]; then\n" +
			"  echo \"LeakBolt：找不到 leakbolt 執行檔，commit 已中止。\" >&2\n" +
			"  echo \"  這台機器裝過 LeakBolt，但現在 PATH 上找不到它。\" >&2\n" +
			"  echo \"  PATH=$PATH\" >&2\n" +
			"  echo \"  把 leakbolt 放回 PATH，或移除專案 hook 設定裡的 LeakBolt 區塊來停用檢查。\" >&2\n" +
			"  echo \"  這次要跳過檢查：git commit --no-verify\" >&2\n" +
			"  echo \"  永久停用 LeakBolt：git config hooks.leakbolt false\" >&2\n" +
			"  exit 1\n" +
			"fi\n"
	}
	return guard +
		"else\n" +
		"  echo \"LeakBolt：找不到 leakbolt 執行檔，commit 已中止。\" >&2\n" +
		"  echo \"  這個 repo 裝過 LeakBolt，但現在 PATH 上找不到它。\" >&2\n" +
		"  echo \"  PATH=$PATH\" >&2\n" +
		"  echo \"  把 leakbolt 放回 PATH，或刪掉 .git/hooks/pre-commit 停用檢查。\" >&2\n" +
		"  echo \"  這次要跳過檢查：git commit --no-verify\" >&2\n" +
		"  echo \"  永久停用 LeakBolt：git config hooks.leakbolt false\" >&2\n" +
		"  exit 1\n" +
		"fi\n"
}

type hookTarget struct {
	Path              string
	Kind              string
	VersionControlled bool
}

type installResult struct {
	Path              string
	Kind              string
	VersionControlled bool
}

type HooksPathOccupiedError struct {
	Scope string
	Value string
}

func (e *HooksPathOccupiedError) Error() string { return "core.hooksPath 已被設定" }

func detectHookTarget(repo string, localOnly bool) (hookTarget, error) {
	if localOnly {
		return hookTarget{Path: filepath.Join(repo, ".git", "hooks", "pre-commit"), Kind: "script"}, nil
	}

	if isDir(filepath.Join(repo, ".husky")) {
		return hookTarget{Path: filepath.Join(repo, ".husky", "pre-commit"), Kind: "script", VersionControlled: true}, nil
	}
	for _, name := range []string{"lefthook.yml", "lefthook.yaml"} {
		path := filepath.Join(repo, name)
		if isFile(path) {
			return hookTarget{Path: path, Kind: "lefthook", VersionControlled: true}, nil
		}
	}
	path := filepath.Join(repo, ".pre-commit-config.yaml")
	if isFile(path) {
		return hookTarget{Path: path, Kind: "precommit", VersionControlled: true}, nil
	}
	return hookTarget{Path: filepath.Join(repo, ".git", "hooks", "pre-commit"), Kind: "script"}, nil
}

// husky v9 安裝時必定會設 repo 層級的 core.hooksPath 為 .husky/_，
// 那是我們認得的情況，要照常寫進 .husky/pre-commit，不能當成佔用而拒裝。
func isHuskyHooksPath(repo, value string) bool {
	cleaned := strings.TrimSuffix(filepath.Clean(value), string(filepath.Separator))
	if cleaned != filepath.Join(".husky", "_") && cleaned != filepath.Join(repo, ".husky", "_") {
		return false
	}
	return isDir(filepath.Join(repo, ".husky"))
}

func installHook(repo string, localOnly bool) (installResult, error) {
	for _, scope := range []string{"local", "global", "system"} {
		value, set, err := gitConfig(repo, scope)
		if err != nil {
			if scope == "system" {
				continue
			}
			return installResult{}, err
		}
		if !set {
			continue
		}
		if isHuskyHooksPath(repo, value) {
			continue
		}
		return installResult{}, &HooksPathOccupiedError{Scope: scope, Value: value}
	}

	target, err := detectHookTarget(repo, localOnly)
	if err != nil {
		return installResult{}, err
	}
	if err := writeHookTarget(target); err != nil {
		return installResult{}, err
	}
	return installResult{Path: target.Path, Kind: target.Kind, VersionControlled: target.VersionControlled}, nil
}

func writeHookTarget(target hookTarget) error {
	switch target.Kind {
	case "script":
		return writeScriptHook(target.Path, target.VersionControlled)
	case "lefthook":
		return appendLefthook(target.Path, target.VersionControlled)
	case "precommit":
		return appendPreCommitConfig(target.Path, target.VersionControlled)
	default:
		return fmt.Errorf("未知 hook 類型：%s", target.Kind)
	}
}

// writeScriptHook 把我們的檢查插在腳本最前面（shebang 之後），不是追加到尾端。
//
// 追加到尾端會被前面任何一個失敗的指令擋掉：husky 預設的 .husky/pre-commit 內容是 npm test，
// 空專案跑起來必定失敗，腳本就結束了，我們那行根本不會執行——使用者以為裝好了，其實沒在掃。
// 資安檢查要最先跑：擋得住就快點擋，也不被別人的失敗遮蔽。
func writeScriptHook(path string, versionControlled bool) error {
	content, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if strings.Contains(string(content), "leakbolt scan --staged") {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	block := "# Added by LeakBolt\n" + hookGuard(versionControlled)

	existing := string(content)
	var out string
	switch {
	case strings.TrimSpace(existing) == "":
		out = "#!/bin/sh\n" + block
	case strings.HasPrefix(existing, "#!"):
		// 保留原本的 shebang，把我們的區塊插在它後面
		idx := strings.Index(existing, "\n")
		if idx < 0 {
			out = existing + "\n" + block
		} else {
			out = existing[:idx+1] + block + existing[idx+1:]
		}
	default:
		out = "#!/bin/sh\n" + block + existing
	}
	if !strings.HasSuffix(out, "\n") {
		out += "\n"
	}
	if err := os.WriteFile(path, []byte(out), 0o755); err != nil {
		return err
	}
	return os.Chmod(path, 0o755)
}

func appendLefthook(path string, versionControlled bool) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if strings.Contains(string(content), "leakbolt scan --staged") {
		return nil
	}
	text := string(content)
	if strings.Contains(text, "pre-commit: {}") || strings.Contains(text, "pre-commit: {") {
		return fmt.Errorf("不支援 inline 的 lefthook pre-commit 設定；請手動加入 LeakBolt")
	}
	guard := hookGuard(versionControlled)
	entry := "    leakbolt:\n      run: |\n        " + strings.ReplaceAll(strings.TrimSuffix(guard, "\n"), "\n", "\n        ") + "\n"
	start, end, found := yamlTopLevelBlock(text, "pre-commit")
	if found {
		block := text[start:end]
		if index := strings.Index(block, "  commands:\n"); index >= 0 {
			insertAt := start + index + len("  commands:\n")
			text = text[:insertAt] + entry + text[insertAt:]
		} else {
			text = text[:start] + "  commands:\n" + entry + text[start:]
		}
	} else {
		if !strings.HasSuffix(text, "\n") {
			text += "\n"
		}
		text += "\npre-commit:\n  commands:\n" + entry
	}
	return os.WriteFile(path, []byte(text), 0o644)
}

func appendPreCommitConfig(path string, versionControlled bool) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if strings.Contains(string(content), "leakbolt scan --staged") {
		return nil
	}
	text := string(content)
	if strings.Contains(text, "repos: []") {
		text = strings.Replace(text, "repos: []", "repos:", 1)
	}
	if !strings.Contains(text, "repos:") {
		if strings.TrimSpace(text) != "" {
			return fmt.Errorf("找不到 repos:；請先修正 .pre-commit-config.yaml")
		}
		text = "repos:\n"
	}
	if !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	guard := strings.ReplaceAll(strings.TrimSpace(hookGuard(versionControlled)), "\n", "; ")
	guard = strings.ReplaceAll(guard, "then; ", "then ")
	guard = strings.ReplaceAll(guard, "else; ", "else ")
	text += "\n# Added by LeakBolt\n- repo: local\n  hooks:\n    - id: leakbolt\n      name: LeakBolt staged secret scan\n      entry: sh -c '" + guard + "'\n      language: system\n      pass_filenames: false\n"
	return os.WriteFile(path, []byte(text), 0o644)
}

// yamlTopLevelBlock 回傳指定頂層 key 內容的起訖索引；這只處理本原型需要的常見 YAML 形式。
func yamlTopLevelBlock(text, key string) (int, int, bool) {
	position := 0
	lines := strings.SplitAfter(text, "\n")
	for index, line := range lines {
		trimmed := strings.TrimSuffix(line, "\n")
		if trimmed != key+":" {
			position += len(line)
			continue
		}

		start := position + len(line)
		end := len(text)
		cursor := start
		for _, following := range lines[index+1:] {
			plain := strings.TrimSpace(following)
			if plain != "" && !strings.HasPrefix(plain, "#") && following[0] != ' ' && following[0] != '\t' {
				end = cursor
				break
			}
			cursor += len(following)
		}
		return start, end, true
	}
	return 0, 0, false
}

func appendFile(path string, data []byte, perm os.FileMode) error {
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, perm)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = file.Write(data)
	return err
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func isFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// effectiveHookPath 回傳 git 實際會去執行的 pre-commit 路徑（尊重 core.hooksPath）。
func effectiveHookPath(repo string) string {
	for _, scope := range []string{"local", "global", "system"} {
		if value, set, err := gitConfig(repo, scope); err == nil && set {
			if filepath.IsAbs(value) {
				return filepath.Join(value, "pre-commit")
			}
			return filepath.Join(repo, value, "pre-commit")
		}
	}
	return filepath.Join(repo, ".git", "hooks", "pre-commit")
}

// verifyHookReachable 檢查設定寫進去之後，git 真的找得到一個可執行的 pre-commit。
// 只寫進 lefthook.yml 或 .pre-commit-config.yaml 而使用者從沒跑過該工具的 install，
// 會造成「設定在、hook 不存在」——裝了卻沒有保護，這時候必須警告，不能回報成功。
func verifyHookReachable(repo string) (string, bool) {
	path := effectiveHookPath(repo)
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return path, false
	}
	// Windows 上 Go 永遠不會回報執行位元（os.Stat 只給 0666 或 0444），
	// 拿 0o111 去判斷會讓每一次 Windows 安裝都跳假警報說 hook 不可執行。
	// Git for Windows 靠副檔名與 Git Bash 決定能不能跑，不看 POSIX 權限位元，
	// 所以那邊只確認檔案存在。
	if runtime.GOOS == "windows" {
		return path, true
	}
	return path, info.Mode()&0o111 != 0
}
