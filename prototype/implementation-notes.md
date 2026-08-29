# LeakBolt prototype

## Scope

- Go standard library CLI: `install`, `scan --staged`, `scan --history`, `allow`, `untrack`, `doctor`.
- 外部呼叫系統上的 `gitleaks`，不內嵌掃描引擎。
- 本機 state.json 只存 salt、不可逆命中指紋、規則 ID、日級時間與 hook 狀態摘要。

## Deviations

- 現況的 Go 1.27 模組模式無法辨識多個標準庫套件（包含 `crypto/rand`）；
  `GO111MODULE=off go build ./...`、`GO111MODULE=off go vet ./...`、
  `GO111MODULE=off go test ./...` 已通過。未能讓無環境變數的三個指令通過。
- `core.hooksPath` 原文可能是檔案路徑，與 state.json 禁止存路徑衝突；現況以 salt 加碼
  HMAC-SHA256 摘要記錄，doctor 比較摘要，不保存原文。
