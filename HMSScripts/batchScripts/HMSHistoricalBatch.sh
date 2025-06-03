#!/bin/bash
# HMSHistoricalBatch.sh - Linux/WSL version
# This script runs HEC-HMS historical simulation

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

# Run HMS with the historical script
echo "Running HEC-HMS Historical simulation..."
cd "$HMS_SCRIPTS_DIR"

# Execute HEC-HMS with the script
# Create symlink to avoid issues with spaces in path
TEMP_SCRIPT="/tmp/computeHistorical.script"
ln -sf "${HMS_SCRIPTS_DIR}/computeHistorical.script" "$TEMP_SCRIPT"

echo "Script path: $TEMP_SCRIPT"
echo "Executing: $HMS_HOME/hec-hms.sh -s $TEMP_SCRIPT"
"$HMS_HOME/hec-hms.sh" -s "$TEMP_SCRIPT"

# Check if the command was successful
if [ $? -eq 0 ]; then
    echo "HEC-HMS Historical simulation completed successfully"
    exit 0
else
    echo "Error: HEC-HMS Historical simulation failed"
    exit 1
fi