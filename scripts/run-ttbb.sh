#!/bin/sh
export GOMEMLIMIT=350MiB
touch /tmp/tokentimeboost.heartbeat

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR" || exit 1

BINARY_BASE="TokenTimeBoostBot_freebsd_amd64"

# Find all versioned binaries matching TokenTimeBoostBot_freebsd_amd64_v*
# sorted by modification time (newest first).
LATEST_BIN=$(ls -1t "${SCRIPT_DIR}/${BINARY_BASE}_v"* 2>/dev/null | head -n 1)

# Fallback to unversioned binary if no versioned binary is found
if [ -z "$LATEST_BIN" ] || [ ! -x "$LATEST_BIN" ]; then
    if [ -x "${SCRIPT_DIR}/${BINARY_BASE}" ]; then
        LATEST_BIN="${SCRIPT_DIR}/${BINARY_BASE}"
    fi
fi

if [ -z "$LATEST_BIN" ] || [ ! -x "$LATEST_BIN" ]; then
    echo "$(date -u '+%Y-%m-%d %H:%M:%SZ'): Error: No executable ${BINARY_BASE} found in ${SCRIPT_DIR}" >&2
    exit 1
fi

# Purge older versioned binaries, keeping the most recent 4 versions.
# Only versioned binaries (TokenTimeBoostBot_freebsd_amd64_v*) are purged.
ls -1t "${SCRIPT_DIR}/${BINARY_BASE}_v"* 2>/dev/null | tail -n +5 | while IFS= read -r old_bin; do
    if [ -n "$old_bin" ] && [ -f "$old_bin" ] && [ "$old_bin" != "$LATEST_BIN" ]; then
        echo "$(date -u '+%Y-%m-%d %H:%M:%SZ'): Purging old bot version: $old_bin"
        rm -f "$old_bin"
    fi
done

echo "$(date -u '+%Y-%m-%d %H:%M:%SZ'): Launching $(basename "$LATEST_BIN")..."
exec "$LATEST_BIN"
