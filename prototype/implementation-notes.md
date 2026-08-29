# LeakBolt prototype

## Scope

- Go standard library CLI: `install`, `scan --staged`, `scan --history`, `allow`, `untrack`, `doctor`.
- 外部呼叫系統上的 `gitleaks`，不內嵌掃描引擎。
- 本機 state.json 只存 salt、不可逆命中指紋、規則 ID、日級時間與 hook 狀態摘要。

## Deviations

- 已解決：Go 1.27 現在無需設定 `GO111MODULE`，可直接執行 `go build ./...`、
  `go vet ./...`、`go test ./...`。
- `core.hooksPath` 原文可能是檔案路徑，與 state.json 禁止存路徑衝突；現況以 salt 加碼
  HMAC-SHA256 摘要記錄，doctor 比較摘要，不保存原文。
- 規格未定義「找到 gitleaks，但 `gitleaks version` 執行失敗或輸出空白」；採保守處理：
  scan 警告後繼續，doctor 報異常。這與版本不符時的既定降級邊界一致。
