# Platform Compatibility Changes Summary

## Overview
Updated the Go code to automatically detect the operating system and use the appropriate script files (.bat for Windows, .sh for Linux/Unix).

## Files Modified

### 1. config_getters.go
- Added `runtime` import
- Modified `GetJythonBatchScriptPath()` to automatically convert .bat to .sh on non-Windows systems
- Modified `GetHMSBatchScriptPath()` to automatically convert .bat to .sh on non-Windows systems
- Added helper functions:
  - `getScriptExtension()` - returns ".bat" or ".sh" based on OS
  - `IsWindows()` - returns true if running on Windows
  - `IsLinux()` - returns true if running on Linux

### 2. hms_real_time.go
- Added `runtime` import
- Updated `executeBatchFile()` to use `runtime.GOOS` instead of checking file extensions
- Added OS-specific validation and handling:
  - Windows: expects .bat files, uses cmd.exe
  - Linux/Unix: expects .sh files, uses bash, sets executable permissions
- Simplified HMS batch script execution - now just calls `GetHMSBatchScriptPath("HMSRealTimeBatch.bat")` which automatically handles OS detection

### 3. Files Created
Created Linux/Unix shell script equivalents for all Jython batch scripts:
- `/python_scripts/Jython_Scripts/batchScripts/MergeGRIBFilesRealTimeBatch.sh`
- `/python_scripts/Jython_Scripts/batchScripts/MergeGRIBFilesRealTimePass2Batch.sh`
- `/python_scripts/Jython_Scripts/batchScripts/MergeGRIBFilesRealTimeHRRBatch.sh`
- `/python_scripts/Jython_Scripts/batchScripts/CombineTwoDssFilesPass1Pass2Batch.sh`
- `/python_scripts/Jython_Scripts/batchScripts/CombineTwoDssFilesRealTimeAndHRRBatch.sh`

Note: HMS batch scripts (.sh versions) already existed in `/HMSScripts/batchScripts/`

## How It Works

1. **Automatic Extension Conversion**: When code calls `GetJythonBatchScriptPath("script.bat")` or `GetHMSBatchScriptPath("script.bat")`, the functions automatically convert to `.sh` on Linux/Unix systems.

2. **OS Detection**: Uses `runtime.GOOS` to detect the operating system:
   - "windows" for Windows
   - "linux" for Linux (including WSL)
   - "darwin" for macOS (treated as Unix)

3. **Execution Handling**: `executeBatchFile()` now:
   - Validates the correct extension for the OS
   - Uses the appropriate shell (cmd.exe for Windows, bash for Linux/Unix)
   - Sets executable permissions on Linux/Unix if needed

## Usage
No code changes needed in the calling functions. Simply continue using:
```go
// These will automatically use .bat on Windows or .sh on Linux
GetJythonBatchScriptPath("MergeGRIBFilesRealTimeBatch.bat")
GetHMSBatchScriptPath("HMSRealTimeBatch.bat")
```

## Configuration Notes for Linux/Unix
The shell scripts assume these paths (update as needed):
- Vortex: `/opt/vortex`
- Jython: `/opt/jython`
- HEC-HMS: `/opt/hms`

These can be configured through environment variables or by editing the .sh files directly.