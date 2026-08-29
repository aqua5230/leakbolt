# LeakBolt Windows 相容性測試
#
# 在 Windows 上跑：PowerShell 開在這個 repo 的根目錄，執行
#   powershell -ExecutionPolicy Bypass -File scripts\windows_matrix.ps1
#
# 做兩件 macOS 與 Linux 測不到的事：
#   1. 用 Git Bash 跑同一份相容矩陣（Git for Windows 內附 bash）
#   2. 測「從 PowerShell 直接 git commit」時 hook 會不會被執行
#      —— 這是 research/06 記過的已知風險：hook 是 shell 腳本，
#         不附 Git Bash 的環境或 GUI 客戶端可能不會執行它。

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot

function Write-Section($text) {
    Write-Host ""
    Write-Host "=== $text ===" -ForegroundColor Cyan
}

Write-Section "環境"
$gitVersion = (git --version)
Write-Host "git: $gitVersion"
Write-Host "OS : $([System.Environment]::OSVersion.VersionString)"
Write-Host "架構: $env:PROCESSOR_ARCHITECTURE"

# --- 找 leakbolt 執行檔 ---
$arch = if ($env:PROCESSOR_ARCHITECTURE -eq "ARM64") { "arm64" } else { "amd64" }
$exe = Join-Path $root "dist\leakbolt-windows-$arch.exe"
if (-not (Test-Path $exe)) {
    Write-Host "找不到 $exe" -ForegroundColor Red
    Write-Host "請先在 macOS/Linux 上執行： GOOS=windows GOARCH=$arch go build -o dist\leakbolt-windows-$arch.exe ." -ForegroundColor Red
    exit 2
}
Write-Host "執行檔: $exe"

# --- 找 gitleaks ---
$gitleaks = Get-Command gitleaks -ErrorAction SilentlyContinue
if (-not $gitleaks) {
    Write-Host "找不到 gitleaks。請先安裝 v8.30.1：" -ForegroundColor Red
    Write-Host "  winget install gitleaks" -ForegroundColor Yellow
    Write-Host "  或從 https://github.com/gitleaks/gitleaks/releases 下載 gitleaks_8.30.1_windows_x64.zip" -ForegroundColor Yellow
    exit 2
}
Write-Host "gitleaks: $(gitleaks version)"

# --- 把 leakbolt 放上 PATH（hook 裡的守衛要找得到它）---
$binDir = Join-Path $env:TEMP "leakbolt-bin"
New-Item -ItemType Directory -Force -Path $binDir | Out-Null
Copy-Item $exe (Join-Path $binDir "leakbolt.exe") -Force
$env:PATH = "$binDir;$env:PATH"
Write-Host "leakbolt: $(leakbolt --help 2>&1 | Select-Object -First 1)"

# ---------------------------------------------------------------
Write-Section "情境 A：從 PowerShell 直接 git commit，hook 會被執行嗎"
# 這是 Windows 專屬的關鍵未知。hook 是 sh 腳本，
# 但 Git for Windows 會用內附的 sh 去執行它，跟呼叫端是哪個 shell 無關——
# 這件事必須實測，不能用推論。

$repo = Join-Path ([System.IO.Path]::GetTempPath()) ("leakbolt-ps-" + [guid]::NewGuid().ToString("N").Substring(0,8))
New-Item -ItemType Directory -Force -Path $repo | Out-Null
Push-Location $repo
try {
    git init -q .
    git config user.email t@t
    git config user.name t
    "hi" | Out-File -Encoding ascii a.txt
    git add a.txt
    git commit -qm init

    leakbolt install --local-only | Out-Null

    'AWS_KEY = "AKIAIMNOJVGFDXXXE4OA"' | Out-File -Encoding ascii config.py
    git add config.py

    $output = (git commit -m leak 2>&1 | Out-String)
    $exitCode = $LASTEXITCODE

    Write-Host "git commit 退出碼: $exitCode"
    Write-Host "輸出:"
    Write-Host $output

    if ($output -match "暫存區掃描完成") {
        Write-Host "PASS  hook 有被執行，而且是 LeakBolt 擋下來的" -ForegroundColor Green
    } elseif ($exitCode -eq 0) {
        Write-Host "FAIL  commit 過了——hook 根本沒被執行，Windows 上沒有保護" -ForegroundColor Red
    } else {
        Write-Host "FAIL  commit 被擋，但不是 LeakBolt 擋的（輸出裡沒有我們的指紋）" -ForegroundColor Red
    }
} finally {
    Pop-Location
    Remove-Item -Recurse -Force $repo -ErrorAction SilentlyContinue
}

# ---------------------------------------------------------------
Write-Section "情境 B：doctor 在 Windows 上會不會跳假警報"
# Go 在 Windows 從不回報執行位元，用 0o111 判斷可達性會讓每次安裝都跳假警報。
# 這個已經修過，這裡驗證修法有效。

$repo2 = Join-Path ([System.IO.Path]::GetTempPath()) ("leakbolt-doc-" + [guid]::NewGuid().ToString("N").Substring(0,8))
New-Item -ItemType Directory -Force -Path $repo2 | Out-Null
Push-Location $repo2
try {
    git init -q .
    git config user.email t@t
    git config user.name t
    "hi" | Out-File -Encoding ascii a.txt
    git add a.txt
    git commit -qm init

    $installOut = (leakbolt install --local-only 2>&1 | Out-String)
    Write-Host "install 輸出:"
    Write-Host $installOut
    if ($installOut -match "不存在或不可執行") {
        Write-Host "FAIL  跳了假警報——執行位元的修法在 Windows 上沒生效" -ForegroundColor Red
    } else {
        Write-Host "PASS  沒有假警報" -ForegroundColor Green
    }

    $doctorOut = (leakbolt doctor 2>&1 | Out-String)
    $doctorCode = $LASTEXITCODE
    Write-Host "doctor 退出碼: $doctorCode"
    Write-Host $doctorOut
    if ($doctorCode -eq 0) {
        Write-Host "PASS  doctor 判定正常" -ForegroundColor Green
    } else {
        Write-Host "FAIL  doctor 判定異常，但這是一個剛裝好的乾淨 repo" -ForegroundColor Red
    }
} finally {
    Pop-Location
    Remove-Item -Recurse -Force $repo2 -ErrorAction SilentlyContinue
}

# ---------------------------------------------------------------
Write-Section "情境 C：用 Git Bash 跑完整相容矩陣"
$bashCandidates = @(
    "C:\Program Files\Git\bin\bash.exe",
    "C:\Program Files (x86)\Git\bin\bash.exe",
    "$env:LOCALAPPDATA\Programs\Git\bin\bash.exe"
)
$bash = $bashCandidates | Where-Object { Test-Path $_ } | Select-Object -First 1
if (-not $bash) {
    Write-Host "找不到 Git Bash，跳過完整矩陣。" -ForegroundColor Yellow
    Write-Host "這本身就是一個發現：沒有 Git Bash 的 Windows 環境，shell hook 不會運作。" -ForegroundColor Yellow
} else {
    Write-Host "Git Bash: $bash"
    $matrixPath = (Join-Path $root "scripts\hook_matrix.sh") -replace '\\', '/'
    $exePath = (Join-Path $binDir "leakbolt.exe") -replace '\\', '/'
    & $bash -c "LEAKBOLT_BIN='$exePath' bash '$matrixPath'"
}

Write-Host ""
Write-Host "測完了。請把上面全部輸出貼回給 Claude。" -ForegroundColor Cyan
