#!/usr/bin/env bash
# scripts/build-cross.sh - Compiles WhatsADK binaries for target platforms:
#   - Mac mini M4:              darwin/arm64 (_darwin_arm64)
#   - Raspberry Pi 4 & 5:       linux/arm64  (_linux_arm64)
#   - Ubuntu / Debian x86_64:   linux/amd64  (_linux_amd64)

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
BIN_DIR="$ROOT_DIR/bin"

mkdir -p "$BIN_DIR"

PLATFORMS=(
    "darwin/arm64"
    "linux/arm64"
    "linux/amd64"
)

COMMANDS=(
    "gateway:cmd/gateway"
    "waba-gateway:cmd/waba-gateway"
    "keygen:cmd/keygen"
    "whatsadk-mcp:cmd/mcp"
    "simulator:cmd/simulator"
    "adksim:cmd/adksim"
    "dbutil:cmd/dbutil"
    "tui:cmd/tui"
)

echo "=================================================="
echo " Building Cross-Platform WhatsADK Binaries"
echo "=================================================="

for platform in "${PLATFORMS[@]}"; do
    GOOS="${platform%/*}"
    GOARCH="${platform#*/}"
    EXT="_${GOOS}_${GOARCH}"
    ALT_EXT="-${GOOS}-${GOARCH}"

    echo ""
    echo ">> Platform: $GOOS/$GOARCH"

    for entry in "${COMMANDS[@]}"; do
        NAME="${entry%:*}"
        PKG="./${entry#*:}"
        OUT="$BIN_DIR/${NAME}${EXT}"

        printf "  -> Compiling %-14s to %s..." "$NAME" "${NAME}${EXT}"
        CGO_ENABLED=0 GOOS="$GOOS" GOARCH="$GOARCH" go build -trimpath -ldflags="-s -w" -o "$OUT" "$PKG"
        echo " done"

        # Create alternate hyphen symlink (e.g. gateway-linux-amd64 -> gateway_linux_amd64)
        (cd "$BIN_DIR" && ln -sf "${NAME}${EXT}" "${NAME}${ALT_EXT}")

        # If whatsadk-mcp, also create mcp alias symlinks
        if [ "$NAME" = "whatsadk-mcp" ]; then
            (cd "$BIN_DIR" && ln -sf "whatsadk-mcp${EXT}" "mcp${EXT}")
            (cd "$BIN_DIR" && ln -sf "whatsadk-mcp${EXT}" "mcp${ALT_EXT}")
        fi
    done
done

echo ""
echo "=================================================="
echo " All cross-platform binaries built successfully!"
echo " Output directory: $BIN_DIR"
echo "=================================================="
