package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"path/filepath" // Added for filepath.Abs
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	// "os" // No longer needed for os.Getenv for URLs
	// "bytes" // No longer needed
	// "encoding/json" // No longer needed
	// "io" // No longer needed
	// "net/http" // No longer needed
)

const pythonExePath = `C:\Users\diego\anaconda3\envs\HMS\python.exe`
const jythonExePath = `C:\Program Files\HEC\HEC-DSSVue\jython.bat`

// executePythonScript is a helper function to execute a Python script
func executePythonScript(ctx context.Context, scriptPath string, scriptArgs ...string) error {
	absScriptPath, err := filepath.Abs(scriptPath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path for script %s: %w", scriptPath, err)
	}

	cmdArgs := append([]string{absScriptPath}, scriptArgs...)
	cmd := exec.CommandContext(ctx, pythonExePath, cmdArgs...)

	log.Printf("INFO: Executing command: %s %s", pythonExePath, strings.Join(cmdArgs, " "))

	output, err := cmd.CombinedOutput() // Captures both stdout and stderr

	if len(output) > 0 {
		// Log output, prefixing each line for clarity
		log.Printf("INFO: Output from %s:\n%s", scriptPath, indentOutput(string(output)))
	}

	if err != nil {
		// If there was an error, CombinedOutput() might still contain useful error messages from the script
		return fmt.Errorf("failed to execute script %s (resolved to %s): %w. Output: %s", scriptPath, absScriptPath, err, string(output))
	}

	log.Printf("INFO: Script %s (resolved to %s) completed successfully.", scriptPath, absScriptPath)
	return nil
}

// executePythonScript is a helper function to execute a Python script
func executeJythonScript(ctx context.Context, scriptPath string) error {
	absScriptPath, err := filepath.Abs(scriptPath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path for script %s: %w", scriptPath, err)
	}

	cmdArgs := append([]string{absScriptPath})
	cmd := exec.CommandContext(ctx, jythonExePath, cmdArgs...)

	log.Printf("INFO: Executing command: %s %s", jythonExePath, strings.Join(cmdArgs, " "))

	output, err := cmd.CombinedOutput() // Captures both stdout and stderr

	if len(output) > 0 {
		// Log output, prefixing each line for clarity
		log.Printf("INFO: Output from %s:\n%s", scriptPath, indentOutput(string(output)))
	}

	if err != nil {
		// If there was an error, CombinedOutput() might still contain useful error messages from the script
		return fmt.Errorf("failed to execute script %s (resolved to %s): %w. Output: %s", scriptPath, absScriptPath, err, string(output))
	}

	log.Printf("INFO: Script %s (resolved to %s) completed successfully.", scriptPath, absScriptPath)
	return nil
}

// executeBatchFile is a helper function to execute a Windows batch file
func executeBatchFile(ctx context.Context, batchPath string, batchArgs ...string) error {
	absBatchPath, err := filepath.Abs(batchPath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path for batch file %s: %w", batchPath, err)
	}

	cmdArgs := append([]string{"/c", absBatchPath}, batchArgs...)
	cmd := exec.CommandContext(ctx, "cmd.exe", cmdArgs...)

	log.Printf("INFO: Executing batch file: cmd.exe %s", strings.Join(cmdArgs, " "))

	output, err := cmd.CombinedOutput() // Captures both stdout and stderr

	if len(output) > 0 {
		// Log output, prefixing each line for clarity
		log.Printf("INFO: Output from %s:\n%s", batchPath, indentOutput(string(output)))
	}

	if err != nil {
		// If there was an error, CombinedOutput() might still contain useful error messages
		return fmt.Errorf("failed to execute batch file %s (resolved to %s): %w. Output: %s", batchPath, absBatchPath, err, string(output))
	}

	log.Printf("INFO: Batch file %s (resolved to %s) completed successfully.", batchPath, absBatchPath)
	return nil
}

// indentOutput adds a prefix to each line of a multi-line string for better log readability.
func indentOutput(output string) string {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	for i, line := range lines {
		lines[i] = "  | " + line
	}
	return strings.Join(lines, "\n")
}

// RunProcessingPipeline orchestrates a sequence of Python script executions.
// It accepts an optional date in YYYYMMDD format and an optional run hour in HH format.
func RunProcessingPipeline(ctx context.Context, optionalDateYYYYMMDD string, optionalRunHourHH string) error {
	// --- Date Calculation (used for download steps if not provided) ---
	dateToUse := optionalDateYYYYMMDD
	if dateToUse == "" {
		// Default to current local date
		dateToUse = time.Now().Format("20060102") // YYYYMMDD format
		log.Printf("INFO: No date provided, using current local date: %s", dateToUse)
	} else {
		log.Printf("INFO: Using provided date: %s", dateToUse)
	}

	// --- Run Hour Calculation (for HRRR download if not provided) ---
	runHourToUse := optionalRunHourHH
	if runHourToUse == "" {
		// Default to current UTC hour minus 1
		utcTimeMinusOneHour := time.Now().UTC().Add(-1 * time.Hour)
		runHourToUse = utcTimeMinusOneHour.Format("15") // "15" is the format code for hour (00-23)
		log.Printf("INFO: No run hour provided for HRRR download, calculating as current UTC hour - 1: %sZ", runHourToUse)
	} else {
		log.Printf("INFO: Using provided run hour for HRRR download: %sZ", runHourToUse)
	}

	var err error

	// Script execution steps
	scriptsToRun := []struct {
		name     string
		path     string
		isBatch  bool
		argsFunc func() []string // Function to generate args, allows use of dateToUse/runHourToUse
	}{
		{
			name:    "Get GRIB2 Files RealTime",
			path:    "D:/FloodaceDocuments/HMS/HMSBackend/python_scripts/RealTime/getgrb2FilesRealTime.py",
			isBatch: false,
			argsFunc: func() []string {
				return []string{dateToUse}
			},
		},
		{
			name:    "Get HRRR Forecast GRIB",
			path:    "D:/FloodaceDocuments/HMS/HMSBackend/python_scripts/RealTime/getHRRRForecastGrb.py",
			isBatch: false,
			argsFunc: func() []string {
				return []string{dateToUse, runHourToUse}
			},
		},
		{
			name:    "Merge GRIB Files RealTime",
			path:    "D:/FloodaceDocuments/HMS/HMSBackend/python_scripts/Jython_Scripts/batchScripts/MergeGRIBFilesRealTimeBatch.bat",
			isBatch: true,
			argsFunc: func() []string {
				// Pass the full folder path to the batch file
				return []string{fmt.Sprintf("D:\\FloodaceDocuments\\HMS\\HMSGit\\HEC-HMS-Floodace\\grb_downloads\\%s", dateToUse)}
			},
		},
		{
			name:    "Merge GRIB Files RealTime Pass 2",
			path:    "D:/FloodaceDocuments/HMS/HMSBackend/python_scripts/Jython_Scripts/batchScripts/MergeGRIBFilesRealTimePass2Batch.bat",
			isBatch: true,
			argsFunc: func() []string {
				// Pass the arguments as separate elements
				return []string{
					fmt.Sprintf("D:\\FloodaceDocuments\\HMS\\HMSGit\\HEC-HMS-Floodace\\grb_downloads\\%s", dateToUse),
					"", // Empty string for shapefile_path to use default
					"D:\\FloodaceDocuments\\HMS\\HMSGit\\HEC-HMS-Floodace\\hms_models\\LeonCreek\\Rainfall\\RainfallRealTimePass2.dss",
				}
			},
		},
		{
			name:    "Merge GRIB Files Forcast",
			path:    "D:/FloodaceDocuments/HMS/HMSBackend/python_scripts/Jython_Scripts/batchScripts/MergeGRIBFilesRealTimeHRRBatch.bat",
			isBatch: true,
			argsFunc: func() []string {
				// Pass the full folder path to the batch file
				return []string{fmt.Sprintf("D:\\FloodaceDocuments\\HMS\\HMSGit\\HEC-HMS-Floodace\\grb_downloads\\%s", dateToUse)}
			},
		},
		{
			name:    "Combine DSS Records Pass1 Pass2",
			path:    "D:/FloodaceDocuments/HMS/HMSBackend/python_scripts/RealTime/combineDssRecordsPass1Pass2.py",
			isBatch: false,
			argsFunc: func() []string {
				return []string{}
			},
		},
		{
			name:    "Combine DSS Records",
			path:    "D:/FloodaceDocuments/HMS/HMSBackend/python_scripts/RealTime/combineDssRecords.py",
			isBatch: false,
			argsFunc: func() []string {
				return []string{}
			},
		},
		{
			name:    "Set Control File",
			path:    "D:/FloodaceDocuments/HMS/HMSBackend/python_scripts/RealTime/setControlFile.py", // Note: historical path
			isBatch: false,
			argsFunc: func() []string {
				return []string{}
			},
		},
		{
			name:    "Run HMS RealTime",
			path:    "D:/FloodaceDocuments/HMS/HMSBackend/python_scripts/RealTime/runHMSRealTime.py",
			isBatch: false,
			argsFunc: func() []string {
				return []string{"6"} // Running with option "6"
			},
		},
	}

	for i, script := range scriptsToRun {
		stepNum := i + 1
		log.Printf("STEP %d: Running script '%s'...", stepNum, script.name)

		// Execute either batch file or Python script based on the isBatch flag
		if script.isBatch {
			err = executeBatchFile(ctx, script.path, script.argsFunc()...)
		} else {
			err = executePythonScript(ctx, script.path, script.argsFunc()...)
		}

		if err != nil {
			return fmt.Errorf("failed at step %d (%s): %w", stepNum, script.name, err)
		}
		log.Printf("STEP %d: Script '%s' completed successfully.", stepNum, script.name)

		// Add delay between tasks (except after the last task)
		if i < len(scriptsToRun)-1 {
			// Longer delay before Pass 2 merge to ensure resources are released
			if script.name == "Merge GRIB Files RealTime" {
				log.Printf("INFO: Waiting 2 seconds before Pass 2 merge task...")
				time.Sleep(15 * time.Second)
			} else {
				log.Printf("INFO: Waiting 300ms before next task...")
				time.Sleep(1000 * time.Millisecond)
			}
		}
	}

	log.Println("INFO: All processing steps triggered successfully!")
	return nil
}

// handleRunHMSPipeline handles the request to run the HMS processing pipeline
func handleRunHMSPipeline(c echo.Context) error {
	// Define a struct for the request body
	type PipelineRequest struct {
		Date    string `json:"date"`     // Optional date in YYYYMMDD format
		RunHour string `json:"run_hour"` // Optional run hour in HH format
	}

	// Parse request body
	var req PipelineRequest
	if err := c.Bind(&req); err != nil {
		log.Printf("Error parsing request body: %v", err)
		return respondWithError(c, http.StatusBadRequest, "Invalid request format")
	}

	// Log the received parameters
	log.Printf("Received HMS pipeline request: date=%s, run_hour=%s", req.Date, req.RunHour)

	// Run the pipeline in a goroutine to avoid blocking the HTTP response
	go func() {
		// Create a new context with a timeout
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Minute)
		defer cancel()

		// Run the pipeline
		err := RunProcessingPipeline(ctx, req.Date, req.RunHour)
		if err != nil {
			log.Printf("HMS pipeline failed: %v", err)
		}
	}()

	// Return a success response immediately
	return respondWithJSON(c, http.StatusAccepted, map[string]string{
		"message": "HMS processing pipeline started",
		"status":  "accepted",
	})
}
