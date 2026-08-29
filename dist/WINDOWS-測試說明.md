# 在 Windows 上跑 LeakBolt 相容性測試

macOS 與 Linux 的相容矩陣都是 11/11，Windows 是唯一還沒有憑證的平台。
這份測試要回答三個問題，全部都是推論不出來、只能實測的。

## 先準備兩樣東西

**1. Git for Windows**（多半已經有了）
沒有的話：https://git-scm.com/download/win
它內附 Git Bash，這很重要——我們的 hook 是 shell 腳本，要靠它執行。

**2. gitleaks v8.30.1**
```
winget install gitleaks
```
或從 https://github.com/gitleaks/gitleaks/releases/tag/v8.30.1
下載 `gitleaks_8.30.1_windows_x64.zip`，解壓後把 `gitleaks.exe` 放進 PATH。

裝好後確認：
```
gitleaks version
```
要印出 `8.30.1`。

## 把這些檔案複製到 Windows

整個 `leakbolt` 資料夾複製過去就好，或至少要有這三個：

```
dist\leakbolt-windows-amd64.exe     （ARM 機器用 arm64 那個）
scripts\windows_matrix.ps1
scripts\hook_matrix.sh
```

## 執行

PowerShell 開在 `leakbolt` 資料夾底下：

```
powershell -ExecutionPolicy Bypass -File scripts\windows_matrix.ps1
```

`-ExecutionPolicy Bypass` 只影響這一次執行，不會改你系統的設定。

## 這個測試在問什麼

**情境 A：從 PowerShell 直接 `git commit`，hook 會不會被執行？**
這是最關鍵的未知。我們的 hook 是 sh 腳本，理論上 Git for Windows 會用內附的 sh 去跑它，
跟呼叫端是哪個 shell 無關——但這件事必須實測，不能用推論。
如果不會執行，那 Windows 上等於沒有保護。

**情境 B：`install` 會不會跳假警報？**
Go 在 Windows 從不回報檔案的執行位元。原本用 `Mode()&0o111` 判斷 hook 可不可執行，
在 Windows 上會 100% 判定為「不可執行」，每次安裝都跳警告。
這個已經修掉了，這裡驗證修法有效。

**情境 C：完整的 11 個相容情境**
用 Git Bash 跑跟 macOS／Linux 同一份腳本。
需要 `node`／`npm`（測 husky）、`lefthook`、`pre-commit`——沒裝的會顯示 SKIP，
腳本把 SKIP 算成失敗，那是刻意的：沒驗證就不算通過。

想跑完整版的話先裝：
```
winget install OpenJS.NodeJS
npm install -g lefthook
pip install pre-commit
```

## 跑完之後

把**全部輸出**貼回來給 Claude。包含 FAIL 的部分——那些才是有價值的資訊。

---

## 2026-08-29 更新：這版多了兩個要看的東西

**情境 D：system 層的 `core.hooksPath`（Windows 最容易踩到）**

Git for Windows 自帶一份 system 層 gitconfig。舊版 LeakBolt 只查 local 與 global，
所以只有 system 層設了 `core.hooksPath` 時，`install` 會把 hook 寫到
`.git\hooks\pre-commit`、回報安裝成功、`doctor` 也顯示正常，
但 git 實際上根本不會執行那個 hook——等於沒有保護卻以為有。這版修掉了。

驗證方式（會改到你的系統設定，要用系統管理員權限的 PowerShell，測完記得復原）：

```
git config --system core.hooksPath C:\temp\myhooks
leakbolt install
```

預期看到：`警告：未安裝 LeakBolt。git config --system core.hooksPath 已設為 "C:\temp\myhooks"；LeakBolt 不會覆寫既有 hook 路徑。`

復原（**這步一定要做，否則你之後所有 repo 的 hook 都會壞掉**）：

```
git config --system --unset core.hooksPath
```

不想動系統設定的話跳過這題，回報「未測」即可，不要硬測。

**新增的警告訊息**

這版起，gitleaks 版本不是 8.30.1 時，每次掃描都會在 stderr 印一行：

```
警告：gitleaks 版本為 <你的版本>，LeakBolt 鎖定的是 8.30.1。偵測結果可能與品質基準不同。
```

這是預期行為，不是壞掉，也**不會**擋住 commit。`leakbolt doctor` 那邊會把它列為異常。
如果你裝的就是 8.30.1，不該看到這行——看到了就是 bug，請回報。

## 重新編譯的指令（原本沒有記錄，補上）

```
cd prototype
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o ../dist/leakbolt-windows-amd64.exe .
CGO_ENABLED=0 GOOS=windows GOARCH=arm64 go build -o ../dist/leakbolt-windows-arm64.exe .
```
