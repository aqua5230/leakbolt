package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"sort"
	"strings"
)

const requiredGitleaksVersion = "8.30.1"

const bundledGitleaksPath = "/usr/local/leakbolt/bin/gitleaks"

type Finding struct {
	RuleID string `json:"RuleID"`
	File   string `json:"File"`
	Commit string `json:"Commit"`
	Secret string `json:"Secret"`
}

type stagedFinding struct {
	Finding
	Fingerprint string
}

type MissingGitleaksError struct{}

func (e *MissingGitleaksError) Error() string {
	return "找不到 gitleaks。請安裝 gitleaks v8.30.1 後重試：https://github.com/gitleaks/gitleaks"
}

func scanStaged(repo string, stderr io.Writer) ([]Finding, error) {
	return runGitleaks(repo, stderr, "protect", "--staged", "--report-format", "json", "--report-path", "-")
}

func scanHistory(repo string, stderr io.Writer) ([]Finding, error) {
	return runGitleaks(repo, stderr, "detect", "--report-format", "json", "--report-path", "-")
}

func runGitleaks(repo string, stderr io.Writer, args ...string) ([]Finding, error) {
	path, version, versionErr := gitleaksPathAndVersion()
	if path == "" {
		return nil, versionErr
	}
	if versionErr != nil {
		fmt.Fprintf(stderr, "警告：無法確認 gitleaks 版本（%v）。偵測結果可能與品質基準不同。\n", versionErr)
	} else if version != requiredGitleaksVersion {
		fmt.Fprintf(stderr, "警告：gitleaks 版本為 %s，LeakBolt 鎖定的是 %s。偵測結果可能與品質基準不同。\n", version, requiredGitleaksVersion)
	}

	if configPath, cleanup, rulesErr := writeSupplementaryRules(); rulesErr == nil {
		defer cleanup()
		args = append([]string{args[0], "--config", configPath}, args[1:]...)
	} else {
		fmt.Fprintf(stderr, "警告：無法寫入補充規則暫存檔（%v），這次掃描只用 gitleaks 預設規則，未涵蓋 LeakBolt 自帶規則。\n", rulesErr)
	}

	cmd := exec.Command(path, args...)
	cmd.Dir = repo
	var stdout, commandStderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &commandStderr
	runErr := cmd.Run()
	if len(bytes.TrimSpace(stdout.Bytes())) == 0 {
		message := strings.TrimSpace(commandStderr.String())
		if message == "" && runErr != nil {
			message = runErr.Error()
		}
		if message == "" {
			message = "gitleaks 未輸出 JSON 報告"
		}
		return nil, fmt.Errorf("gitleaks 執行失敗：%s", message)
	}
	findings, parseErr := parseFindings(stdout.Bytes())
	if parseErr == nil {
		return findings, nil
	}
	return nil, parseErr
}

func gitleaksPathAndVersion() (string, string, error) {
	path, err := findGitleaksPath(bundledGitleaksPath)
	if err != nil {
		return "", "", &MissingGitleaksError{}
	}
	output, err := exec.Command(path, "version").Output()
	if err != nil {
		return path, "", fmt.Errorf("執行 gitleaks version 失敗：%w", err)
	}
	version := strings.TrimSpace(string(output))
	version = strings.TrimPrefix(version, "v")
	if version == "" {
		return path, "", fmt.Errorf("gitleaks version 沒有輸出版本字串")
	}
	return path, version, nil
}

func findGitleaksPath(bundledPath string) (string, error) {
	if path, err := exec.LookPath(bundledPath); err == nil {
		return path, nil
	}
	return exec.LookPath("gitleaks")
}

func parseFindings(data []byte) ([]Finding, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return nil, nil
	}
	var findings []Finding
	if err := json.Unmarshal(trimmed, &findings); err != nil {
		return nil, fmt.Errorf("無法解析 gitleaks JSON 報告：%w", err)
	}
	return findings, nil
}

func prepareStagedFindings(state *State, findings []Finding) ([]stagedFinding, error) {
	recent := make([]RecentFinding, 0, len(findings))
	seenRecent := make(map[string]bool)
	visible := make([]stagedFinding, 0, len(findings))
	for _, finding := range findings {
		value, err := fingerprint(state, finding.RuleID, finding.Secret)
		if err != nil {
			return nil, err
		}
		if !seenRecent[value] {
			recent = append(recent, RecentFinding{Fingerprint: value, RuleID: finding.RuleID})
			seenRecent[value] = true
		}
		if !hasAllowedFingerprint(state, value) {
			visible = append(visible, stagedFinding{Finding: finding, Fingerprint: value})
		}
	}
	state.Recent = recent
	return visible, nil
}

func printStagedSummary(w io.Writer, findings []stagedFinding, total int) {
	fmt.Fprintf(w, "暫存區掃描完成：找到 %d 筆，未標記 %d 筆。\n", total, len(findings))
	if len(findings) == 0 {
		return
	}

	copyFindings := append([]stagedFinding(nil), findings...)
	sort.Slice(copyFindings, func(i, j int) bool {
		if copyFindings[i].File != copyFindings[j].File {
			return copyFindings[i].File < copyFindings[j].File
		}
		return copyFindings[i].RuleID < copyFindings[j].RuleID
	})
	for _, finding := range copyFindings {
		file := finding.File
		if file == "" {
			file = "（無檔案資訊）"
		}
		rule := finding.RuleID
		if rule == "" {
			rule = "（無規則 ID）"
		}
		fmt.Fprintf(w, "- 檔案 %s｜規則 %s｜指紋 %s\n", file, rule, shortFingerprint(finding.Fingerprint))
	}
	fmt.Fprintln(w, "commit 已中止。確認是誤報可執行 leakbolt allow <指紋前綴>。")
}

func printHistorySummary(w io.Writer, findings []Finding) {
	fmt.Fprintf(w, "歷史掃描完成：找到 %d 筆。\n", len(findings))
	if len(findings) == 0 {
		return
	}

	copyFindings := append([]Finding(nil), findings...)
	sort.Slice(copyFindings, func(i, j int) bool {
		if copyFindings[i].Commit != copyFindings[j].Commit {
			return copyFindings[i].Commit < copyFindings[j].Commit
		}
		if copyFindings[i].File != copyFindings[j].File {
			return copyFindings[i].File < copyFindings[j].File
		}
		return copyFindings[i].RuleID < copyFindings[j].RuleID
	})
	for _, finding := range copyFindings {
		commit := finding.Commit
		if commit == "" {
			commit = "（無 commit 資訊）"
		}
		file := finding.File
		if file == "" {
			file = "（無檔案資訊）"
		}
		rule := finding.RuleID
		if rule == "" {
			rule = "（無規則 ID）"
		}
		fmt.Fprintf(w, "- commit %s｜檔案 %s｜規則 %s\n", commit, file, rule)
	}
}
