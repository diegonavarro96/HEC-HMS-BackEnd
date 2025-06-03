#!/bin/bash
# MergeGRIBFilesRealTimeBatch.sh - Linux/WSL version
# This script merges GRIB files using Jython and Vortex

# ===== BASIC CONFIGURATION =====================================
VORTEX_HOME="/opt/vortex/vortex-0.11.25"
JYTHON_JAR="/opt/jython.jar"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
JYTHON_SCRIPT="$(dirname "$SCRIPT_DIR")/MergeGRIBFilesRealTimeJython.py"
HEAP_GB=32  # Adjust heap size as needed

# ===== PATHS AND ENVIRONMENT VARIABLES =========================
export PATH="$VORTEX_HOME/bin/gdal:$VORTEX_HOME/bin/netcdf:$PATH"
export GDAL_DATA="/usr/share/gdal"
export PROJ_LIB="/usr/share/proj"

# ----- CLASSPATH -----------------------------------------------
export CLASSPATH="$VORTEX_HOME/lib/*:$JYTHON_JAR"

# ===== LIMIT PARALLELISM (avoid ConcurrentImporter) ============
export JAVA_TOOL_OPTIONS="-Djava.util.concurrent.ForkJoinPool.common.parallelism=1"

# ===== CHECK HEAP ALLOCATION ===================================
echo "=== JVM heap check =========================================="
java -Xmx${HEAP_GB}g -XX:+PrintFlagsFinal -version 2>&1 | grep -i "MaxHeapSize"
echo "============================================================="

# ===== RUN THE JYTHON SCRIPT ===================================
java \
    -Xmx${HEAP_GB}g \
    -Djava.library.path="$VORTEX_HOME/bin:$VORTEX_HOME/bin/gdal" \
    -cp "$CLASSPATH" \
    org.python.util.jython "$JYTHON_SCRIPT" "$@"

if [ $? -ne 0 ]; then
    echo "**** ERROR: Script failed. Check the log. ****"
    exit 1
fi

echo "Script completed successfully."
exit 0