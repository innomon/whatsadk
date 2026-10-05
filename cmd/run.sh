#!/usr/bin/env bash
# cmd/run.sh - Cross-platform launcher for WhatsADK binaries.
#
# Detects host operating system and hardware architecture, appends the
# appropriate platform extension to the command prefix, and executes the binary.
#
# Usage:
#   cmd/run.sh <command-prefix> [arguments...]
#
# Supported Platforms:
#   - Mac mini M4:              darwin/arm64 (_darwin_arm64)
#   - Raspberry Pi 4 & 5:       linux/arm64  (_linux_arm64)
#   - Ubuntu / Debian x86_64:   linux/amd64  (_linux_amd64)

set -e

if [ $# -lt 1 ]; then
    echo "Usage: $0 <command-prefix> [arguments...]"
    echo ""
    echo "Available commands:"
    echo "  gateway       - WhatsApp Multi-Device Gateway"
    echo "  waba-gateway  - WhatsApp Cloud API (WABA) Gateway"
    echo "  keygen        - Key Generation Utility"
    echo "  whatsadk-mcp  - Model Context Protocol (MCP) Server"
    echo "  simulator     - WhatsApp TUI Simulator"
    echo "  adksim        - ADK Agent TUI Simulator"
    echo "  dbutil        - Database Export/Import Utility"
    echo ""
    echo "Example:"
    echo "  $0 gateway -config config.yaml"
    echo "  $0 dbutil export -db data/p2p_num1.db -out dump.jsonl"
    exit 1
fi

CMD_PREFIX="$1"
shift

# Clean command prefix (strip any leading path)
CMD_NAME="$(basename "$CMD_PREFIX")"

# 1. Detect Operating System
UNAME_S="$(uname -s)"
case "$UNAME_S" in
    Darwin)
        GOOS="darwin"
        ;;
    Linux)
        GOOS="linux"
        ;;
    MINGW*|MSYS*|CYGWIN*)
        GOOS="windows"
        ;;
    *)
        GOOS="$(echo "$UNAME_S" | tr '[:upper:]' '[:lower:]')"
        ;;
esac

# 2. Detect Hardware Architecture
UNAME_M="$(uname -m)"
case "$UNAME_M" in
    x86_64|amd64)
        GOARCH="amd64"
        ;;
    aarch64|arm64)
        GOARCH="arm64"
        ;;
    armv7l|armhf)
        GOARCH="arm"
        ;;
    i386|i686)
        GOARCH="386"
        ;;
    *)
        GOARCH="$UNAME_M"
        ;;
esac

# Formulate platform extensions
EXT="_${GOOS}_${GOARCH}"
ALT_EXT="-${GOOS}-${GOARCH}"

# Locate directory roots
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." 2>/dev/null && pwd)"

SEARCH_DIRS=(
    "$ROOT_DIR/bin"
    "$SCRIPT_DIR/../bin"
    "$SCRIPT_DIR/bin"
    "$SCRIPT_DIR"
    "./bin"
    "."
)

# Potential binary name aliases
CMD_ALIASES=("$CMD_NAME")
if [ "$CMD_NAME" = "mcp" ]; then
    CMD_ALIASES+=("whatsadk-mcp")
elif [ "$CMD_NAME" = "whatsadk-mcp" ]; then
    CMD_ALIASES+=("mcp")
fi

FOUND_BIN=""

for dir in "${SEARCH_DIRS[@]}"; do
    if [ ! -d "$dir" ]; then
        continue
    fi
    for name in "${CMD_ALIASES[@]}"; do
        # 1. Check primary extension (_os_arch)
        if [ -x "$dir/${name}${EXT}" ]; then
            FOUND_BIN="$dir/${name}${EXT}"
            break 2
        fi
        # 2. Check alternate extension (-os-arch)
        if [ -x "$dir/${name}${ALT_EXT}" ]; then
            FOUND_BIN="$dir/${name}${ALT_EXT}"
            break 2
        fi
        # 3. Check plain unadorned binary
        if [ -x "$dir/${name}" ]; then
            FOUND_BIN="$dir/${name}"
            break 2
        fi
    done
done

if [ -z "$FOUND_BIN" ]; then
    echo "Error: Could not find executable for command '$CMD_NAME' on platform ${GOOS}/${GOARCH}." >&2
    echo "Expected one of:" >&2
    for name in "${CMD_ALIASES[@]}"; do
        echo "  - bin/${name}${EXT}" >&2
        echo "  - bin/${name}${ALT_EXT}" >&2
        echo "  - bin/${name}" >&2
    done
    echo "" >&2
    echo "To compile cross-platform binaries, run:" >&2
    echo "  make build-all" >&2
    exit 1
fi

exec "$FOUND_BIN" "$@"
