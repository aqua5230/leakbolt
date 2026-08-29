# 使用者作業系統分布與 Git Hook 衝突事實查證報告

## 1. 開發者作業系統分布調查數據與主流 AI 編碼工具作業系統支援現況

### 1.1 開發者作業系統分布公開調查數據

#### (1) Stack Overflow Developer Survey 2025
- **調查對象**：全球 49,000+ 位開發者。
- **數據類型**：開發者主要作業系統分布（複選）。
- **數據內容**：
  - **個人使用（Personal use）**：
    - Windows：56.7%
    - macOS：32.7%
    - Android：29.1%
    - Ubuntu：27.8%
    - iOS：18.9%
    - Linux（non-WSL）：17.6%
    - Windows Subsystem for Linux (WSL)：15.9%
    - Debian：11.4%
    - Arch：9.7%
    - iPadOS：8.1%
    - Fedora：5.8%
    - NixOS：3.4%
    - Red Hat：1.8%
  - **專業工作使用（Professional use）**：
    - Windows：49.5%
    - macOS：32.9%
    - Ubuntu：27.7%
    - Windows Subsystem for Linux (WSL)：16.8%
    - Linux（non-WSL）：16.7%
    - Android：11.9%
    - iOS：10.5%
    - Debian：10.4%
    - Red Hat：5.7%
    - Arch：4.6%
    - Fedora：3.7%
    - iPadOS：2.8%
- **來源 URL**：`https://survey.stackoverflow.co/2025/technology`（HTTP 狀態碼：`200`）

#### (2) Stack Overflow Developer Survey 2024
- **調查對象**：全球 65,000+ 位開發者。
- **數據內容**：
  - **個人使用（Personal use）**：
    - Windows：59.2%
    - macOS：31.8%
    - Ubuntu：27.7%
    - Android：17.9%
    - Windows Subsystem for Linux (WSL)：17.1%
    - iOS：11.5%
    - Debian：9.8%
    - Other Linux-based：8.4%
    - Arch：8.0%
    - iPadOS：5.3%
    - Fedora：4.8%
    - Red Hat：2.3%
  - **專業工作使用（Professional use）**：
    - Windows：47.6%
    - macOS：31.8%
    - Ubuntu：27.7%
    - Windows Subsystem for Linux (WSL)：16.8%
    - Debian：9.1%
    - Android：8.4%
    - Other Linux-based：8.0%
    - iOS：7.3%
    - Red Hat：4.9%
    - Arch：4.3%
    - Fedora：3.3%
    - iPadOS：2.7%
- **來源 URL**：`https://survey.stackoverflow.co/2024/technology`（HTTP 狀態碼：`200`）

#### (3) JetBrains State of Developer Ecosystem 2023
- **調查對象**：全球 26,348 位開發者。
- **數據內容**：
  - **開發環境作業系統分布（On which operating systems are your development environments?，複選）**：
    - Windows：64%
    - Linux：43%
    - macOS：42%
    - Other：1%
  - **跨平台桌面應用程式開發目標平台（Target platforms for cross-platform desktop apps）**：
    - Windows：88%
    - Linux：77%
    - macOS：53%
    - Other：2%
- **來源 URL**：`https://www.jetbrains.com/lp/devecosystem-2023/development/`（HTTP 狀態碼：`200`）

---

### 1.2 主流 AI 編碼工具官方作業系統支援與原生 Windows 版查證

| 工具名稱 | 官方支援之作業系統形態 | 是否有原生 Windows 版 | 事實說明與安裝方式 | 來源 URL 與 HTTP 狀態碼 |
| :--- | :--- | :--- | :--- | :--- |
| **Cursor** | macOS (Intel / Apple Silicon)、Windows (x64 / ARM64)、Linux (AppImage / .deb / .tar.gz) | **有** | 官方提供 Windows 原生 64 位元及 ARM64 的 `.exe` 桌面安裝程式。 | `https://www.cursor.com/download`（HTTP `200`） |
| **Claude Code** | macOS、Linux、Windows（原生 CMD/PowerShell 與 WSL 2） | **有** | 支援原生 Windows 10 (1809+) 與 Windows 11，可透過 PowerShell（`irm https://claude.ai/install.ps1 | iex`）或 CMD（`install.cmd`）原生安裝運行，預設搭配 Git for Windows 與 PowerShell；同時亦支援 WSL 2。 | `https://docs.anthropic.com/en/docs/agents-and-tools/claude-code/overview`（HTTP `200`） |
| **Lovable** | Web 瀏覽器（SaaS）、macOS 桌面版、Windows 桌面版 | **有** | 提供原生 Windows Desktop 安裝包（支援本地 Local MCP Server 連接 Figma/Paper、多專案分頁與原生快捷鍵）。 | `https://docs.lovable.dev/integrations/desktop-app`（HTTP `200`） |
| **v0 (by Vercel)** | Web 瀏覽器（純雲端 SaaS 平台） | **無** | v0 官方為純瀏覽器運行的雲端 Web 應用服務，無任何 Windows、macOS 或 Linux 原生桌面安裝程式。 | `https://v0.dev/faq`（HTTP `200`） |
| **Replit** | Web 瀏覽器、macOS 桌面版、Windows 桌面版、iOS / Android 行動版 | **有** | 官方提供 Windows 原生 Replit Desktop App 安裝程式（具備原生多工處理、狀態訊號與分頁預覽）。 | `https://replit.com/desktop`（HTTP `200`） |

---

## 2. Git core.hooksPath 機制與 Husky / Lefthook / Pre-commit 衝突事實查證

### 2.1 Git `core.hooksPath` 機制特性（單一值與取代行為）
- **是否為單一值**：**是**。Git 配置中的 `core.hooksPath` 為單一目錄路徑字串（可為絕對路徑或相對於執行目錄的相對路徑），不支援路徑列表（如 PATH 式多路徑）或串聯配置。
- **是否完全取代 `.git/hooks`**：**是**。當 Git 設定了 `core.hooksPath`（無論於 `--system`、`--global` 或 `--local` 層級），Git 在觸發任何 hook 時，只會在 `core.hooksPath` 所指向的目錄中搜尋對應名稱之 hook 執行檔，**完全忽略並取代**本機專案預設的 `.git/hooks/` 目錄，兩者不會合併或連鎖執行。
- **來源 URL**：
  - Git Config 官方文件：`https://git-scm.com/docs/git-config`（HTTP 狀態碼：`200`）
  - Git Hooks 官方文件：`https://git-scm.com/docs/githooks`（HTTP 狀態碼：`200`）

---

### 2.2 Husky v9 安裝機制與 `core.hooksPath` 設定行為
- **安裝與執行機制**：
  - Husky v9 透過執行 `husky`（或 `npx husky init` 於 `package.json` 加入 `"prepare": "husky"`）進行初始化。
  - Husky v9 的核心邏輯直接調用 Git 命令設定 Repository 層級（`--local`）的 `core.hooksPath`，指令為：
    `git config core.hooksPath .husky/_`
  - Husky 在 `.husky/_/` 目錄下生成所有 Git hook 的包裝腳本（包含 `pre-commit`、`commit-msg` 等），當 Git 觸發 hook 時，會執行 `.husky/_/<hook>` 腳本，進而調用 `.husky/<hook>` 中的使用者自訂指令。
- **是否設定 repo 層級 `core.hooksPath`**：**會**。Husky v9 必定會修改專案 `.git/config` 中的 `core.hooksPath` 指向 `.husky/_`。
- **原始碼依據**（`typicode/husky` 之 `index.js` 第 12 行）：
  ```javascript
  let { status: s, stderr: e } = c.spawnSync('git', ['config', 'core.hooksPath', `${d}/_`])
  ```
- **來源 URL**：
  - Husky 官方網站：`https://typicode.github.io/husky/`（HTTP 狀態碼：`200`）
  - Husky GitHub 倉庫：`https://github.com/typicode/husky`（HTTP 狀態碼：`200`）
  - Husky 原始碼 `index.js`：`https://raw.githubusercontent.com/typicode/husky/main/index.js`（HTTP 狀態碼：`200`）

---

### 2.3 Lefthook 安裝機制與 `core.hooksPath` 衝突處理
- **安裝與執行機制**：
  - Lefthook 透過執行 `lefthook install` 安裝 hook。
  - Lefthook 透過 `git rev-parse --path-format=absolute --show-toplevel --git-path hooks ...` 取得 hook 安裝路徑，並直接將二進位／腳本寫入該目錄（預設為 `.git/hooks/`）。
- **是否會覆寫 `core.hooksPath`**：**不會主動設定或覆寫**。Lefthook 本身不使用 `core.hooksPath` 機制，而是直接寫入 `.git/hooks/`。
- **面對 `core.hooksPath` 衝突時的行為**：
  - Lefthook 在安裝時會調用 `ensureHooksPathUnset()` 函式主動檢查 `git config --local core.hooksPath` 與 `git config --global core.hooksPath`。
  - 若偵測到 `core.hooksPath` 已被設定且不等於 `.git/hooks`，Lefthook 會視為衝突並**直接中斷報錯（拒絕安裝）**，輸出錯誤提示：`Custom hooks paths are not supported by default.`。
  - 使用者必須加上 `--reset-hooks-path` 參數（由 Lefthook 執行 `git config --unset-all core.hooksPath` 將其清除），或加上 `--force` 強制寫入自訂路徑，否則無法完成安裝。
- **原始碼依據**（`evilmartians/lefthook` 之 `internal/command/install.go` 第 514-551 行 與 `internal/git/paths.go`）：
  ```go
  // internal/command/install.go:514
  func (l *Lefthook) ensureHooksPathUnset(force, resetHooksPath bool) error {
      local, global := l.getHooksPathConfig()
      ...
      if !force && !resetHooksPath {
          l.logger.Error(formatHooksPathError(local, global))
          return errors.New("")
      }
      if resetHooksPath {
          return l.unsetHooksPathConfig(local, global)
      }
      ...
  }
  ```
- **來源 URL**：
  - Lefthook GitHub 倉庫：`https://github.com/evilmartians/lefthook`（HTTP 狀態碼：`200`）
  - Lefthook 原始碼 `install.go`：`https://raw.githubusercontent.com/evilmartians/lefthook/master/internal/command/install.go`（HTTP 狀態碼：`200`）
  - Lefthook 原始碼 `paths.go`：`https://raw.githubusercontent.com/evilmartians/lefthook/master/internal/git/paths.go`（HTTP 狀態碼：`200`）

---

### 2.4 Pre-commit (Python 框架) 安裝機制與 `core.hooksPath` 衝突處理
- **安裝與執行機制**：
  - pre-commit 透過執行 `pre-commit install` 安裝 hook。
  - pre-commit 透過 `git rev-parse --git-common-dir` 計算路徑，並固定將 hook 腳本寫入 `.git/hooks/<hook_type>`（例如 `.git/hooks/pre-commit`）。
- **是否會覆寫 `core.hooksPath`**：**不會主動設定或覆寫**。pre-commit 不依賴 `core.hooksPath`，永遠只寫入 `.git/hooks/`。
- **面對 `core.hooksPath` 衝突時的行為**：
  - pre-commit 在安裝時會調用 `git.has_core_hookpaths_set()` 檢查是否存在 `core.hooksPath` 設定（`git config core.hooksPath`）。
  - 若偵測到 `core.hooksPath` 已設定，pre-commit 認為寫入 `.git/hooks` 會被 Git 忽略導致 hook 失效，因此會**直接中斷安裝並返回 exit code 1**，輸出明確錯誤訊息：
    `Cowardly refusing to install hooks with core.hooksPath set.`
    `hint: git config --unset-all core.hooksPath`。
- **原始碼依據**（`pre-commit/pre-commit` 之 `pre_commit/commands/install_uninstall.py` 第 123-128 行 與 `pre_commit/git.py` 第 180-182 行）：
  ```python
  # pre_commit/commands/install_uninstall.py:123
  if git_dir is None and git.has_core_hookpaths_set():
      logger.error(
          'Cowardly refusing to install hooks with `core.hooksPath` set.\n'
          'hint: `git config --unset-all core.hooksPath`',
      )
      return 1

  # pre_commit/git.py:180
  def has_core_hookpaths_set() -> bool:
      _, out, _ = cmd_output_b('git', 'config', 'core.hooksPath', check=False)
      return bool(out.strip())
  ```
- **來源 URL**：
  - pre-commit 官方網站：`https://pre-commit.com/`（HTTP 狀態碼：`200`）
  - pre-commit GitHub 倉庫：`https://github.com/pre-commit/pre-commit`（HTTP 狀態碼：`200`）
  - pre-commit 原始碼 `install_uninstall.py`：`https://raw.githubusercontent.com/pre-commit/pre-commit/main/pre_commit/commands/install_uninstall.py`（HTTP 狀態碼：`200`）
  - pre-commit 原始碼 `git.py`：`https://raw.githubusercontent.com/pre-commit/pre-commit/main/pre_commit/git.py`（HTTP 狀態碼：`200`）

---

### 2.5 Hook 管理工具機制與衝突總結對照

| 工具 | 安裝目標路徑 | 是否設定 `core.hooksPath` | 當環境已有 `core.hooksPath` 時的行為 |
| :--- | :--- | :--- | :--- |
| **Husky (v9)** | `.husky/_/` 與 `.husky/` | **是**（設定 `git config core.hooksPath .husky/_`） | 直接覆寫 repo 層級的 `core.hooksPath` 設定為 `.husky/_`，導致原先在 `.git/hooks/` 的其他工具 hook 完全失效。 |
| **Lefthook** | `.git/hooks/` | **否**（不設定） | 拒絕安裝並報錯，提示使用者以 `--reset-hooks-path` 清除 `core.hooksPath`，或使用 `--force`。 |
| **pre-commit (Python)** | `.git/hooks/` | **否**（不設定） | 拒絕安裝並報錯（`Cowardly refusing`），提示使用者先執行 `git config --unset-all core.hooksPath`。 |
