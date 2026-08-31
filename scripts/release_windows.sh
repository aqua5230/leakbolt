#!/bin/sh
set -eu

repo_dir=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)
dist_dir="$repo_dir/dist"

mkdir -p "$dist_dir"
for arch in amd64 arm64; do
	(
		cd "$repo_dir/prototype"
		CGO_ENABLED=0 GOOS=windows GOARCH="$arch" go build -buildvcs=true -trimpath -o "$dist_dir/leakbolt-windows-$arch.exe" .
	)
done

(
	cd "$dist_dir"
	shasum -a 256 leakbolt-windows-amd64.exe leakbolt-windows-arm64.exe > SHA256SUMS-windows.txt
)
