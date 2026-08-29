#!/usr/bin/env bash
# 在 Linux 容器裡跑相容矩陣。macOS 的 11/11 不能當跨平台憑證。
#
# 用交叉編譯而不是在容器裡編：我們零 cgo 依賴，靜態連結，
# 這樣不必拉幾百 MB 的 golang 映像，本機有 ubuntu 就能跑。
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"

ARCH="$(docker run --rm ubuntu:24.04 uname -m)"
case "$ARCH" in
  aarch64) GOARCH=arm64; GLARCH=arm64 ;;
  x86_64)  GOARCH=amd64; GLARCH=x64 ;;
  *) echo "未知的容器架構：$ARCH" >&2; exit 2 ;;
esac
echo "容器架構 $ARCH → GOARCH=$GOARCH，gitleaks $GLARCH"

BIN="$(mktemp)"
( cd "$ROOT/prototype" && GOOS=linux GOARCH="$GOARCH" CGO_ENABLED=0 go build -o "$BIN" . )
chmod +x "$BIN"

docker run --rm \
  -v "$ROOT/scripts":/scripts:ro \
  -v "$BIN":/usr/local/bin/leakbolt:ro \
  -e GLARCH="$GLARCH" \
  ubuntu:24.04 bash -c '
set -e
export DEBIAN_FRONTEND=noninteractive
apt-get update -qq >/dev/null 2>&1
apt-get install -y -qq git curl ca-certificates nodejs npm python3-pip >/dev/null 2>&1
curl -fsSL "https://github.com/gitleaks/gitleaks/releases/download/v8.30.1/gitleaks_8.30.1_linux_${GLARCH}.tar.gz" \
  | tar -xz -C /usr/local/bin gitleaks
npm install -g --silent lefthook >/dev/null 2>&1 || true
pip3 install --quiet --break-system-packages pre-commit >/dev/null 2>&1 || true
echo "=== 環境 ==="
uname -srm; git --version; gitleaks version
node --version 2>/dev/null || echo "node 缺"
lefthook version 2>/dev/null || echo "lefthook 缺"
pre-commit --version 2>/dev/null || echo "pre-commit 缺"
git config --global user.email t@t
git config --global user.name t
git config --global init.defaultBranch main
echo "=== 相容矩陣（Linux）==="
LEAKBOLT_BIN=/usr/local/bin/leakbolt bash /scripts/hook_matrix.sh
'
rm -f "$BIN"
