## 1. gitleaks 官方 README／文件／repo 裡，建議的 pre-commit hook 腳本原文是什麼？把腳本內容原封不動貼出來。

來源 URL：https://github.com/gitleaks/gitleaks/blob/master/scripts/pre-commit.py [HTTP 200]
（歷史文件參照：https://github.com/gitleaks/gitleaks/blob/master/README.md [HTTP 200]）

腳本原文（`scripts/pre-commit.py`）：

```python
#!/usr/bin/env python3
"""Helper script to be used as a pre-commit hook."""
import os
import sys
import subprocess


def gitleaksEnabled():
    """Determine if the pre-commit hook for gitleaks is enabled."""
    out = subprocess.getoutput("git config --bool hooks.gitleaks")
    if out == "false":
        return False
    return True


if gitleaksEnabled():
    exitCode = os.WEXITSTATUS(os.system('gitleaks protect -v --staged'))
    if exitCode == 1:
        print('''Warning: gitleaks has detected sensitive information in your changes.
To disable the gitleaks precommit hook run the following command:

    git config hooks.gitleaks false
''')
        sys.exit(1)
else:
    print('gitleaks precommit disabled\
     (enable with `git config hooks.gitleaks true`)')
```

## 2. 那個腳本有沒有「先確認 gitleaks 存在才執行」這類保護？（例如 `command -v gitleaks`、`which gitleaks`、`if [ -x ... ]`）

來源 URL：https://github.com/gitleaks/gitleaks/blob/master/scripts/pre-commit.py [HTTP 200]

沒有。腳本中直接呼叫 `os.system('gitleaks protect -v --staged')`，沒有使用 `command -v`、`which`、`shutil.which`、`if [ -x ... ]` 或任何預先確認 `gitleaks` 執行檔是否存在的保護檢查。

## 3. gitleaks 有沒有提供環境變數之類的跳過機制？（例如 GITLEAKS_ENABLE=false、或文件裡叫人用 git commit --no-verify）

來源 URL：
- https://github.com/gitleaks/gitleaks/blob/master/scripts/pre-commit.py [HTTP 200]
- https://github.com/gitleaks/gitleaks/blob/master/README.md [HTTP 200]

1. **`scripts/pre-commit.py` 腳本中的跳過機制**：透過 Git 設定值判斷。使用 `git config hooks.gitleaks false` 停用，使用 `git config hooks.gitleaks true` 啟用。
2. **README 官方建議 pre-commit framework 的跳過機制**：使用環境變數 `SKIP=gitleaks git commit -m "..."`。
3. **gitleaks CLI 本身與文件**：
   - 無 `GITLEAKS_ENABLE=false` 這類環境變數開關。
   - 文件中未提及使用 `git commit --no-verify`。
   - 官方提供的略過方式為在程式碼註解加入 `#gitleaks:allow`，或在 `.gitleaksignore` 設定 fingerprint。

## 4. gitleaks 官方有沒有提供 pre-commit framework 的 hook 定義（.pre-commit-hooks.yaml）？裡面的 entry 怎麼寫？

來源 URL：https://github.com/gitleaks/gitleaks/blob/master/.pre-commit-hooks.yaml [HTTP 200]

有提供。`.pre-commit-hooks.yaml` 設定檔原文如下：

```yaml
- id: gitleaks
  name: Detect hardcoded secrets
  description: Detect hardcoded secrets using Gitleaks
  entry: gitleaks git --pre-commit --redact --staged --verbose
  language: golang
  pass_filenames: false
- id: gitleaks-docker
  name: Detect hardcoded secrets
  description: Detect hardcoded secrets using Gitleaks
  entry: zricethezav/gitleaks git --pre-commit --redact --staged --verbose
  language: docker_image
  pass_filenames: false
- id: gitleaks-system
  name: Detect hardcoded secrets
  description: Detect hardcoded secrets using Gitleaks
  entry: gitleaks git --pre-commit --redact --staged --verbose
  language: system
```

各 hook 的 entry 內容：
- `id: gitleaks` 的 entry：`gitleaks git --pre-commit --redact --staged --verbose`
- `id: gitleaks-docker` 的 entry：`zricethezav/gitleaks git --pre-commit --redact --staged --verbose`
- `id: gitleaks-system` 的 entry：`gitleaks git --pre-commit --redact --staged --verbose`
