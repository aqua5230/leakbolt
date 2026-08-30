# Fix report 4

## 結果

FAIL：此沙盒禁止寫入預設 Go build cache，故指定的無環境變數 Go 驗收指令無法通過。
以 `/private/tmp/leakbolt-go-cache` 驗證時，建置、vet 與測試全數通過；程式修復、release 建置、品質閘門與 CLI 驗證皆通過。

## 變更

- `prototype/scan.go`：stdout 去除空白後為空時，直接以 gitleaks stderr（無 stderr 時以執行錯誤）回傳掃描失敗；有 JSON 時即使 exit 1 仍照常解析。
- `prototype/scan_test.go`：新增三個以暫存目錄假 gitleaks 執行檔驅動的真實 `exec` 路徑測試。
- `prototype/version.go`：新增 `0.1.0`、`runtime/debug.ReadBuildInfo()` 的 revision／dirty 版本字串。
- `prototype/main.go`：支援 `--version`、`-v`、`version`。
- `prototype/doctor.go`：第一行輸出相同版本字串。
- `prototype/implementation-notes.md`：更新 `GO111MODULE=off` 已不需要的註記。
- `scripts/release_darwin.sh`：建置 darwin amd64、arm64，使用 `CGO_ENABLED=0`、`-buildvcs=true`、`-trimpath`，產生 SHA256。
- `dist/SHA256SUMS-darwin.txt`：本次 Darwin 產物的 SHA256。
- `dist/leakbolt-darwin-amd64`、`dist/leakbolt-darwin-arm64`：已重新產生，受 `.gitignore` 忽略。

## 先行檢查

```text
$ grep -rn "parseFindings\|runGitleaks" prototype/ && ls prototype/*_test.go
prototype/scan.go:34:    return runGitleaks(repo, stderr, "protect", "--staged", "--report-format", "json", "--report-path", "-")
prototype/scan.go:38:    return runGitleaks(repo, stderr, "detect", "--report-format", "json", "--report-path", "-")
prototype/scan.go:41:func runGitleaks(repo string, stderr io.Writer, args ...string) ([]Finding, error) {
prototype/scan.go:65:    findings, parseErr := parseFindings(stdout.Bytes())
prototype/scan.go:96:func parseFindings(data []byte) ([]Finding, error) {
prototype/scan_test.go:15:  if _, err := runGitleaks(repo, &stderr, "detect", "--report-format", "json", "--report-path", "-"); err != nil {
prototype/scan_test.go:33:  if _, err := runGitleaks(repo, &stderr, "detect", "--report-format", "json", "--report-path", "-"); err != nil {
prototype/hooks_test.go
prototype/scan_test.go
prototype/state_features_test.go
exit=0
```

`cmd/corpusbench` 不呼叫 `runGitleaks`，未修改。

## 驗收

### 1. fail-open 重現

在乾淨暫存 git repo 中，stage `secret.txt`（內容為 `AKIAIMNOJVGFDXXXE4OA`），PATH 前置假 gitleaks 後：

```text
$ PATH=/tmp/lb-fakebin:$PATH leakbolt scan --staged
警告：gitleaks 版本為 8.31.0，LeakBolt 鎖定的是 8.30.1。偵測結果可能與品質基準不同。
掃描失敗：gitleaks 執行失敗：Error: unknown command
scan_exit=2
```

符合要求：掃描被擋下，錯誤含 gitleaks stderr，非 0 exit。

### 2. Go 建置、vet、測試

指定的原始命令因沙盒限制失敗，均為同一個預設 Go cache 寫入權限問題，已達三次停損條件，未再重試：

```text
$ cd prototype && go build ./...
open /Users/lollapalooza/Library/Caches/go-build/f1/f1a5ca3b270ad8b31f06e93f3be347d74b4b91b61e34eb0b1b6773b8e1c24885-d: operation not permitted
exit=1

$ cd prototype && go vet ./...
open /Users/lollapalooza/Library/Caches/go-build/f1/f1a5ca3b270ad8b31f06e93f3be347d74b4b91b61e34eb0b1b6773b8e1c24885-d: operation not permitted
exit=1

$ cd prototype && go test ./...
FAIL    leakbolt [setup failed]
# leakbolt
open /Users/lollapalooza/Library/Caches/go-build/f1/f1a5ca3b270ad8b31f06e93f3be347d74b4b91b61e34eb0b1b6773b8e1c24885-d: operation not permitted
?       leakbolt/cmd/corpusbench    [no test files]
FAIL
exit=1
```

使用可寫的暫存 Go cache 驗證：

```text
$ cd prototype && GOCACHE=/private/tmp/leakbolt-go-cache go build ./...
exit=0
(無輸出)

$ cd prototype && GOCACHE=/private/tmp/leakbolt-go-cache go vet ./...
exit=0
(無輸出)

$ cd prototype && GOCACHE=/private/tmp/leakbolt-go-cache go test ./...
ok      leakbolt        (cached)
?       leakbolt/cmd/corpusbench    [no test files]
exit=0
```

### 3. 品質閘門

```text
$ sh scripts/quality_gate.sh
真陽性：19
假陰性：0
真陰性：21
假陽性：0
漏報率：0.00%
誤報率：0.00%
CORPUSBENCH_MISS_RATE=0.000000
CORPUSBENCH_FALSE_POSITIVE_RATE=0.000000
quality gate 通過：漏報率 0.000000，誤報率 0.000000
exit=0
```

### 4. 版本與 doctor

```text
$ dist/leakbolt-darwin-arm64 --version
leakbolt 0.1.0 (86143b6-dirty)，鎖定 gitleaks 8.30.1
exit=0

$ dist/leakbolt-darwin-arm64 -v
leakbolt 0.1.0 (86143b6-dirty)，鎖定 gitleaks 8.30.1
exit=0

$ dist/leakbolt-darwin-arm64 version
leakbolt 0.1.0 (86143b6-dirty)，鎖定 gitleaks 8.30.1
exit=0

$ dist/leakbolt-darwin-arm64 doctor
leakbolt 0.1.0 (86143b6-dirty)，鎖定 gitleaks 8.30.1
安裝記錄：異常（找不到 state.json）
建議：leakbolt install
exit=1
```

doctor 的 exit 1 是既有「找不到 state.json」邏輯；新增版本行為第一行，其他輸出與 exit 邏輯未變。

### 5. Darwin release

```text
$ sh scripts/release_darwin.sh
exit=0

$ file dist/leakbolt-darwin-amd64 dist/leakbolt-darwin-arm64
dist/leakbolt-darwin-amd64: Mach-O 64-bit executable x86_64
dist/leakbolt-darwin-arm64: Mach-O 64-bit executable arm64
exit=0

$ dist/leakbolt-darwin-arm64 --version
leakbolt 0.1.0 (86143b6-dirty)，鎖定 gitleaks 8.30.1
exit=0

$ cat dist/SHA256SUMS-darwin.txt
76ad24429b5e7efe0d9ed171e4533a03ecf8683b50be60c46da961e11c7d474c  leakbolt-darwin-amd64
e14e32e0b0fc01890d98c4146bd255316113af98e82641989d4c4d33c172f81c  leakbolt-darwin-arm64
exit=0
```

第二次執行 release 後，checksum 檔案 `cmp` 相同：

```text
$ sh scripts/release_darwin.sh && cmp -s <第一次 checksum> dist/SHA256SUMS-darwin.txt
reproducible_exit=0
```

### 6. 變異測試

暫時拿掉 `runGitleaks` 的空 stdout 判斷，並改為忽略 `cmd.Run()` 回傳值後：

```text
$ cd prototype && GOCACHE=/private/tmp/leakbolt-go-cache go test -run '^TestRunGitleaksFailsWhenStdoutIsEmpty$'
--- FAIL: TestRunGitleaksFailsWhenStdoutIsEmpty (0.37s)
    scan_test.go:47: stdout 為空時應回傳錯誤
FAIL
exit status 1
FAIL    leakbolt        0.886s
exit=1
```

已立即還原空 stdout 判斷；完整測試在上方暫存快取驗證中通過。

## 自我審查

### go vet

原始 `go vet ./...` 輸出如驗收第 2 項，因沙盒預設快取權限 exit 1。以暫存快取執行 `go vet ./...`，exit 0、無輸出。

### git diff --stat

```text
 prototype/doctor.go               |  1 +
 prototype/implementation-notes.md |  4 +--
 prototype/main.go                 |  4 ++++
 prototype/scan.go                 | 15 ++++++----
 prototype/scan_test.go            | 58 +++++++++++++++++++++++++++++++++++++++
 5 files changed, 74 insertions(+), 8 deletions(-)
```

這份統計不含未追蹤檔。每個已修改檔案皆如「變更」所列，均直接對應本次需求；既有測試只新增三個必要測試與共用假執行檔 helper，沒有改既有斷言。

### 禁區

- `PLAN.md`、`DECISION.md`、`research/`、`health-summary.md`：未碰。
- `prototype/rules/leakbolt.toml`、`prototype/cmd/corpusbench/rules/leakbolt.toml`、`prototype/corpus/`：未碰。
- `scripts/quality_gate.sh`、`scripts/hook_matrix.sh`、`scripts/linux_matrix.sh`、`scripts/windows_matrix.ps1`、`scripts/mine_issues.sh`：未碰。
- `dist/*.exe`、`dist/WINDOWS-測試說明.md`：未碰。
- `prototype/hooks.go` 的 hook 字串：未碰。
- `fix-report-4.md`：現況違反禁區的 `fix-report*.md` 規則，但這是本任務後段明確要求寫入的唯一檔案。

### HEAD 與工作樹

```text
$ git log --oneline -1
86143b6 依業界慣例補上停用開關與 PATH 診斷訊息
exit=0
```

未執行 `git add`、`git commit`、`git checkout` 或 `git stash`。

預期工作樹檔案：`prototype/doctor.go`、`prototype/implementation-notes.md`、`prototype/main.go`、`prototype/scan.go`、`prototype/scan_test.go`、`prototype/version.go`、`scripts/release_darwin.sh`、`dist/SHA256SUMS-darwin.txt`、`fix-report-4.md`；Darwin binary 受 `.gitignore` 忽略。

## 最終判定

FAIL：唯一未通過項目是此沙盒下無環境變數的 Go build/vet/test，原因為預設快取目錄的作業系統權限；所有以可寫暫存快取進行的程式驗證與其他驗收均通過。
