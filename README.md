# LeakBolt

在 git commit 之前自動攔截並防範 API 金鑰與密鑰外洩的命令列工具。

## 這是什麼／解決什麼問題

LeakBolt 是一個命令列工具，幫你在 git commit 之前擋下不小心寫進程式碼的 API 金鑰與密鑰。

它不自己做偵測——偵測引擎用的是 gitleaks。LeakBolt 做的是 gitleaks 沒做的那一層：自動裝好 git hook、檢查 hook 有沒有失效、處理誤報、把誤加進版控的憑證檔移出去。

## 前置需求

使用者要**自己先裝 gitleaks，版本 v8.30.1**。LeakBolt 不會幫你裝。

- macOS 安裝方式：`brew install gitleaks`
- 官方專案頁面：https://github.com/gitleaks/gitleaks

若未安裝 gitleaks，執行 `leakbolt scan` 會回報錯誤並中止：
`找不到 gitleaks。請安裝 gitleaks v8.30.1 後重試：https://github.com/gitleaks/gitleaks`

版本若不是 8.30.1，每次執行會印出警告，但仍會繼續掃描。

## 安裝

目前沒有 Homebrew 或任何套件管理器可以安裝，現行提供兩種管道：

1. **自行編譯**：需要 Go 1.21 或更新版本，執行 `cd prototype && go build -o leakbolt .`，然後將 `leakbolt` 放進 PATH。
2. **預編譯檔**：使用 repo 內 `dist/` 的預編譯檔（目前只有 macOS 與 Windows）。

### macOS Gatekeeper 警告
若執行檔是從網路下載或他人傳送，macOS 會因為缺乏 Apple 簽章而阻擋執行，顯示「Apple 無法驗證…是否為惡意軟體」且沒有「打開」選項。解法二選一：
- 於終端機執行：`xattr -d com.apple.quarantine <檔案路徑>` 移除隔離標記
- 至「系統設定 → 隱私權與安全性」點擊「仍要打開」

自行用 `go build` 編譯的執行檔不會遇到此問題。

## 快速開始

1. **安裝 hook**：在 repo 根目錄執行 `leakbolt install`。
2. **測試 commit**：新增一個包含假金鑰的檔案並加入暫存區，執行 `git commit`。
3. **確認阻擋**：commit 會被 LeakBolt 攔截中斷，終端機顯示偵測到的金鑰警告。

## 指令

```
用法：
  leakbolt install [--local-only]
  leakbolt scan --staged
  leakbolt scan --history
  leakbolt allow <指紋前綴>
  leakbolt purge-backups
  leakbolt untrack <檔案路徑>
  leakbolt doctor
```

版本查詢指令：支援 `leakbolt --version`、`leakbolt -v`、`leakbolt version`，輸出格式如 `leakbolt 0.1.0 (3ba8f46)，鎖定 gitleaks 8.30.1`。

- `install`：在目前 repo 裝上 pre-commit hook，並強制跑一次完整 git 歷史掃描把結果印出來。
- `scan --staged`：掃暫存區（staged，指已 `git add` 但還沒 commit 的內容）。
- `scan --history`：掃完整 git 歷史。
- `allow <指紋前綴>`：把某一筆標成誤報並記住，之後不再擋。
- `untrack <檔案路徑>`：把誤被 git 追蹤的憑證檔移出版控。
- `doctor`：健康檢查，看 hook 是不是還有效。
- `purge-backups`：清掉明文備份。

## 停用

- **單一 repo 停用**：執行 `git config hooks.leakbolt false`。停用期間每次 commit 會印出一行提醒，`leakbolt doctor` 會回報異常並提供恢復指令。
- **單次跳過**：使用 `git commit --no-verify`，但會同時跳過該 repo 的所有其他 hook。

## 偵測範圍

除了 gitleaks 內建規則，LeakBolt 自帶補充規則，涵蓋 gitleaks 沒收但官方有公開格式的供應商：Groq、Replicate、OpenRouter、xAI、Fireworks、DeepSeek、Supabase secret key 與 PAT、Clerk secret key。規則內嵌在執行檔裡，不依賴外部檔案。

同時**刻意排除**設計上就該公開的金鑰以避免誤報：Clerk publishable、Stripe publishable、Supabase publishable 與 anon key。但排除規則不涵蓋「只寫 `NEXT_PUBLIC_`」的情況——把 service_role key 放進 `NEXT_PUBLIC_` 變數本身就是嚴重洩漏，照常抓取。

偵測品質基準語料庫共 40 個樣本（19 個真陽性、21 個假陽性），目前漏報率 0%、誤報率 0%，可用 `sh scripts/quality_gate.sh` 重跑。

## 平台狀態

- **macOS**：相容矩陣 11/11 通過，有預編譯檔。
- **Linux**：相容矩陣 11/11 通過，但**沒有預編譯檔**，要自己編。
- **Windows**：**完全沒有實機驗證過**。`dist/` 裡有 Windows 執行檔，但沒有人在 Windows 上跑過。

## 已知限制

1. **hook 擋不住所有路徑。** `git commit --no-verify`、不經 git 的部署、部分 GUI git 客戶端、hook 被其他工具改寫，都能繞過。請把它當**第一層**防護，不是保證。
2. **把金鑰搬進 `.env` 不等於修好。** 如果那個金鑰曾經進過 git 歷史，就算搬走也必須去供應商那邊作廢重發。另外某些前端框架會把特定前綴的環境變數編進瀏覽器 bundle。
3. **規則會過期。** 鎖定 gitleaks 版本能給穩定基線，但新的供應商與新的 key 格式會繼續出現，不更新就會漏報。
4. **大型 repo 的 commit 延遲還沒量過。**
5. **誤報記錄只存規則 ID 與不可逆指紋**，不存原始命中內容。

## 授權

MIT。詳見 repo 根目錄的 `LICENSE`。

gitleaks 本身也是 MIT，LeakBolt 是以子程序方式呼叫它的執行檔，不是鏈結函式庫。
