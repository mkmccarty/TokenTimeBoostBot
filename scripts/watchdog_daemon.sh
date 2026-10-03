#!/bin/sh

# Config
BINARY_NAME="TokenTimeBoostBot"
HEARTBEAT="/tmp/tokentimeboost.heartbeat"
CHECK_INTERVAL=60 # Seconds between checks

echo "Watchdog started for $BINARY_NAME"

while true; do
    # Find the heartbeat file if it's older than 5 minutes
    STALE=$(find "$HEARTBEAT" -mmin +5 2>/dev/null)

    if [ -n "$STALE" ]; then
        echo "$(date): Heartbeat stale! Killing $BINARY_NAME"
        pkill -9 -f "$BINARY_NAME"
    fi

    sleep "$CHECK_INTERVAL"
done
