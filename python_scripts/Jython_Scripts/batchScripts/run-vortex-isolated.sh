#!/bin/bash
# run-vortex-isolated.sh - Run Vortex in isolated environment
# This script runs Vortex with clean environment to avoid glibc conflicts

# Save original LD_LIBRARY_PATH
ORIG_LD_LIBRARY_PATH="$LD_LIBRARY_PATH"

# Clear LD_LIBRARY_PATH to avoid conflicts
unset LD_LIBRARY_PATH

# Set minimal required paths
HMS_HOME="/opt/hms"
JYTHON_JAR="/opt/jython.jar"

# Run with HMS's Java and minimal library path
exec "$HMS_HOME/jre/bin/java" \
    -Djava.library.path="$HMS_HOME/bin" \
    "$@"