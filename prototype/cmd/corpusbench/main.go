package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

const (
	expectDetect = "detect"
	expectClean  = "clean"
)

type manifest struct {
	Samples []sample `json:"samples"`
}

type sample struct {
	Path   string `json:"path"`
	Expect string `json:"expect"`
	Note   string `json:"note"`
}

type finding struct {
	RuleID string `json:"RuleID"`
}

type result struct {
	Sample   sample
	Findings []finding
}

type metrics struct {
	TruePositive  int
	FalseNegative int
	TrueNegative  int
	FalsePositive int
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("corpusbench", flag.ContinueOnError)
	flags.SetOutput(stderr)
	corpusDir := flags.String("corpus", "corpus", "語料庫目錄")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 {
		if err == nil {
			fmt.Fprintln(stderr, "corpusbench 不接受位置參數")
		}
		return 2
	}

	gitleaks, err := exec.LookPath("gitleaks")
	if err != nil {
		fmt.Fprintln(stderr, "找不到 gitleaks。請安裝 gitleaks v8.30.1 後重試：https://github.com/gitleaks/gitleaks")
		return 2
	}

	corpus, err := filepath.Abs(*corpusDir)
	if err != nil {
		fmt.Fprintf(stderr, "解析語料庫路徑失敗：%v\n", err)
		return 2
	}
	manifest, err := loadManifest(corpus)
	if err != nil {
		fmt.Fprintf(stderr, "讀取 manifest 失敗：%v\n", err)
		return 2
	}

	results := make([]result, 0, len(manifest.Samples))
	for _, entry := range manifest.Samples {
		path, err := corpusSamplePath(corpus, entry.Path)
		if err != nil {
			fmt.Fprintf(stderr, "manifest 無效：%v\n", err)
			return 2
		}
		findings, err := scanFile(gitleaks, path)
		if err != nil {
			fmt.Fprintf(stderr, "掃描 %s 失敗：%v\n", entry.Path, err)
			return 2
		}
		results = append(results, result{Sample: entry, Findings: findings})
	}

	printReport(stdout, results)
	return 0
}

func loadManifest(corpus string) (manifest, error) {
	path := filepath.Join(corpus, "manifest.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return manifest{}, err
	}
	var value manifest
	if err := json.Unmarshal(data, &value); err != nil {
		return manifest{}, err
	}
	if len(value.Samples) == 0 {
		return manifest{}, fmt.Errorf("samples 不可空白")
	}
	for _, sample := range value.Samples {
		if sample.Path == "" {
			return manifest{}, fmt.Errorf("sample path 不可空白")
		}
		if sample.Expect != expectDetect && sample.Expect != expectClean {
			return manifest{}, fmt.Errorf("%s 的 expect 必須是 %q 或 %q", sample.Path, expectDetect, expectClean)
		}
	}
	return value, nil
}

func corpusSamplePath(corpus, relative string) (string, error) {
	if filepath.IsAbs(relative) {
		return "", fmt.Errorf("%s 不可使用絕對路徑", relative)
	}
	path := filepath.Join(corpus, relative)
	contained, err := filepath.Rel(corpus, path)
	if err != nil || contained == ".." || strings.HasPrefix(contained, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("%s 必須在 corpus 內", relative)
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("%s：%w", relative, err)
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("%s 必須是一般檔案", relative)
	}
	return path, nil
}

func scanFile(gitleaks, path string) ([]finding, error) {
	// 一定要帶上跟正式掃描同一份補充規則，否則量到的不是產品實際行為。
	args := []string{"detect", "--no-git", "--source", path, "--report-format", "json", "--report-path", "-"}
	if configPath, cleanup, ok := writeSupplementaryRules(); ok {
		defer cleanup()
		args = append([]string{"detect", "--config", configPath}, args[1:]...)
	}
	cmd := exec.Command(gitleaks, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	runErr := cmd.Run()

	var findings []finding
	data := bytes.TrimSpace(stdout.Bytes())
	if len(data) > 0 {
		if err := json.Unmarshal(data, &findings); err != nil {
			return nil, fmt.Errorf("無法解析 gitleaks JSON 報告：%w", err)
		}
	}
	if runErr == nil {
		return findings, nil
	}
	var exitErr *exec.ExitError
	if errors.As(runErr, &exitErr) && exitErr.ExitCode() == 1 && len(findings) > 0 {
		return findings, nil
	}
	message := strings.TrimSpace(stderr.String())
	if message == "" {
		message = runErr.Error()
	}
	return nil, fmt.Errorf("gitleaks 執行失敗：%s", message)
}

func printReport(stdout io.Writer, results []result) {
	value := metrics{}
	var incorrect []result
	for _, result := range results {
		detected := len(result.Findings) > 0
		switch {
		case result.Sample.Expect == expectDetect && detected:
			value.TruePositive++
		case result.Sample.Expect == expectDetect:
			value.FalseNegative++
			incorrect = append(incorrect, result)
		case detected:
			value.FalsePositive++
			incorrect = append(incorrect, result)
		default:
			value.TrueNegative++
		}
	}

	missRate := rate(value.FalseNegative, value.TruePositive+value.FalseNegative)
	falsePositiveRate := rate(value.FalsePositive, value.FalsePositive+value.TrueNegative)
	fmt.Fprintf(stdout, "真陽性：%d\n", value.TruePositive)
	fmt.Fprintf(stdout, "假陰性：%d\n", value.FalseNegative)
	fmt.Fprintf(stdout, "真陰性：%d\n", value.TrueNegative)
	fmt.Fprintf(stdout, "假陽性：%d\n", value.FalsePositive)
	fmt.Fprintf(stdout, "漏報率：%.2f%%\n", missRate*100)
	fmt.Fprintf(stdout, "誤報率：%.2f%%\n", falsePositiveRate*100)
	fmt.Fprintf(stdout, "CORPUSBENCH_MISS_RATE=%.6f\n", missRate)
	fmt.Fprintf(stdout, "CORPUSBENCH_FALSE_POSITIVE_RATE=%.6f\n", falsePositiveRate)

	if len(incorrect) == 0 {
		return
	}
	fmt.Fprintln(stdout, "判斷錯誤：")
	for _, result := range incorrect {
		actual := expectClean
		if len(result.Findings) > 0 {
			actual = expectDetect
		}
		fmt.Fprintf(stdout, "- %s｜預期 %s，實際 %s｜規則 %s\n", result.Sample.Path, result.Sample.Expect, actual, ruleIDs(result.Findings))
	}
}

func rate(numerator, denominator int) float64 {
	if denominator == 0 {
		return 0
	}
	return float64(numerator) / float64(denominator)
}

func ruleIDs(findings []finding) string {
	if len(findings) == 0 {
		return "（無命中）"
	}
	set := make(map[string]bool)
	for _, finding := range findings {
		if finding.RuleID != "" {
			set[finding.RuleID] = true
		}
	}
	if len(set) == 0 {
		return "（無規則 ID）"
	}
	ids := make([]string, 0, len(set))
	for id := range set {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return strings.Join(ids, ", ")
}
