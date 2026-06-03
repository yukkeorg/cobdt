#!/usr/bin/env bash
set -euo pipefail

export PROGNAME=${PROGNAME:-cdm}

export GOOS=${GOOS:-linux}
export GOARCH=${GOARCH:-amd64}
export RELEASE_TAG=${RELEASE_TAG:-local}
export PACKAGETYPE=${PACKAGETYPE:-tar.gz}

mkdir -p dist
binary="${PROGNAME}"
if [ "${GOOS}" = "windows" ]; then
    binary="${binary}.exe"
fi

go build -trimpath -ldflags="-s -w" -o "dist/${binary}" ./cmd/cdm

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