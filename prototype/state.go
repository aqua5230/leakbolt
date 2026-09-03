package main

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	stateDirectoryName = "leakbolt"
	stateFileName      = "state.json"
)

// State 永遠不存原始命中、檔案路徑或 hooksPath 原文。
type State struct {
	Salt      string          `json:"salt"`
	Allowlist []AllowEntry    `json:"allowlist,omitempty"`
	Recent    []RecentFinding `json:"recent,omitempty"`
	Install   *InstallState   `json:"install,omitempty"`
}

type AllowEntry struct {
	Fingerprint string `json:"fingerprint"`
	RuleID      string `json:"rule_id"`
	AddedOn     string `json:"added_on"`
}

type RecentFinding struct {
	Fingerprint string `json:"fingerprint"`
	RuleID      string `json:"rule_id"`
}

type InstallState struct {
	InstalledOn           string `json:"installed_on"`
	HookKind              string `json:"hook_kind"`
	LocalHooksPathDigest  string `json:"local_hooks_path_digest"`
	GlobalHooksPathDigest string `json:"global_hooks_path_digest"`
	SystemHooksPathDigest string `json:"system_hooks_path_digest"`
}

func stateDirectory(repo string) string {
	return filepath.Join(gitCommonDir(repo), stateDirectoryName)
}

func statePath(repo string) string {
	return filepath.Join(stateDirectory(repo), stateFileName)
}

func newState() (*State, error) {
	salt := make([]byte, 32)
	if _, err := rand.Read(salt); err != nil {
		return nil, fmt.Errorf("產生本機 salt 失敗：%w", err)
	}
	return &State{Salt: hex.EncodeToString(salt)}, nil
}

func loadState(repo string) (*State, error) {
	data, err := os.ReadFile(statePath(repo))
	if err != nil {
		return nil, err
	}
	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("讀取 state.json 失敗：%w", err)
	}
	if _, err := state.saltBytes(); err != nil {
		return nil, err
	}
	return &state, nil
}

func loadOrCreateState(repo string) (*State, error) {
	state, err := loadState(repo)
	if err == nil {
		return state, nil
	}
	if !os.IsNotExist(err) {
		return nil, err
	}
	return newState()
}

func writeState(repo string, state *State) error {
	if _, err := state.saltBytes(); err != nil {
		return err
	}
	dir := stateDirectory(repo)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	temp, err := os.CreateTemp(dir, ".state-*.tmp")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	if err := temp.Chmod(0o600); err != nil {
		temp.Close()
		return err
	}
	if _, err := temp.Write(data); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tempPath, statePath(repo)); err != nil {
		return err
	}
	return os.Chmod(statePath(repo), 0o600)
}

func (s *State) saltBytes() ([]byte, error) {
	salt, err := hex.DecodeString(s.Salt)
	if err != nil || len(salt) != 32 {
		return nil, fmt.Errorf("state.json 的 salt 無效")
	}
	return salt, nil
}

func fingerprint(state *State, ruleID, secret string) (string, error) {
	salt, err := state.saltBytes()
	if err != nil {
		return "", err
	}
	mac := hmac.New(sha256.New, salt)
	_, _ = mac.Write([]byte(ruleID))
	_, _ = mac.Write([]byte{0})
	_, _ = mac.Write([]byte(secret))
	return hex.EncodeToString(mac.Sum(nil)), nil
}

func hookPathDigest(state *State, scope, value string, set bool) (string, error) {
	salt, err := state.saltBytes()
	if err != nil {
		return "", err
	}
	mac := hmac.New(sha256.New, salt)
	_, _ = mac.Write([]byte("hooksPath\x00" + scope + "\x00"))
	if set {
		_, _ = mac.Write([]byte("set\x00" + value))
	} else {
		_, _ = mac.Write([]byte("unset"))
	}
	return hex.EncodeToString(mac.Sum(nil)), nil
}

func dateToday() string {
	return time.Now().Format("2006-01-02")
}

func shortFingerprint(value string) string {
	if len(value) <= 12 {
		return value
	}
	return value[:12]
}

func hasAllowedFingerprint(state *State, value string) bool {
	for _, entry := range state.Allowlist {
		if entry.Fingerprint == value {
			return true
		}
	}
	return false
}

// lookupRecentFingerprint 找出最近一次 staged 掃描裡符合前綴的命中，唯讀，
// 不寫入 allowlist——給 guide 這類只需要「這是哪個規則」而不需要標記誤報的指令用。
func lookupRecentFingerprint(repo, prefix string) (RecentFinding, error) {
	prefix = strings.ToLower(prefix)
	if len(prefix) < 8 {
		return RecentFinding{}, fmt.Errorf("指紋前綴至少需要 8 個字元")
	}
	state, err := loadState(repo)
	if err != nil {
		if os.IsNotExist(err) {
			return RecentFinding{}, fmt.Errorf("尚無最近一次 staged 掃描；請先執行 leakbolt scan --staged")
		}
		return RecentFinding{}, err
	}
	var matches []RecentFinding
	for _, recent := range state.Recent {
		if strings.HasPrefix(recent.Fingerprint, prefix) {
			matches = append(matches, recent)
		}
	}
	if len(matches) == 0 {
		return RecentFinding{}, fmt.Errorf("最近一次 staged 掃描找不到這個指紋前綴")
	}
	if len(matches) > 1 {
		return RecentFinding{}, fmt.Errorf("指紋前綴比對到 %d 筆；請提供更長的前綴", len(matches))
	}
	return matches[0], nil
}

func allowRecentFingerprint(repo, prefix string) (AllowEntry, bool, error) {
	prefix = strings.ToLower(prefix)
	if len(prefix) < 8 {
		return AllowEntry{}, false, fmt.Errorf("指紋前綴至少需要 8 個字元")
	}
	state, err := loadState(repo)
	if err != nil {
		if os.IsNotExist(err) {
			return AllowEntry{}, false, fmt.Errorf("尚無最近一次 staged 掃描；請先執行 leakbolt scan --staged")
		}
		return AllowEntry{}, false, err
	}
	var matches []RecentFinding
	for _, recent := range state.Recent {
		if strings.HasPrefix(recent.Fingerprint, prefix) {
			matches = append(matches, recent)
		}
	}
	if len(matches) == 0 {
		return AllowEntry{}, false, fmt.Errorf("最近一次 staged 掃描找不到這個指紋前綴")
	}
	if len(matches) > 1 {
		return AllowEntry{}, false, fmt.Errorf("指紋前綴比對到 %d 筆；請提供更長的前綴", len(matches))
	}
	match := matches[0]
	if hasAllowedFingerprint(state, match.Fingerprint) {
		return AllowEntry{Fingerprint: match.Fingerprint, RuleID: match.RuleID}, true, nil
	}
	entry := AllowEntry{Fingerprint: match.Fingerprint, RuleID: match.RuleID, AddedOn: dateToday()}
	state.Allowlist = append(state.Allowlist, entry)
	filtered := state.Recent[:0]
	for _, recent := range state.Recent {
		if recent.Fingerprint != match.Fingerprint {
			filtered = append(filtered, recent)
		}
	}
	state.Recent = filtered
	if err := writeState(repo, state); err != nil {
		return AllowEntry{}, false, err
	}
	return entry, false, nil
}
