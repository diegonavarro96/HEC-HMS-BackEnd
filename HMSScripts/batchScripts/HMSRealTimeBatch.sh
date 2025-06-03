#!/bin/bash
# HMSRealTimeBatch.sh - Linux/WSL version
# This script runs HEC-HMS real-time simulation

# Set HEC-HMS home directory
HMS_HOME="/opt/hms"

# Check if HMS directory exists
if [ ! -d "$HMS_HOME" ]; then
    echo "Error: HEC-HMS directory not found at $HMS_HOME"
    exit 1
fi

# Get the script directory (for relative paths)
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
HMS_SCRIPTS_DIR="$(dirname "$SCRIPT_DIR")"

# Run HMS with the real-time script
echo "Running HEC-HMS Real-Time simulation..."
cd "$HMS_SCRIPTS_DIR"

# Execute HEC-HMS with the script
"$HMS_HOME/bin/hec-hms.sh" -s "$HMS_SCRIPTS_DIR/computeRealTime.script"

# Check if the command was successful
if [ $? -eq 0 ]; then
    echo "HEC-HMS Real-Time simulation completed successfully"
    exit 0
else
    echo "Error: HEC-HMS Real-Time simulation failed"
    exit 1
fi