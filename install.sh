#!/bin/sh
# cobdt インストールスクリプト
#
# 使い方:
#   curl -sSL https://raw.githubusercontent.com/yukkeorg/cobdt/main/install.sh | sh
#
# 環境変数で挙動を変更できる:
#   COBDT_VERSION      インストールするバージョン (例: v1.0.0)。未指定なら latest。
#   COBDT_INSTALL_DIR  インストール先ディレクトリ。未指定なら自動判定。
#
set -eu

REPO="yukkeorg/cobdt"
PROGNAME="cobdt"

# --- ログ出力 ----------------------------------------------------------------

info() {
    printf '\033[32m==>\033[0m %s\n' "$1"
}

err() {
    printf '\033[31mError:\033[0m %s\n' "$1" >&2
    exit 1
}

# --- 前提コマンドの確認 ------------------------------------------------------

need_cmd() {
    if ! command -v "$1" >/dev/null 2>&1; then
        err "Required command '$1' not found. Please install it and try again."
    fi
}

# ダウンローダの判定 (curl 優先、なければ wget)
if command -v curl >/dev/null 2>&1; then
    DOWNLOAD="curl -sSL -o"
    DOWNLOAD_STDOUT="curl -sSL"
elif command -v wget >/dev/null 2>&1; then
    DOWNLOAD="wget -qO"
    DOWNLOAD_STDOUT="wget -qO -"
else
    err "curl or wget is required."
fi

need_cmd tar
need_cmd uname

# --- OS / アーキテクチャの判定 -----------------------------------------------

detect_os() {
    os="$(uname -s)"
    case "$os" in
        Linux)  echo "linux" ;;
        Darwin) echo "darwin" ;;
        *)      err "Unsupported OS: $os" ;;
    esac
}

detect_arch() {
    arch="$(uname -m)"
    case "$arch" in
        x86_64 | amd64)        echo "amd64" ;;
        aarch64 | arm64)       echo "arm64" ;;
        *)                     err "Unsupported architecture: $arch" ;;
    esac
}

# --- 最新バージョンの取得 ----------------------------------------------------

latest_version() {
    # GitHub の latest リリースへのリダイレクト先 URL からタグを取り出す
    api_url="https://api.github.com/repos/${REPO}/releases/latest"
    tag="$($DOWNLOAD_STDOUT "$api_url" | grep '"tag_name"' | head -n1 | cut -d'"' -f4)"
    if [ -z "$tag" ]; then
        err "Failed to fetch the latest version. Please specify COBDT_VERSION explicitly."
    fi
    echo "$tag"
}

# --- インストール先の決定 ----------------------------------------------------

choose_install_dir() {
    if [ -n "${COBDT_INSTALL_DIR:-}" ]; then
        echo "$COBDT_INSTALL_DIR"
        return
    fi
    # 書き込み可能なら /usr/local/bin、なければ ~/.local/bin
    if [ -w /usr/local/bin ] 2>/dev/null; then
        echo "/usr/local/bin"
    elif [ "$(id -u)" = "0" ]; then
        echo "/usr/local/bin"
    else
        echo "${HOME}/.local/bin"
    fi
}

# --- メイン処理 --------------------------------------------------------------

main() {
    OS="$(detect_os)"
    ARCH="$(detect_arch)"

    VERSION="${COBDT_VERSION:-}"
    if [ -z "$VERSION" ]; then
        info "Fetching the latest version..."
        VERSION="$(latest_version)"
    fi
    info "Installing version ${VERSION} (${OS}/${ARCH})"

    asset="${PROGNAME}_${VERSION}_${OS}_${ARCH}.tar.gz"
    base_url="https://github.com/${REPO}/releases/download/${VERSION}"
    asset_url="${base_url}/${asset}"
    checksums_url="${base_url}/checksums.txt"

    # 一時ディレクトリの作成と後始末
    tmpdir="$(mktemp -d)"
    trap 'rm -rf "$tmpdir"' EXIT INT TERM

    info "Downloading: ${asset_url}"
    if ! $DOWNLOAD "${tmpdir}/${asset}" "$asset_url"; then
        err "Download failed: ${asset_url}"
    fi

    # チェックサム検証 (取得できた場合のみ)
    if $DOWNLOAD "${tmpdir}/checksums.txt" "$checksums_url" 2>/dev/null \
        && [ -s "${tmpdir}/checksums.txt" ]; then
        if command -v sha256sum >/dev/null 2>&1; then
            sha_cmd="sha256sum"
        elif command -v shasum >/dev/null 2>&1; then
            sha_cmd="shasum -a 256"
        else
            sha_cmd=""
        fi
        if [ -n "$sha_cmd" ]; then
            expected="$(grep " ${asset}\$" "${tmpdir}/checksums.txt" | awk '{print $1}')"
            if [ -n "$expected" ]; then
                actual="$($sha_cmd "${tmpdir}/${asset}" | awk '{print $1}')"
                if [ "$expected" != "$actual" ]; then
                    err "Checksum mismatch (expected: ${expected}, actual: ${actual})"
                fi
                info "Checksum verified"
            fi
        fi
    fi

    info "Extracting..."
    tar -C "$tmpdir" -xzf "${tmpdir}/${asset}"

    # アーカイブ内のバイナリを探す (cobdt_<TAG>_<OS>_<ARCH>/cobdt)
    binary="$(find "$tmpdir" -type f -name "$PROGNAME" | head -n1)"
    if [ -z "$binary" ]; then
        err "Could not find the ${PROGNAME} binary in the archive."
    fi

    INSTALL_DIR="$(choose_install_dir)"
    mkdir -p "$INSTALL_DIR"

    dest="${INSTALL_DIR}/${PROGNAME}"
    info "Installing to: ${dest}"
    if [ -w "$INSTALL_DIR" ]; then
        install -m 0755 "$binary" "$dest" 2>/dev/null || {
            cp "$binary" "$dest"
            chmod 0755 "$dest"
        }
    elif command -v sudo >/dev/null 2>&1; then
        info "Using sudo to write to ${INSTALL_DIR}"
        sudo install -m 0755 "$binary" "$dest"
    else
        err "Cannot write to ${INSTALL_DIR}. Specify another location with COBDT_INSTALL_DIR."
    fi

    info "Installation complete: ${dest}"

    # PATH に含まれているか確認
    case ":${PATH}:" in
        *":${INSTALL_DIR}:"*) ;;
        *)
            printf '\033[33mWarning:\033[0m %s is not in your PATH.\n' "$INSTALL_DIR"
            printf '         Add the following line to your shell config file:\n'
            printf '           export PATH="%s:$PATH"\n' "$INSTALL_DIR"
            ;;
    esac

    "$dest" --version 2>/dev/null || "$dest" version 2>/dev/null || true
}

main
