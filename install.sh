#!/bin/sh
set -e

# Repository details
OWNER="SavNico"
REPO="just"

# Detect OS
OS="$(uname -s)"
case "${OS}" in
    Linux)  OS_NAME="Linux" ;;
    Darwin) OS_NAME="Darwin" ;;
    *)
        echo "Error: Operating system ${OS} is not supported."
        exit 1
        ;;
esac

# Detect Architecture
ARCH="$(uname -m)"
case "${ARCH}" in
    x86_64) ARCH_NAME="Amd64" ;;
    arm64|aarch64) ARCH_NAME="Arm64" ;;
    *)
        echo "Error: Architecture ${ARCH} is not supported."
        exit 1
        ;;
esac

echo "Detecting latest release..."
# Get latest release data from GitHub API
RELEASE_JSON=$(curl -s "https://api.github.com/repos/${OWNER}/${REPO}/releases/latest")
LATEST_TAG=$(echo "${RELEASE_JSON}" | grep '"tag_name":' | head -n 1 | sed -E 's/.*"([^"]+)".*/\1/')

if [ -z "${LATEST_TAG}" ]; then
    echo "Error: Could not retrieve latest release tag."
    exit 1
fi

echo "Latest release is ${LATEST_TAG}"

# Dynamically locate asset URL matching OS and Architecture
DOWNLOAD_URL=$(echo "${RELEASE_JSON}" | grep "browser_download_url" | grep -i "${OS_NAME}" | grep -i "${ARCH_NAME}" | head -n 1 | cut -d '"' -f 4)

if [ -z "${DOWNLOAD_URL}" ]; then
    # Fallback if asset pattern differs
    TARBALL="just_${LATEST_TAG}_${OS_NAME}_${ARCH_NAME}.tar.gz"
    DOWNLOAD_URL="https://github.com/SavNico/just/releases/download/${LATEST_TAG}/${TARBALL}"
else
    TARBALL=$(basename "${DOWNLOAD_URL}")
fi

echo "Downloading from ${DOWNLOAD_URL}..."
TMP_DIR=$(mktemp -d)
if ! curl -sSLf "${DOWNLOAD_URL}" -o "${TMP_DIR}/${TARBALL}"; then
    echo "Error: Failed to download release asset from ${DOWNLOAD_URL}."
    rm -rf "${TMP_DIR}"
    exit 1
fi

echo "Extracting..."
tar -xzf "${TMP_DIR}/${TARBALL}" -C "${TMP_DIR}"

# Determine install path
INSTALL_DIR="/usr/local/bin"
if [ ! -w "${INSTALL_DIR}" ]; then
    echo "Requires sudo privileges to install to ${INSTALL_DIR}"
    sudo mv "${TMP_DIR}/just" "${INSTALL_DIR}/just"
else
    mv "${TMP_DIR}/just" "${INSTALL_DIR}/just"
fi

chmod +x "${INSTALL_DIR}/just"
rm -rf "${TMP_DIR}"

echo "Success! 'just' has been installed to ${INSTALL_DIR}/just"
"${INSTALL_DIR}/just" -v
