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

驗證方式。**不要**去改真正的系統設定——用 `GIT_CONFIG_SYSTEM` 這個環境變數
把 git 的 system 設定臨時指到一個暫存檔就好。這樣不用系統管理員權限，
關掉這個視窗就自動復原，沒有任何東西需要善後。

在 PowerShell（一般權限即可）貼這三行：

```
$env:GIT_CONFIG_SYSTEM = "$env:TEMP\fake-system-gitconfig"
git config --file $env:GIT_CONFIG_SYSTEM core.hooksPath C:/temp/myhooks
.\dist\leakbolt-windows-amd64.exe install
```

預期看到：`警告：未安裝 LeakBolt。git config --system core.hooksPath 已設為 "C:/temp/myhooks"；LeakBolt 不會覆寫既有 hook 路徑。`

看到別的（例如「已安裝到 ...」）就是這個修復在 Windows 上沒生效，請回報。

測完直接關掉這個 PowerShell 視窗，環境變數就沒了，你的 git 設定從頭到尾沒被動過。

（這個做法已在 macOS 實測過：設了 `GIT_CONFIG_SYSTEM` 後
`git config --system --get core.hooksPath` 讀得到暫存檔的值，
沒設時回 exit 1。leakbolt 內部就是跑這個指令。）

**情境 E：leakbolt 不在 PATH 上時，hook 要擋下 commit**

這版改掉了一個真的漏洞：舊版的 hook 找不到 leakbolt 就整段跳過、直接放行，
帶著密鑰的 commit 會安靜通過，一個字都不印。

驗證方式：把 `leakbolt.exe` 暫時移到別的資料夾（或從 PATH 拿掉），
然後帶一個假密鑰 commit。預期看到：

```
LeakBolt：找不到 leakbolt 執行檔，commit 已中止。
  這個 repo 裝過 LeakBolt，但現在 PATH 上找不到它。
  把 leakbolt 放回 PATH，或刪掉 .git/hooks/pre-commit 停用檢查。
  這次要跳過檢查：git commit --no-verify
```

commit 要失敗。如果 commit 成功了，代表這個修復在 Windows 上沒生效，請回報。

假密鑰用這個（是專案語料庫裡的真陽性樣本，不是真的 key）：

```
AWS_ACCESS_KEY_ID = "AKIAIMNOJVGFDXXXE4OA"
```

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

## 確認你手上的 exe 是修復後的版本

`.exe` 不進版控，所以檔案本身看不出是哪一版。複製到 Windows 之後先對雜湊：

```
certutil -hashfile dist\leakbolt-windows-amd64.exe SHA256
certutil -hashfile dist\leakbolt-windows-arm64.exe SHA256
```

2026-08-29 22:42 修復後打包的版本應該是：

```
amd64  5781c6742d3a0081e7d9b557959d67fe4b70667aad8a547b067e64bd488b28c4
arm64  23a43577ccc2115c7e6731e26534f3a16f30d0be0c4a8e19eb0693774ab0e16f
```

（雜湊每次重新打包都會變。以本檔記錄的時間為準，對不上就是拿到舊的。）

對不上就是複製到舊版了，重新複製一次再測——不然測出來的 FAIL 無法判讀。
