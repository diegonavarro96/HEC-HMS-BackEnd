#!/bin/bash
# CombineTwoDssFilesRealTimeAndHRRBatch.sh - Linux/WSL version
# This script combines RealTime and HRR DSS files using Jython

# ===== BASIC CONFIGURATION =====================================
HMS_HOME="/opt/hms"
JYTHON_JAR="/opt/jython.jar"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
JYTHON_SCRIPT="$(dirname "$SCRIPT_DIR")/CombineTwoDssFiles.py"
HEAP_GB=8  # Smaller heap for DSS operations

# Use HMS's Java for compatibility
JAVA_EXEC="$HMS_HOME/jre/bin/java"

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
    echo "**** ERROR: DSS combination failed. Check the log. ****"
    exit 1
fi

echo "Script completed successfully."
exit 0