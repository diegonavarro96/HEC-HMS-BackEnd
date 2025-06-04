#!/bin/bash
# HMSHistoricalBatch.sh - Linux/WSL version
# This script runs HEC-HMS historical simulation
# Usage: HMSHistoricalBatch.sh <script_path> <hms_models_dir>

# Set HEC-HMS home directory
HMS_HOME="/opt/hms"

# Check if HMS directory exists
if [ ! -d "$HMS_HOME" ]; then
    echo "Error: HEC-HMS directory not found at $HMS_HOME"
    exit 1
fi

# Get arguments
SCRIPT_PATH="$1"
HMS_MODELS_DIR="$2"

# Check if required arguments are provided
if [ -z "$SCRIPT_PATH" ] || [ -z "$HMS_MODELS_DIR" ]; then
    echo "Error: Missing required arguments"
    echo "Usage: $0 <script_path> <hms_models_dir>"
    exit 1
fi

# Export HMS_MODELS_DIR for the script to use
export HMS_MODELS_DIR="$HMS_MODELS_DIR"
echo "HMS_MODELS_DIR set to: $HMS_MODELS_DIR"

# Get the script directory (for relative paths)
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
HMS_SCRIPTS_DIR="$(dirname "$SCRIPT_DIR")"

# Run HMS with the historical script
echo "Running HEC-HMS Historical simulation..."
echo "Script path: $SCRIPT_PATH"
cd "$HMS_SCRIPTS_DIR"

# Execute HEC-HMS with the script
"$HMS_HOME/hec-hms.sh" -s "$SCRIPT_PATH"

# Check if the command was successful
if [ $? -eq 0 ]; then
    echo "HEC-HMS Historical simulation completed successfully"
    exit 0
else
    echo "Error: HEC-HMS Historical simulation failed"
    exit 1
fi