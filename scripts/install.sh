#!/bin/sh
set -e

REPO="donnycrash/clasp"
INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"

TAG=""
while [ $# -gt 0 ]; do
  case "$1" in
    --version) TAG="$2"; shift 2 ;;
    *)         echo "Unknown option: $1" >&2; exit 1 ;;
  esac
done

detect_os() {
  case "$(uname -s)" in
    Linux)  echo "linux" ;;
    Darwin) echo "darwin" ;;
    *)      echo "Unsupported OS: $(uname -s)" >&2; exit 1 ;;
  esac
}

detect_arch() {
  case "$(uname -m)" in
    x86_64|amd64)   echo "amd64" ;;
    arm64|aarch64)   echo "arm64" ;;
    *)               echo "Unsupported architecture: $(uname -m)" >&2; exit 1 ;;
  esac
}

OS="$(detect_os)"
ARCH="$(detect_arch)"

if [ -z "$TAG" ]; then
  echo "Fetching latest release..."
  TAG="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
    | grep '"tag_name"' \
    | cut -d'"' -f4)"
fi

echo "Fetching release ${TAG}..."
URL="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/tags/${TAG}" \
  | grep '"browser_download_url"' \
  | grep "_${OS}_${ARCH}\.tar\.gz" \
  | cut -d'"' -f4)"

if [ -z "$URL" ]; then
  echo "Error: could not find asset for ${OS}/${ARCH} in release ${TAG}" >&2
  exit 1
fi

ARCHIVE="${URL##*/}"
_v="${ARCHIVE%.tar.gz}"
_v="${_v#clasp_}"
VERSION="${_v%_${OS}_${ARCH}}"

TMPDIR="$(mktemp -d)"
trap 'rm -rf "$TMPDIR"' EXIT

echo "Downloading clasp ${VERSION} for ${OS}/${ARCH}..."
curl -fsSL -o "${TMPDIR}/${ARCHIVE}" "${URL}"

tar -xzf "${TMPDIR}/${ARCHIVE}" -C "${TMPDIR}"

mkdir -p "${INSTALL_DIR}"
cp "${TMPDIR}/clasp" "${INSTALL_DIR}/clasp"
chmod +x "${INSTALL_DIR}/clasp"

echo "clasp ${VERSION} installed to ${INSTALL_DIR}/clasp"
echo ""

case ":${PATH}:" in
  *:"${INSTALL_DIR}":*) ;;
  *)
    echo "Warning: ${INSTALL_DIR} is not in your PATH."
    echo "Add this to your shell rc file:"
    echo ""
    echo "  export PATH=\"${INSTALL_DIR}:\$PATH\""
    echo ""
    ;;
esac

# Set up the background schedule.
echo "Setting up background schedule..."
"${INSTALL_DIR}/clasp" install

echo ""
echo "Next steps:"
echo "  clasp auth login"
