# LeakBolt prototype

## Scope

- Go standard library CLI: `install [--local-only | --global]`, `uninstall --global`, `scan --staged`, `scan --history`, `allow`, `untrack`, `doctor`.
- 外部呼叫系統上的 `gitleaks`，不內嵌掃描引擎。
- 本機 state.json 只存 salt、不可逆命中指紋、規則 ID、日級時間與 hook 狀態摘要。

## Deviations

- macOS 視窗提示未改既有函式簽名；僅把 staged 掃描收尾抽成
  `finishStagedScan`，讓測試能直接證明 `osascript` 失敗時仍回傳既有 exit code 1。
- 規格未定義 `git config --bool --get leakbolt.gui` 本身失敗時的處理；採保守選項，
  靜默略過視窗，不影響掃描結果與 commit 成敗。
- 無圖形介面探測使用 AppleScriptObjC 讀取 AppKit 的前景程式，不顯示視窗；
  2 秒內失敗或取不到前景程式即靜默略過。
- 本次全域保護實作未偏離指定設計；既有單一 repo 安裝流程與函式簽名保持不變。
- `gofmt -l .` 會列出既有的 `corpus/falsepositive/fixture_test.go` 與
  `corpus/truepositive/main.go`；兩者是本次任務外的掃描樣本，依「不碰無關檔案」保留。
  本次新增與修改的 Go 檔案皆無 `gofmt` 輸出。

- Go 1.27 可直接執行 `go build ./...`、`go vet ./...`、`go test ./...`，
  不需設定 `GO111MODULE=off`。
- `core.hooksPath` 原文可能是檔案路徑，與 state.json 禁止存路徑衝突；現況以 salt 加碼
  HMAC-SHA256 摘要記錄，doctor 比較摘要，不保存原文。
- 規格未定義「找到 gitleaks，但 `gitleaks version` 執行失敗或輸出空白」；採保守處理：
  scan 警告後繼續，doctor 報異常。這與版本不符時的既定降級邊界一致。
- `.pkg` 的 postinstall 透過 `/dev/console` 取得目前登入帳號，再以該帳號的 UID、家目錄
  與登入工作階段執行 `leakbolt install --global`；失敗只寫安裝紀錄，不讓套件安裝失敗。
- LeakBolt 先找套件私有位置 `/usr/local/leakbolt/bin/gitleaks`，不存在或不可執行時才沿用
  PATH 查找；既有缺少工具中止與版本警告行為未變。
- 本次環境無法解析 `github.com`，依停損條件未改用假檔案，也未產生 `.pkg`；未完成的
  `pkgutil`、`lipo` 與展開檢查詳列於 `pkg-report.md`。
