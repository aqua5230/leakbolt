# Gitleaks 與 Secret 掃描生態調查報告

## 1. gitleaks 官方現況

gitleaks 官方由 Zachary Rice 創立並維護，目前官方核心 repository 已正式宣告「功能凍結（feature complete）」，未來僅提供安全性修補，開發重心轉向新專案 Betterleaks。官方提供內容與缺漏清單如下：

| 名稱 | URL | stars | 最後更新 | 做什麼 | 缺什麼 |
| :--- | :--- | :--- | :--- | :--- | :--- |
| gitleaks | https://github.com/gitleaks/gitleaks | 29004 | 2026-08-26 | 官方 Go 語言 CLI 工具，支援掃描 Git 歷史、暫存區（staged）、檔案系統與 stdin，並內建 pre-commit hook 配置。 | 官方已宣告功能凍結；無官方 GUI、無官方 MCP server，僅具備靜態正規表達式與熵值檢測，缺乏動態憑證有效性驗證。 |
| gitleaks-action | https://github.com/gitleaks/gitleaks-action | 639 | 2026-07-21 | 官方 GitHub Action，供 CI/CD 流程在 Pull Request 與 Push 時自動執行 gitleaks 掃描並回報結果。 | 僅限 GitHub Actions CI 環境運行，無法在開發者本機編程或 commit 當下提供即時互動與桌面攔截。 |
| betterleaks | https://github.com/betterleaks/betterleaks | 1807 | 2026-08-28 | gitleaks 作者 Zachary Rice 開發的下一代 secret 掃描引擎，旨在提升掃描效能與情境覆蓋率。 | 尚處於初期開發階段，僅有 CLI 引擎，缺乏 GUI、IDE 外掛與 MCP 整合。 |

- **官方 CLI**：提供（Golang 二進位檔、Homebrew、Docker、Release 預編譯包）。
- **官方 GitHub Action**：提供（`gitleaks/gitleaks-action`）。
- **官方 pre-commit hook**：提供（Repository 內建 `.pre-commit-hooks.yaml`，可直接搭配 `pre-commit` 框架執行 `gitleaks protect --staged`）。
- **官方 GUI**：**無**（官方未曾推出任何桌面版、網頁版、選單列或瀏覽器 GUI 工具）。
- **官方 MCP server**：**無**（官方目前未釋出任何 Model Context Protocol 伺服器）。

---

## 2. 第三方包裝（GUI / Desktop / WebUI / 選單列 / 擴充）

| 名稱 | URL | stars | 最後更新 | 做什麼 | 缺什麼 |
| :--- | :--- | :--- | :--- | :--- | :--- |
| gitleaks-visualizer | https://github.com/Abhinandan-Khurana/gitleaks-visualizer | 0 | 2025-05-07 | 純前端 HTML/JS 工具，支援拖放 Gitleaks JSON 報告進行圖形化視覺呈現與篩選。 | 超過一年未更新；只能被動讀取已產生的 JSON 報告，無法主動觸發掃描，無桌面或選單列整合。 |
| clutch-vscode-extension | https://github.com/clutchsecurity/clutch-vscode-extension | 17 | 2025-09-24 | VS Code 擴充套件，在 IDE 內自動掃描開啟的工作區目錄並標註洩漏的 secret。 | 停更將近一年；需外部依賴，無自動修復引導或環境變數轉換流程。 |
| BurpSuite-Gitleaks-Extension | https://github.com/TheArqsz/BurpSuite-Gitleaks-Extension | 5 | 2026-07-31 | Burp Suite 資安外掛，提供 GUI 介面使用 Gitleaks 規則過濾 HTTP 流量與檔案中的機密。 | 僅限資安人員於 Burp Suite 滲透環境使用，非日常開發者適用的本機或 IDE 工具。 |
| gitleaks-ui | https://github.com/WithOps-Com/gitleaks-ui | 0 | 2024-11-06 | 為 Gitleaks 提供簡易 Web 前端操作介面。 | 停更將近兩年，功能極為陽春，無社群維護與自動安裝流程。 |
| openreport | https://github.com/kettu-studio/openreport | 0 | 2026-05-19 | 開源 Web 管理平台，集中呈現並分析 Trivy 與 Gitleaks 產生的安全掃描報告。 | 偏向 CI 報告聚合後台，需自建後端伺服器與資料庫，不適合作為輕量本地開發輔助。 |
| DidILeak | https://github.com/frangelbarrera/DidILeak | 7 | 2026-07-06 | 本地端 LLM 歷史掃描工具與 HTML Dashboard，掃描 ChatGPT、Claude、Cursor 等對話紀錄中的外洩金鑰。 | 屬於事後稽核（post-audit）工具，無法在 commit 當下進行事前攔截。 |
| vaultbix-extension | https://github.com/carlgaopapi-png/vaultbix-extension | 2 | 2026-06-04 | Chrome 擴充套件，在瀏覽器端本地掃描，防止使用者將 API Key 貼入 Web 版 ChatGPT 或 Cursor。 | 僅防護網頁輸入框，無法防護本機終端機、Git 操作或本機 IDE 程式碼。 |
| azure-devops-gitleaks | https://github.com/JoostVoskuil/azure-devops-gitleaks | 43 | 2026-08-22 | Azure DevOps Pipeline 任務擴充套件，在雲端 CI 建置流程中自動下載並執行 Gitleaks。 | 僅適用於 Azure DevOps CI，無任何本機桌面、選單列或 GUI 互動介面。 |
| wasmleaks | https://github.com/zricethezav/wasmleaks | 3 | 2025-03-17 | Gitleaks 原作者 Zachary Rice 的概念驗證專案，將 Gitleaks 編譯為 WASM 於瀏覽器端執行。 | 超過一年未維護，僅為最小 PoC，無完整 UI 與本機檔案系統掃描整合。 |
| leak-lock | https://github.com/nikolareljin/leak-lock | 1 | 2026-08-24 | VS Code 擴充套件，掃描 Git 歷史中的機密、驗證 live 憑證並提供安全重寫（rewrite）流程。 | 剛起步專案（star 數極低），僅支援 VS Code，無獨立桌面或系統選單列形態。 |

---

## 3. 第三方 MCP Server

| 名稱 | URL | stars | 最後更新 | 成熟度 | 做什麼 | 缺什麼 |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| codeinspectus | https://github.com/Synvoya/codeinspectus | 43 | 2026-08-25 | 能用 | Local-first MCP 安全掃描工具，為 Claude Code、Cursor、Codex 提供掃描 → 修復 → 複掃工作流。 | 偏重於 AI 生成程式碼的靜態漏洞檢測，對即時 Pre-commit 攔截與憑證動態驗證支援較弱。 |
| luvv-mcp-server | https://github.com/js2005happy/luvv-mcp-server | 0 | 2026-06-18 | 玩具 | 封裝 Semgrep + Gitleaks 的 MCP Server，透過 stdio 為 Claude Desktop 提供程式碼安全審計工具。 | 個人玩具性質專案，無測試、文檔簡略、無長期維護紀錄。 |
| leakferret | https://github.com/leakferrethq/leakferret | 5 | 2026-07-25 | 能用 | Rust 開發的 MCP 原生 secret 掃描器，內建引擎、CLI 與 MCP server，支援 API key 實體驗證與重寫。 | 專案非常新且社群知名度低（5 stars），缺乏 GUI/IDE 擴充外掛，主要面向 LLM 工具呼叫。 |
| repo-mcp | https://github.com/westkevin12/repo-mcp | 2 | 2026-02-21 | 玩具 | 整合 TruffleHog 與 git-filter-repo 的 MCP Server，供 AI 自動審計與清理 Git 歷史中的機密。 | 半年未更新，功能為單純 subprocess 封裝，缺乏例外管理與錯誤處理。 |
| appsec-galaxy | https://github.com/cparnin/appsec-galaxy | 2 | 2026-08-25 | 玩具 | 學術/實驗性 AppSec 掃描器，整合 Semgrep + Gitleaks + Trivy 並提供 MCP 介面供 LLM 呼叫。 | 架構過於龐大（包含 SBOM/EPSS/SARIF 等），設定繁瑣且非專注於輕量 secret 掃描。 |
| ggmcp | https://github.com/GitGuardian/ggmcp | 37 | 2026-08-25 | 能用 | GitGuardian 官方 MCP Server，透過 API 檢測 600+ 種 secret 並在 LLM 對話中提供修復建議。 | 依賴 GitGuardian 商業 SaaS 服務與 API Key，非完全本地離線運作，有資料上傳疑慮與配額限制。 |
| mcp-audit | https://github.com/apisec-inc/mcp-audit | 157 | 2026-08-18 | 能用 | 專門掃描 MCP 配置檔與 AI 工具權限，檢測是否存在外洩憑證與 Shadow API。 | 僅針對 MCP 伺服器與配置本身進行審計，不負責日常 Git 程式碼庫與一般檔案的 secret 掃描。 |
| argus-codescan-mcp | https://github.com/argus-code-scanning/argus-codescan-mcp | 8 | 2026-08-05 | 能用 | 開源安全掃描器 Argus 的 MCP 介面，支援 SAST、Secrets、IaC 掃描。 | 重型多合一掃描框架，secret 掃描僅為子模組，依賴底層多種外部工具鏈安裝。 |
| ghas-mcp | https://github.com/dipsylala/ghas-mcp | 0 | 2026-06-07 | 玩具 | 單一 Go 二進位檔 MCP server，供 AI 查詢 GitHub Advanced Security 警報資料。 | 僅能唯讀查詢 GitHub 雲端既有的 GHAS 警報，無法在本地端離線掃描或攔截。 |
| security-mcp-suite | https://github.com/guanpengzhang0-spec/security-mcp-suite | 0 | 2026-04-30 | 玩具 | 包含 5 個 MCP server 的集合套件（nmap, nuclei, trivy, sqlmap, gitleaks）。 | 概念展示型腳本集合，無測試、零維護、無防禦或攔截機制。 |
| ship-safe | https://github.com/asamassekou10/ship-safe | 826 | 2026-08-29 | 能用 | Agentic 時代的 CLI 與 MCP 安全掃描器，檢測 CI/CD 配置錯誤、Agent 權限風險與硬編碼憑證。 | 主打 Agent 執行環境合規性，非專注於 Git pre-commit 細粒度 secret 攔截與修復引導。 |
| medusa | https://github.com/Pantheon-Security/medusa | 971 | 2026-08-10 | 能用 | AI 時代安全掃描器，內建 `medusa scan --git` 與 `medusa secrets scan`，支援 40,000+ 特徵規則並審查 `.claude/`。 | 定位為全功能 SAST/攻擊特徵掃描引擎，工具體積與規則庫龐大，非輕量專用 secret 守門員。 |

---

## 4. Claude Code / Cursor 生態的 secret 防護

| 名稱 | URL | stars | 最後更新 | 做什麼 | 缺什麼 |
| :--- | :--- | :--- | :--- | :--- | :--- |
| cc-safety-net | https://github.com/kenryu42/cc-safety-net | 1512 | 2026-08-29 | 跨 AI Coding Agent（Claude Code、Cursor、Codex 等）的 CLI hook 防護網，攔截破壞性 Git 操作與敏感檔案存取。 | 偏重於阻擋危險 bash 指令與特定敏感檔名存取，缺乏對 diff 內容中高熵字串/API Key 的深度內容層級掃描。 |
| claude-code-privacy-guard | https://github.com/datumbrain/claude-code-privacy-guard | 14 | 2026-07-23 | Claude Code 專用 plugin，在 prompt 傳送給 Claude 之前攔截包含 API Key、Secrets 與 PII 的輸入。 | 專注於 Prompt 傳出攔截（Inbound），未整合 Git pre-commit 本地 commit 阻擋機制。 |
| heimdall | https://github.com/frahet/heimdall | 0 | 2026-05-06 | Claude Code 的 PreToolUse hook，在 AI 執行 cat/Read 等工具讀取檔案前掃描內容，防範敏感憑證被讀入 context。 | 個人 PoC，星數為 0，僅防範 Tool Use 讀入，未防範 AI 寫入或 Git commit 洩漏。 |
| secguard | https://github.com/random1st/secguard | 65 | 2026-08-09 | 具備 3 層安全防禦（正則、啟發式、ML 分類）的 AI Agent 工具箱，包含 Claude Code hooks、Git pre-commit 與 CI 掃描。 | 依賴自訂 Python 環境與 ML 模型，安裝步驟繁瑣，缺乏 GUI 與直覺的桌面狀態提醒。 |
| safe-push | https://github.com/JamesShi96/safe-push | 12 | 2026-04-25 | Claude Code 的 Pre-commit 敏感資訊掃描 Skill，在 push 或 commit 前呼叫掃描邏輯。 | 依賴使用者或 Agent 主動呼叫 skill，非底層強制性（deterministic）的 Git Hook 攔截。 |
| sanitize | https://github.com/vxcozy/sanitize | 20 | 2026-03-03 | 提供 12 項檢測的 Pre-commit hook 與 Claude Code skill，專門阻擋憑證、API key 與敏感資料。 | 規則庫較小（僅 12 項規則），無動態憑證有效性驗證，誤報率較高且無 GUI 互動。 |
| agent-skills | https://github.com/GitGuardian/agent-skills | 7 | 2026-08-26 | GitGuardian 為 Claude Code、Cursor、Codex 設計的 Agent Skill，透過 ggshield 掃描程式碼中的 secret。 | 必須在本機預先安裝 ggshield 並註冊 GitGuardian 帳號，未整合原生單鍵安裝 hook。 |
| secretless-ai | https://github.com/opena2a-org/secretless-ai | 24 | 2026-08-29 | 單一指令防止 secrets 進入 LLM，支援 Claude Code、Cursor、Copilot、Windsurf 等多種 AI 工具。 | 著重於環境變數虛擬化與 Proxy 遮蔽，未提供 Git pre-commit commit-time 阻擋與視覺化介面。 |
| agent-security | https://github.com/mintmcp/agent-security | 75 | 2025-10-21 | 為 Claude Code 與 Cursor 提供的 secret scanning hooks 專案。 | 停更多時（2025 年底至今未更新），未跟進 Claude Code 與 Cursor 最新的 hook/skill 規格。 |
| Claudoscope | https://github.com/cordwainersmith/Claudoscope | 232 | 2026-08-26 | 原生 macOS App，為 Claude Code 與 Cowork 階段提供即時儀表板、對話歷史分析、安全加固與即時 secrets 檢測。 | 僅限 macOS 平台，偏重於 Claude Code 整體 session 監控儀表板，非獨立通用於所有 Git 專案的 secret 守門員。 |
| agent-sweep | https://github.com/Ishannaik/agent-sweep | 70 | 2026-08-25 | 尋找並遮蔽（redact）AI Coding Agent（如 Claude Code）對話紀錄與檔案歷史中的 secrets。 | 屬於事後修補工具，無法在 commit 發生的當下做即時阻擋。 |
| Code-VulnScan-Skill | https://github.com/Bhanunamikaze/Code-VulnScan-Skill | 6 | 2026-08-13 | 支援 Claude Code/Cursor/Windsurf 的深層漏洞掃描 Skill，包含污點分析、Secrets 檢測與 IaC 安全。 | 執行耗時較長，需要 Agent 完整分析程式碼邏輯，無法作為毫秒級 pre-commit hook 阻擋。 |

---

## 5. 專案缺點與局限分析

綜合目前 GitHub、npm 與 VS Code Marketplace 的專案生態，各類包裝與防護專案存在以下明顯缺點與局限：

1. **產品形態割裂與嚴重過期**：第三方程式碼包裝多數為 2024～2025 年的個人 PoC 或學生作業，星數大多在 0～15 顆之間且已停止維護；現存工具要嘛是純 CLI（報錯生硬、無互動修復介面），要嘛是重型的 ASPM/CI 儀表板（需搭建伺服器與資料庫），缺乏開箱即用的本機輕量產品。
2. **缺乏即時處置與修復閉環**：現有 Gitleaks 工具在攔截到金鑰時，僅回傳終端機 exit 1 與死板路徑，缺乏「一鍵轉移至 `.env`」、「一鍵加入 `.gitleaksignore` 誤報白名單」或「呼叫 API 驗證是否為有效 Live Key」的即時輔助工作流。
3. **AI Agent 防禦方向偏離 commit-time**：在 Claude Code / Cursor 生態中，多數專案集中在「Prompt 輸入端遮蔽（防止傳給 LLM）」或「對話紀錄事後掃描（Post-audit）」，在「本地 Git commit 前確定性阻擋（Deterministic Pre-commit Gate）＋ MCP 自動修復」的雙向整合上仍高度碎片化且缺乏主導產品。
4. **商業 SaaS 綁定門檻高**：具備較高成熟度與修復能力的方案（如 GitGuardian ggshield / ggmcp）強制綁定商業雲端帳號與 API Key，有程式碼上傳疑慮且受限於網路連線與免費額度，非 100% 本地離線可用。

---

## 空位判斷

1. **本地原生輕量守門員（Desktop / Menu Bar + CLI Hook）完全空白**：目前市場上缺乏一款零依賴、純本地離線、能在 Git commit 攔截到金鑰時即時彈出 macOS / 桌面互動視窗引導開發者「一鍵抽取至 `.env`」或「一鍵標記誤報」的輕量產品。
2. **AI Coding Agent 的確定性本地安全閉環尚未被標準化**：雖然開發者大量使用 Claude Code 與 Cursor，但現有防護要嘛是粗糙的檔名黑名單 hook，要嘛是需連網的商業 SaaS；將 Gitleaks 的高效離線檢測包裝為「底層強制 Git Hook ＋ 上層 Agent 智慧修復 Skill/MCP」具備極佳的生態位。
3. **結論**：以 Gitleaks / Betterleaks 為核心引擎，打造「極簡本機安裝、雙模運作（終端機 Hook + 選單列/桌面通知）、支援一鍵修復與 AI Agent 協作」的開發者端點安全產品，在現階段賽道中存在非常清晰且無強大競品的市場空缺。
