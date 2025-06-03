#!/bin/bash
# MergeGRIBFilesRealTime.sh - Linux/WSL version
# Script to merge GRIB files using Jython and HEC-Vortex libraries

# Set environment variables
JYTHON_JAR="/opt/jython.jar"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
JYTHON_SCRIPTS_DIR="$(dirname "$SCRIPT_DIR")"
JYTHON_SCRIPT="$JYTHON_SCRIPTS_DIR/MergeGRIBFilesRealTimeJython.py"

# HEC-HMS libraries needed for Jython scripts
HMS_HOME="/opt/hms"
HMS_LIB="$HMS_HOME/lib"

# Check if Jython JAR exists
if [ ! -f "$JYTHON_JAR" ]; then
    echo "Error: Jython JAR not found at $JYTHON_JAR"
    echo "Please install it with: wget https://repo1.maven.org/maven2/org/python/jython-standalone/2.7.3/jython-standalone-2.7.3.jar -O /opt/jython.jar"
    exit 1
fi

# Check if script exists
if [ ! -f "$JYTHON_SCRIPT" ]; then
    echo "Error: Jython script not found at $JYTHON_SCRIPT"
    exit 1
fi

# Get arguments
GRIB_DIR="$1"
START_DATE="$2"
END_DATE="$3"

# Validate arguments
if [ -z "$GRIB_DIR" ] || [ -z "$START_DATE" ] || [ -z "$END_DATE" ]; then
    echo "Usage: $0 <grib_directory> <start_date> <end_date>"
    echo "Example: $0 /path/to/grib/files 2024-01-01 2024-01-31"
    exit 1
fi

# Set Java memory options
export JAVA_OPTS="-Xmx4096m -Xms512m"

# Build classpath with HEC libraries
CLASSPATH="$JYTHON_JAR"
for jar in "$HMS_LIB"/*.jar; do
    CLASSPATH="$CLASSPATH:$jar"
done

echo "Running Jython script to merge GRIB files..."
echo "GRIB Directory: $GRIB_DIR"
echo "Date Range: $START_DATE to $END_DATE"

# Execute Jython script
java $JAVA_OPTS -cp "$CLASSPATH" org.python.util.jython "$JYTHON_SCRIPT" "$GRIB_DIR" "$START_DATE" "$END_DATE"

# Check exit status
if [ $? -eq 0 ]; then
    echo "GRIB files merged successfully"
    exit 0
else
    echo "Error: Failed to merge GRIB files"
    exit 1
fi