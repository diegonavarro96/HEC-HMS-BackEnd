#!/bin/bash
# MergeGRIBFilesRealTimeBatch.sh - Linux/WSL version
# This script merges GRIB files using Jython and Vortex

# ===== BASIC CONFIGURATION =====================================
HMS_HOME="/opt/hms"
JYTHON_JAR="/opt/jython.jar"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
JYTHON_SCRIPT="$(dirname "$SCRIPT_DIR")/MergeGRIBFilesRealTimeJython.py"
HEAP_GB=8  # Adjust heap size as needed

# Use HMS's Java for compatibility
JAVA_EXEC="$HMS_HOME/jre/bin/java"

# ===== SCRIPT ARGUMENTS ========================================
# Go passes: gribDownloadPath, shapefilePath, outputDSS
INPUT_FOLDER="$1"
SHAPEFILE_PATH_ARG="$2"
OUTPUT_DSS="$3"

# Use default shapefile if empty string passed
if [ -z "$SHAPEFILE_PATH_ARG" ]; then
    SHAPEFILE_PATH="/home/diego/Documents/FloodaceDocuments/HEC-HMS-BackEnd/gis_data/shapefiles/Bexar_County.shp"
else
    SHAPEFILE_PATH="$SHAPEFILE_PATH_ARG"
fi

# Export for Jython script
export VORTEX_OUTPUT_DSS_PATH="$OUTPUT_DSS"
export VORTEX_SHAPEFILE_PATH="$SHAPEFILE_PATH"

# ===== PATHS AND ENVIRONMENT VARIABLES =========================
# Use minimal environment to avoid conflicts
export GDAL_DATA="$HMS_HOME/bin/gdal/gdal-data"
export PROJ_LIB="$HMS_HOME/bin/gdal/proj"

# ----- CLASSPATH -----------------------------------------------
export CLASSPATH="$HMS_HOME/lib/*:$JYTHON_JAR"

# ===== LIMIT PARALLELISM (avoid ConcurrentImporter) ============
export JAVA_TOOL_OPTIONS="-Djava.util.concurrent.ForkJoinPool.common.parallelism=1"

# ===== CHECK HEAP ALLOCATION ===================================
echo "=== JVM heap check =========================================="
$JAVA_EXEC -Xmx${HEAP_GB}g -XX:+PrintFlagsFinal -version 2>&1 | grep -i "MaxHeapSize"
echo "============================================================="

# ===== RUN THE JYTHON SCRIPT ===================================
$JAVA_EXEC \
    -Xmx${HEAP_GB}g \
    -Djava.library.path="$HMS_HOME/bin" \
    -cp "$CLASSPATH" \
    org.python.util.jython "$JYTHON_SCRIPT" "$@"

if [ $? -ne 0 ]; then
    echo "**** ERROR: Script failed. Check the log. ****"
    exit 1
fi

echo "Script completed successfully."
exit 0