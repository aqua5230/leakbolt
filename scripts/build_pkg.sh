#!/bin/sh
set -eu

repo_dir=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)
dist_dir="$repo_dir/dist"
log_file="$dist_dir/pkg-build.log"
work_dir=""
status_line="STOPPED: 原因＝建置套件時發生未預期錯誤"

cleanup() {
	status=$?
	if [ -n "$work_dir" ] && [ -d "$work_dir" ]; then
		/bin/rm -rf "$work_dir"
	fi
	if [ "$status" -eq 0 ]; then
		/bin/rm -f "$log_file"
		printf 'DONE\n'
	else
		printf '%s\n' "$status_line"
	fi
}

fail() {
	status_line="STOPPED: 原因＝$1"
	exit 1
}

step() {
	status_line="STOPPED: 原因＝$1；詳見 $log_file"
}

require_universal() {
	binary=$1
	label=$2
	archs=$(/usr/bin/lipo -archs "$binary" 2>>"$log_file") || fail "無法檢查 $label 的晶片架構；詳見 $log_file"
	case " $archs " in
		*" arm64 "*) ;;
		*) fail "$label 缺少 arm64 架構" ;;
	esac
	case " $archs " in
		*" x86_64 "*) ;;
		*) fail "$label 缺少 x86_64 架構" ;;
	esac
}

verify_sha256() {
	archive=$1
	expected=$2
	label=$3
	actual=$(/usr/bin/shasum -a 256 "$archive" | /usr/bin/awk '{print $1}')
	if [ "$actual" != "$expected" ]; then
		fail "$label SHA256 驗證失敗（預期 $expected，實際 $actual）"
	fi
}

trap cleanup 0
trap 'exit 1' HUP INT TERM

/bin/mkdir -p "$dist_dir"
: >"$log_file"

for command_name in go curl tar lipo pkgbuild productbuild pkgutil shasum; do
	command -v "$command_name" >/dev/null 2>&1 || fail "缺少必要工具：$command_name"
done

version=$(/usr/bin/awk -F'"' '/^const version = "/ { print $2 }' "$repo_dir/prototype/version.go")
[ -n "$version" ] || fail "無法從 prototype/version.go 讀取 LeakBolt 版本"

gitleaks_version=$(/usr/bin/awk -F'"' '/^const requiredGitleaksVersion = "/ { print $2 }' "$repo_dir/prototype/scan.go")
[ "$gitleaks_version" = "8.30.1" ] || fail "prototype/scan.go 鎖定的 gitleaks 版本不是 8.30.1"

work_dir=$(/usr/bin/mktemp -d "${TMPDIR:-/tmp}/leakbolt-pkg.XXXXXX")
payload_dir="$work_dir/payload"
download_dir="$work_dir/downloads"
component_pkg="$work_dir/LeakBolt-component.pkg"
unsigned_or_signed_pkg="$work_dir/LeakBolt-$version.pkg"
output_pkg="$dist_dir/LeakBolt-$version.pkg"

/bin/mkdir -p \
	"$payload_dir/usr/local/bin" \
	"$payload_dir/usr/local/leakbolt/bin" \
	"$download_dir/arm64" \
	"$download_dir/x64" \
	"$work_dir/go-cache"

step "LeakBolt 的兩種晶片版本建置失敗"
(
	cd "$repo_dir/prototype"
	GOCACHE="$work_dir/go-cache" CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -buildvcs=true -trimpath -o "$work_dir/leakbolt-amd64" .
	GOCACHE="$work_dir/go-cache" CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -buildvcs=true -trimpath -o "$work_dir/leakbolt-arm64" .
) >>"$log_file" 2>&1

step "LeakBolt universal binary 合成失敗"
/usr/bin/lipo -create -output "$payload_dir/usr/local/bin/leakbolt" \
	"$work_dir/leakbolt-amd64" "$work_dir/leakbolt-arm64" >>"$log_file" 2>&1
require_universal "$payload_dir/usr/local/bin/leakbolt" "leakbolt"

arm64_archive="gitleaks_${gitleaks_version}_darwin_arm64.tar.gz"
x64_archive="gitleaks_${gitleaks_version}_darwin_x64.tar.gz"
release_url="https://github.com/gitleaks/gitleaks/releases/download/v${gitleaks_version}"

step "無法從官方 GitHub Release 下載 gitleaks"
/usr/bin/curl -fsSL --retry 3 --connect-timeout 20 \
	-o "$download_dir/$arm64_archive" "$release_url/$arm64_archive" >>"$log_file" 2>&1
/usr/bin/curl -fsSL --retry 3 --connect-timeout 20 \
	-o "$download_dir/$x64_archive" "$release_url/$x64_archive" >>"$log_file" 2>&1

# 官方值：https://github.com/gitleaks/gitleaks/releases/tag/v8.30.1
verify_sha256 "$download_dir/$arm64_archive" \
	"b40ab0ae55c505963e365f271a8d3846efbc170aa17f2607f13df610a9aeb6a5" \
	"gitleaks darwin arm64 壓縮檔"
verify_sha256 "$download_dir/$x64_archive" \
	"dfe101a4db2255fc85120ac7f3d25e4342c3c20cf749f2c20a18081af1952709" \
	"gitleaks darwin x64 壓縮檔"

step "gitleaks 壓縮檔解開失敗"
/usr/bin/tar -xzf "$download_dir/$arm64_archive" -C "$download_dir/arm64" >>"$log_file" 2>&1
/usr/bin/tar -xzf "$download_dir/$x64_archive" -C "$download_dir/x64" >>"$log_file" 2>&1
[ -x "$download_dir/arm64/gitleaks" ] || fail "arm64 壓縮檔內找不到可執行的 gitleaks"
[ -x "$download_dir/x64/gitleaks" ] || fail "x64 壓縮檔內找不到可執行的 gitleaks"

step "gitleaks universal binary 合成失敗"
/usr/bin/lipo -create -output "$payload_dir/usr/local/leakbolt/bin/gitleaks" \
	"$download_dir/x64/gitleaks" "$download_dir/arm64/gitleaks" >>"$log_file" 2>&1
require_universal "$payload_dir/usr/local/leakbolt/bin/gitleaks" "gitleaks"

/usr/bin/install -m 0755 "$repo_dir/scripts/uninstall_pkg.sh" \
	"$payload_dir/usr/local/leakbolt/uninstall.sh"

# 清掉延伸屬性，否則 pkgbuild 會把 ._ 開頭的中繼資料檔一起打包進去
/usr/bin/xattr -rc "$payload_dir"

step "pkgbuild 建置失敗"
/usr/bin/pkgbuild \
	--root "$payload_dir" \
	--scripts "$repo_dir/scripts/pkg/scripts" \
	--identifier "com.leakbolt.pkg" \
	--version "$version" \
	--install-location / \
	"$component_pkg" >>"$log_file" 2>&1

step "productbuild 建置失敗"
if [ -n "${LEAKBOLT_SIGN_IDENTITY:-}" ]; then
	/usr/bin/productbuild \
		--distribution "$repo_dir/scripts/pkg/Distribution.xml" \
		--package-path "$work_dir" \
		--resources "$repo_dir/scripts/pkg/resources" \
		--sign "$LEAKBOLT_SIGN_IDENTITY" \
		"$unsigned_or_signed_pkg" >>"$log_file" 2>&1
else
	/usr/bin/productbuild \
		--distribution "$repo_dir/scripts/pkg/Distribution.xml" \
		--package-path "$work_dir" \
		--resources "$repo_dir/scripts/pkg/resources" \
		"$unsigned_or_signed_pkg" >>"$log_file" 2>&1
fi

step "建置後套件檢查失敗"
/usr/sbin/pkgutil --check-signature "$unsigned_or_signed_pkg" >>"$log_file" 2>&1 || true
/bin/mv -f "$unsigned_or_signed_pkg" "$output_pkg"
