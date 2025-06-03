package main

import (
	"path/filepath"
	"runtime"
	"strings"
)

// getScriptExtension returns the appropriate script extension based on the OS
func getScriptExtension() string {
	if runtime.GOOS == "windows" {
		return ".bat"
	}
	return ".sh"
}

// GetPythonPath returns the appropriate Python executable path
func GetPythonPath(envType string) string {
	switch envType {
	case "hms":
		return AppConfig.Python.HMSEnvPath
	case "grib2cog":
		return AppConfig.Python.Grib2CogEnvPath
	default:
		return AppConfig.Python.HMSEnvPath
	}
}

// GetJythonPath returns the Jython executable path
func GetJythonPath() string {
	return AppConfig.Jython.ExecutablePath
}

// GetJythonBatchScriptPath returns the full path to a Jython batch script
// Automatically adjusts the extension based on the operating system
func GetJythonBatchScriptPath(scriptName string) string {
	// If the script name ends with .bat, adjust for the OS
	if strings.HasSuffix(scriptName, ".bat") {
		if runtime.GOOS != "windows" {
			// Replace .bat with .sh for non-Windows systems
			scriptName = strings.TrimSuffix(scriptName, ".bat") + ".sh"
		}
	}
	return filepath.Join(AppConfig.Jython.BatchScriptsDir, scriptName)
}

// GetHMSPath returns the HEC-HMS executable path
func GetHMSPath() string {
	return AppConfig.HMS.ExecutablePath
}

// GetHMSControlFile returns the appropriate control file path
func GetHMSControlFile(runType string) string {
	switch runType {
	case "historical":
		return AppConfig.HMS.HistoricalControlFile
	default:
		return AppConfig.HMS.RealTimeControlFile
	}
}

// GetHMSScript returns the appropriate HMS script path
func GetHMSScript(runType string) string {
	var scriptPath string
	switch runType {
	case "historical":
		scriptPath = filepath.Join(AppConfig.Paths.HMSScriptsDir, AppConfig.HMS.HistoricalScript)
	default:
		scriptPath = filepath.Join(AppConfig.Paths.HMSScriptsDir, AppConfig.HMS.RealTimeScript)
	}

	// Convert to absolute path
	absPath, err := filepath.Abs(scriptPath)
	if err != nil {
		// Return the original path if we can't get absolute path
		return scriptPath
	}
	return absPath
}

// GetDSSPath returns the full path to a DSS file in the Leon Creek model
func GetDSSPath(filename string) string {
	return filepath.Join(AppConfig.Paths.HMSModelsDir, "LeonCreek", "Rainfall", filename)
}

// GetDSSPath returns the full path to a DSS file in the Leon Creek model
func GetHistoricalDSSPath(filename string) string {
	return filepath.Join(AppConfig.Paths.HMSHistoricalModelsDir, "LeonCreek", "Rainfall", filename)
}

// GetGribDownloadPath returns the full path for GRIB downloads
func GetGribDownloadPath(filename string) string {
	return filepath.Join(AppConfig.Paths.GrbDownloadsDir, filename)
}

// GetPythonScriptPath returns the full path to a Python script
func GetPythonScriptPath(scriptPath string) string {
	return filepath.Join(AppConfig.Paths.PythonScriptsDir, scriptPath)
}

// GetJSONOutputPath returns the full path for JSON output files
func GetJSONOutputPath(filename string) string {
	return filepath.Join(AppConfig.Paths.JSONOutputDir, filename)
}

// GetHMSBatchScriptPath returns the full path to an HMS batch script
// Automatically adjusts the extension based on the operating system
func GetHMSBatchScriptPath(scriptName string) string {
	// If the script name ends with .bat, adjust for the OS
	if strings.HasSuffix(scriptName, ".bat") {
		if runtime.GOOS != "windows" {
			// Replace .bat with .sh for non-Windows systems
			scriptName = strings.TrimSuffix(scriptName, ".bat") + ".sh"
		}
	}
	return filepath.Join(AppConfig.Paths.HMSScriptsDir, "batchScripts", scriptName)
}

// IsWindows returns true if running on Windows
func IsWindows() bool {
	return runtime.GOOS == "windows"
}

// IsLinux returns true if running on Linux
func IsLinux() bool {
	return runtime.GOOS == "linux"
}
