#!/bin/sh

set -eu

REPO="${VOIDNET_REPO:-JimmyMcBride/voidnet}"
RELEASE_BASE_URL="${VOIDNET_RELEASE_BASE_URL:-https://github.com/${REPO}/releases}"
INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"
VERSION="${VERSION:-latest}"
BIN_NAME="voidnet"

say() {
  printf '%s\n' "$*"
}

fail() {
  say "voidnet install: $*" >&2
  exit 1
}

need_cmd() {
  command -v "$1" >/dev/null 2>&1 || fail "missing required command: $1"
}

download() {
  url="$1"
  output="$2"
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL "$url" -o "$output"
    return
  fi
  if command -v wget >/dev/null 2>&1; then
    wget -qO "$output" "$url"
    return
  fi
  fail "install requires curl or wget"
}

detect_os() {
  case "$(uname -s)" in
    Linux) echo "linux" ;;
    Darwin) echo "darwin" ;;
    CYGWIN*|MINGW*|MSYS*) echo "windows" ;;
    *) fail "unsupported operating system: $(uname -s)" ;;
  esac
}

detect_arch() {
  case "$(uname -m)" in
    x86_64|amd64) echo "amd64" ;;
    arm64|aarch64) echo "arm64" ;;
    *) fail "unsupported architecture: $(uname -m)" ;;
  esac
}

linux_audio_hint() {
  if command -v apt-get >/dev/null 2>&1; then
    echo "sudo apt-get install libasound2"
    return
  fi
  if command -v dnf >/dev/null 2>&1; then
    echo "sudo dnf install alsa-lib"
    return
  fi
  if command -v yum >/dev/null 2>&1; then
    echo "sudo yum install alsa-lib"
    return
  fi
  if command -v pacman >/dev/null 2>&1; then
    echo "sudo pacman -S alsa-lib"
    return
  fi
  if command -v zypper >/dev/null 2>&1; then
    echo "sudo zypper install alsa-lib"
    return
  fi
  if command -v apk >/dev/null 2>&1; then
    echo "sudo apk add alsa-lib"
    return
  fi
  echo "install your distro package that provides libasound.so.2"
}

has_libasound() {
  if command -v ldconfig >/dev/null 2>&1 && ldconfig -p 2>/dev/null | grep -q 'libasound\.so\.2'; then
    return 0
  fi

  for path in \
    /usr/lib/libasound.so.2 \
    /usr/lib64/libasound.so.2 \
    /lib/libasound.so.2 \
    /lib64/libasound.so.2 \
    /usr/lib/x86_64-linux-gnu/libasound.so.2 \
    /lib/x86_64-linux-gnu/libasound.so.2
  do
    if [ -e "$path" ]; then
      return 0
    fi
  done
  return 1
}

resolve_asset() {
  os="$1"
  arch="$2"

  case "${os}-${arch}" in
    linux-amd64) echo "voidnet-linux-amd64.tar.gz" ;;
    darwin-amd64) echo "voidnet-darwin-amd64.tar.gz" ;;
    darwin-arm64) echo "voidnet-darwin-arm64.tar.gz" ;;
    windows-amd64) fail "use PowerShell with https://voidnet.jimmymcbride.dev/install.ps1 on Windows" ;;
    *) fail "unsupported target: ${os}-${arch}" ;;
  esac
}

release_asset_url() {
  asset="$1"
  if [ "$VERSION" = "latest" ]; then
    printf '%s/latest/download/%s' "$RELEASE_BASE_URL" "$asset"
    return
  fi
  printf '%s/download/%s/%s' "$RELEASE_BASE_URL" "$VERSION" "$asset"
}

verify_checksum() {
  archive="$1"
  asset="$2"
  checksums_file="$3"

  expected="$(awk -v name="$asset" '$2 == name { print $1 }' "$checksums_file")"
  if [ -z "$expected" ]; then
    fail "checksum for ${asset} not found"
  fi

  if command -v sha256sum >/dev/null 2>&1; then
    actual="$(sha256sum "$archive" | awk '{print $1}')"
  elif command -v shasum >/dev/null 2>&1; then
    actual="$(shasum -a 256 "$archive" | awk '{print $1}')"
  else
    say "warning: sha256sum/shasum not found; skipping checksum verification"
    return
  fi

  if [ "$actual" != "$expected" ]; then
    fail "checksum mismatch for ${asset}"
  fi
}

ensure_install_dir() {
  mkdir -p "$INSTALL_DIR"
}

ensure_on_path() {
  case ":$PATH:" in
    *":$INSTALL_DIR:"*) return 0 ;;
  esac

  line='export PATH="$HOME/.local/bin:$PATH"'
  shell_name="$(basename "${SHELL:-}")"
  rc_file=""

  if [ "$INSTALL_DIR" != "$HOME/.local/bin" ]; then
    line="export PATH=\"$INSTALL_DIR:\$PATH\""
  fi

  case "$shell_name" in
    zsh) rc_file="$HOME/.zshrc" ;;
    bash) rc_file="$HOME/.bashrc" ;;
    *) rc_file="$HOME/.profile" ;;
  esac

  if [ -f "$rc_file" ] && grep -F "$line" "$rc_file" >/dev/null 2>&1; then
    return 0
  fi

  printf '\n%s\n' "$line" >>"$rc_file"
  say "added ${INSTALL_DIR} to PATH in ${rc_file}"
}

need_cmd uname
need_cmd tar
need_cmd mktemp

os="$(detect_os)"
arch="$(detect_arch)"
asset="$(resolve_asset "$os" "$arch")"

if [ "$os" = "linux" ] && ! has_libasound; then
  fail "Voidnet needs libasound.so.2 on Linux. Install it first with: $(linux_audio_hint)"
fi

tmp_dir="$(mktemp -d)"
trap 'rm -rf "$tmp_dir"' EXIT INT TERM

archive_path="${tmp_dir}/${asset}"
checksums_path="${tmp_dir}/voidnet-checksums.txt"

say "downloading ${asset} from ${REPO} (${VERSION})"
download "$(release_asset_url "$asset")" "$archive_path"

if download "$(release_asset_url "voidnet-checksums.txt")" "$checksums_path"; then
  verify_checksum "$archive_path" "$asset" "$checksums_path"
else
  say "warning: could not download checksum file; skipping verification"
fi

tar -xzf "$archive_path" -C "$tmp_dir"
ensure_install_dir
install -m 0755 "${tmp_dir}/${BIN_NAME}" "${INSTALL_DIR}/${BIN_NAME}"
ensure_on_path

say "installed ${BIN_NAME} to ${INSTALL_DIR}/${BIN_NAME}"
say "run '${BIN_NAME} -version' to verify or start the game with '${BIN_NAME}'"
