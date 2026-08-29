# husky

版本：Husky v9（v9.1.7）與 Husky v8（v8.0.3）

1. 它產生／安裝的 hook 腳本，在找不到自己的執行檔時，是「擋下 commit」還是「安靜放行」？
- **Husky v9**：**擋下 commit**。
  - 機制：Husky v9 透過 `git config core.hooksPath .husky/_` 設定 hook 目錄。Git 觸發時執行 `.husky/_/<hook>`（如 `.husky/_/pre-commit`），該腳本執行 `. "$(dirname "$0")/h"`。在 helper 腳本 `h` 中，執行 `sh -e "$s" "$@"`（`$s` 為使用者定義的 hook 檔案，如 `.husky/pre-commit`）。當使用者 hook 中的指令在 `PATH` 中找不到時，子 shell 退出碼為 127，腳本捕捉 `$c = 127` 並執行 `exit $c`，導致非零退出碼而**擋下 commit**。（若使用者未建立該 hook 檔案 `$s`，則由 `[ ! -f "$s" ] && exit 0` 放行）。
  - 原始碼片段（`.husky/_/h` / `husky`）：
```sh
#!/usr/bin/env sh
[ "$HUSKY" = "2" ] && set -x
n=$(basename "$0")
s=$(dirname "$(dirname "$0")")/$n

[ ! -f "$s" ] && exit 0

if [ -f "$HOME/.huskyrc" ]; then
	echo "husky - '~/.huskyrc' is DEPRECATED, please move your code to ~/.config/husky/init.sh"
fi
i="${XDG_CONFIG_HOME:-$HOME/.config}/husky/init.sh"
[ -f "$i" ] && . "$i"

[ "${HUSKY-}" = "0" ] && exit 0

export PATH="node_modules/.bin:$PATH"
sh -e "$s" "$@"
c=$?

[ $c != 0 ] && echo "husky - $n script failed (code $c)"
[ $c = 127 ] && echo "husky - command not found in PATH=$PATH"
exit $c
```
- **Husky v8**：**擋下 commit**。
  - 機制：Husky v8 在 `.husky/<hook>` 中載入 `. "$(dirname -- "$0")/_/husky.sh"`。若指令找不到（退出碼 127），`husky.sh` 輸出 `command not found in PATH=$PATH` 並以 `exit $exitCode`（127）退出，**擋下 commit**。若未安裝依賴導致 `husky.sh` 不存在，shell source 失敗亦會以非零狀態碼中斷並**擋下 commit**。
  - 原始碼片段（`husky.sh`）：
```sh
#!/usr/bin/env sh
if [ -z "$husky_skip_init" ]; then
  debug () {
    if [ "$HUSKY_DEBUG" = "1" ]; then
      echo "husky (debug) - $1"
    fi
  }

  readonly hook_name="$(basename -- "$0")"
  debug "starting $hook_name..."

  if [ "$HUSKY" = "0" ]; then
    debug "HUSKY env variable is set to 0, skipping hook"
    exit 0
  fi

  if [ -f ~/.huskyrc ]; then
    debug "sourcing ~/.huskyrc"
    . ~/.huskyrc
  fi

  readonly husky_skip_init=1
  export husky_skip_init
  sh -e "$0" "$@"
  exitCode="$?"

  if [ $exitCode != 0 ]; then
    echo "husky - $hook_name hook exited with code $exitCode (error)"
  fi

  if [ $exitCode = 127 ]; then
    echo "husky - command not found in PATH=$PATH"
  fi

  exit $exitCode
fi
```

2. 它提供什麼跳過機制？
- Git 原生參數：`git commit -n` 或 `git commit --no-verify`
- 環境變數 `HUSKY=0`：
  - 單次指令：`HUSKY=0 git commit ...`
  - 當前 Shell 階段：`export HUSKY=0`
  - 全域或 GUI 客戶端設定：在 `~/.config/husky/init.sh` 中加入 `export HUSKY=0`
- 安裝與 CI 略過機制：在 CI 環境或生產環境下設定 `HUSKY=0` 避免安裝 hook；或在 `package.json` 中使用 `"prepare": "husky || true"`

3. 這個跳過機制的官方文件怎麼描述它的用途？
- 官方文件原文：
```
## Skipping Git Hooks

### For a Single Command

Most Git commands include a `-n/--no-verify` option to skip hooks:

```sh
git commit -m "..." -n # Skips Git hooks
```

For commands without this flag, disable hooks temporarily with HUSKY=0:

```shell
HUSKY=0 git ... # Temporarily disables all Git hooks
git ... # Hooks will run again
```

### For multiple commands

To disable hooks for an extended period (e.g., during rebase/merge):

```shell
export HUSKY=0 # Disables all Git hooks
git ...
git ...
unset HUSKY # Re-enables hooks
```

### For a GUI or Globally

To disable Git hooks in a GUI client or globally, modify the husky config:

```sh
# ~/.config/husky/init.sh
export HUSKY=0 # Husky won't install and won't run hooks on your machine
```

## CI server and Docker

To avoid installing Git Hooks on CI servers or in Docker, use `HUSKY=0`. For instance, in GitHub Actions:

```yml
# https://docs.github.com/en/actions/learn-github-actions/variables
env:
  HUSKY: 0
```
```

- 來源 URL：
  - https://github.com/typicode/husky/blob/main/husky [200]
  - https://github.com/typicode/husky/blob/main/index.js [200]
  - https://github.com/typicode/husky/blob/v8.0.3/husky.sh [200]
  - https://typicode.github.io/husky/how-to.html [200]
  - https://typicode.github.io/husky/troubleshoot.html [200]

# lefthook

版本：Lefthook v1.x（v1.11.x / master）

1. 它產生／安裝的 hook 腳本，在找不到自己的執行檔時，是「擋下 commit」還是「安靜放行」？
- **安靜放行**（預設情況）；若設定 `assert_lefthook_installed: true` 則為**擋下 commit**。
  - 機制：Lefthook 安裝於 `.git/hooks/<hook>` 的腳本由 `internal/templates/hook.tmpl` 生成。其 `call_lefthook` 函式會依序搜尋 `$LEFTHOOK_BIN`、安裝時路徑、PATH、`node_modules` 下二進位檔、`go tool`、`bundle exec`、`yarn`、`pnpm`、`swift`、`mint`、`uv`、`mise`、`devbox` 等。若全部未找到，腳本會輸出 `Can't find lefthook in PATH`。預設情況下（`.AssertLefthookInstalled` 為 `false`），模板不會輸出 `exit 1`，函式結束並回傳 0，因此為**安靜放行**。僅在 `lefthook.yml` 明確設定 `assert_lefthook_installed: true` 時，模板才會包含 `exit 1` 並**擋下 commit**。
  - 原始碼片段（`internal/templates/hook.tmpl`）：
```sh
    else
      echo "Can't find lefthook in PATH"
      {{- if .AssertLefthookInstalled}}
      echo "ERROR: Operation is aborted due to lefthook settings."
      echo "Make sure lefthook is available in your environment and re-try."
      echo "To skip these checks use --no-verify git argument or set LEFTHOOK=0 env variable."
      exit 1
      {{- end}}
    fi
```

2. 它提供什麼跳過機制？
- Git 原生參數：`git commit --no-verify`
- 環境變數 `LEFTHOOK=0` 或 `LEFTHOOK=false`：停用所有 lefthook 執行（腳本開頭判斷 `if [ "$LEFTHOOK" = "0" ]; then exit 0; fi`）
- 環境變數 `LEFTHOOK_EXCLUDE`：依標籤（tags）或指令名稱跳過指定檢查（例如 `LEFTHOOK_EXCLUDE=ruby,security,lint git commit ...`）
- 設定檔 `skip` 選項：可在 `lefthook.yml` 或 `lefthook-local.yml` 設定跳過條件：
  - 強制跳過：`skip: true`
  - 依 Git 動作狀態跳過：`skip: merge`、`skip: rebase`、`skip: merge-commit`
  - 依 Git 分支跳過：`skip: - ref: main`、`skip: - ref: dev/*`
  - 依自訂指令退出狀態跳過：`skip: - run: test "${NO_HOOK}" -eq 1` 或 `skip: - run: "! which aiautocommit"`

3. 這個跳過機制的官方文件怎麼描述它的用途？
- 官方文件原文：
  - `LEFTHOOK` 環境變數：
```
Use `LEFTHOOK=0 git ...` or `LEFTHOOK=false git ...` to disable lefthook when running git commands.
```
  - `LEFTHOOK_EXCLUDE` 環境變數：
```
Use `LEFTHOOK_EXCLUDE={list of tags or command names to be excluded}` to skip some commands or scripts by tag or name (for commands only). See the `exclude_tags` configuration option for more details.
```
  - `skip` 設定選項：
```
You can skip all or specific commands and scripts using `skip` option. You can also skip when merging, rebasing, or being on a specific branch. Globs are available for branches.
```
  - `assert_lefthook_installed` 設定選項：
```
When set to `true`, fail (with exit status 1) if `lefthook` executable can't be found in $PATH, under node_modules/, as a Ruby gem, or other supported method. This makes sure git hook won't omit `lefthook` rules if `lefthook` ever was installed.
```

- 來源 URL：
  - https://github.com/evilmartians/lefthook/blob/master/internal/templates/hook.tmpl [200]
  - https://github.com/evilmartians/lefthook/blob/master/docs/configuration/assert_lefthook_installed.md [200]
  - https://github.com/evilmartians/lefthook/blob/master/docs/usage/envs/LEFTHOOK.md [200]
  - https://github.com/evilmartians/lefthook/blob/master/docs/usage/envs/LEFTHOOK_EXCLUDE.md [200]
  - https://github.com/evilmartians/lefthook/blob/master/docs/configuration/skip.md [200]
  - https://evilmartians.github.io/lefthook/configuration/assert_lefthook_installed/ [200]
  - https://evilmartians.github.io/lefthook/usage/envs/LEFTHOOK/ [200]
  - https://evilmartians.github.io/lefthook/usage/envs/LEFTHOOK_EXCLUDE/ [200]
  - https://evilmartians.github.io/lefthook/configuration/skip/ [200]

# pre-commit

版本：pre-commit v3.x / v4.x（v4.1.0 / main）

1. 它產生／安裝的 hook 腳本，在找不到自己的執行檔時，是「擋下 commit」還是「安靜放行」？
- **擋下 commit**。
  - 機制：`pre-commit install` 安裝於 `.git/hooks/<hook>` 的腳本由 `pre_commit/resources/hook-tmpl` 產生。腳本先檢查安裝時記錄的 `$INSTALL_PYTHON` 是否可執行，若否則搜尋 PATH 中的 `pre-commit`。若皆找不到，輸出錯誤訊息 <code>\`pre-commit\` not found.  Did you forget to activate your virtualenv?</code> 並以 `exit 1` 退出，直接**擋下 commit**。
  - 原始碼片段（`pre_commit/resources/hook-tmpl`）：
```bash
if [ -x "$INSTALL_PYTHON" ]; then
    exec "$INSTALL_PYTHON" -mpre_commit "${ARGS[@]}"
elif command -v pre-commit > /dev/null; then
    exec pre-commit "${ARGS[@]}"
else
    echo '`pre-commit` not found.  Did you forget to activate your virtualenv?' 1>&2
    exit 1
fi
```

2. 它提供什麼跳過機制？
- Git 原生參數：`git commit -n` 或 `git commit --no-verify`
- 環境變數 `SKIP`：以逗號分隔指定要跳過的 hook id 或 alias（例如 `SKIP=flake8 git commit -m "foo"`）
- 環境變數 `PRE_COMMIT_ALLOW_NO_CONFIG=1`：當專案缺少 `.pre-commit-config.yaml` 時略過 pre-commit 執行（未設定此變數且缺少設定檔時，預設會報錯並 `exit 1` 擋下 commit）
- 安裝與全域參數 `--allow-missing-config`（或 `--skip-on-missing-config`）：允許在缺少設定檔時靜默放行（`pre-commit init-templatedir` 預設使用此選項）

3. 這個跳過機制的官方文件怎麼描述它的用途？
- 官方文件原文：
  - `SKIP` 環境變數：
```
Not all hooks are perfect so sometimes you may need to skip execution of one
or more hooks. pre-commit solves this by querying a `SKIP` environment
variable. The `SKIP` environment variable is a comma separated list of hook
ids. This allows you to skip a single hook instead of `--no-verify`ing the
entire commit.

```console
$ SKIP=flake8 git commit -m "foo"
```
```
  - `PRE_COMMIT_ALLOW_NO_CONFIG` 與 `--allow-missing-config`：
```
`pre-commit install`
--allow-missing-config: Hook scripts will permit a missing configuration file.

No .pre-commit-config.yaml file was found
- To temporarily silence this, run `PRE_COMMIT_ALLOW_NO_CONFIG=1 git ...`
- To permanently silence this, install pre-commit with the --allow-missing-config option
- To uninstall pre-commit run `pre-commit uninstall`
```
  - `--skip-on-missing-config`：
```
- `--skip-on-missing-config`: silently pass when a config is missing

--skip-on-missing-config is recommended here as arbitrary git repositories may not have a `.pre-commit-config.yaml`.
```

- 來源 URL：
  - https://github.com/pre-commit/pre-commit/blob/main/pre_commit/resources/hook-tmpl [200]
  - https://github.com/pre-commit/pre-commit/blob/main/pre_commit/commands/hook_impl.py [200]
  - https://github.com/pre-commit/pre-commit/blob/main/pre_commit/commands/run.py [200]
  - https://pre-commit.com/#temporarily-disabling-hooks [200]
  - https://pre-commit.com/#pre-commit-install [200]
  - https://pre-commit.com/#cli [200]
