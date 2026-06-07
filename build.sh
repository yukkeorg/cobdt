#!/usr/bin/env bash
set -euo pipefail

export PROGNAME=${PROGNAME:-cobdt}

export GOOS=${GOOS:-linux}
export GOARCH=${GOARCH:-amd64}
export RELEASE_TAG=${RELEASE_TAG:-local}
export PACKAGETYPE=${PACKAGETYPE:-tar.gz}

mkdir -p dist
binary="${PROGNAME}"
if [ "${GOOS}" = "windows" ]; then
    binary="${binary}.exe"
fi

# バージョン情報を ldflags で埋め込む（cmd/cobdt/main.go の var version/commit/date）。
# version はリリースタグ（release.yml が RELEASE_TAG に v1.0.0 等を渡す。ローカルは local）、
# commit は短縮 SHA、date はビルド日（UTC）。Git 情報が取れない場合は既定値にフォールバックする。
commit="$(git rev-parse --short HEAD 2>/dev/null || echo none)"
date="$(date -u +%Y-%m-%d)"
ldflags="-s -w"
ldflags="${ldflags} -X main.version=${RELEASE_TAG}"
ldflags="${ldflags} -X main.commit=${commit}"
ldflags="${ldflags} -X main.date=${date}"

go build -trimpath -ldflags="${ldflags}" -o "dist/${binary}" ./cmd/cobdt

asset_dir="${PROGNAME}_${RELEASE_TAG}_${GOOS}_${GOARCH}"
mkdir -p "dist/${asset_dir}"
cp "dist/${binary}" "dist/${asset_dir}/"

if [ "${PACKAGETYPE}" = "zip" ]; then
    ( cd dist; zip -r "${asset_dir}.zip" "${asset_dir}" )
else
    tar -C dist -czf "dist/${asset_dir}.tar.gz" "${asset_dir}"
fi

unset asset_dir
unset binary