package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/labstack/echo/v4"
)

// ---- Echo handler -------------------------------------------------
func handelGetLatestPrecip(c echo.Context) error {
	ctx, cancel := context.WithTimeout(c.Request().Context(), 2*time.Minute)
	defer cancel()

	meta, err := runGRIBtoCOG(ctx) // This already returns the PrecipMeta with CogPath
	if err != nil {
		// Use your existing respondWithError function
		// Ensure the error message from runGRIBtoCOG is passed
		return respondWithError(c, http.StatusInternalServerError, err.Error())
	}

	// Construct the URL for the COG file
	// meta.CogPath from Python is something like "data/cogs_output\\20250519_21Z.tif"
	// We only need the filename part for the URL if /cogs is mapped to data/cogs_output
	fileName := filepath.Base(meta.COGPath) // Extracts "20250519_21Z.tif"

	// This is the URL prefix you configured with e.Static() in main.go
	cogURLPrefix := "/cogs/"
	cogAccessURL := cogURLPrefix + fileName

	// Log the constructed URL for debugging
	log.Printf("Constructed COG access URL for frontend: %s", cogAccessURL)

	// Use your existing respondWithJSON function
	// Send a response that includes this new cogAccessURL
	return respondWithJSON(c, http.StatusOK, echo.Map{
		"timestamp": meta.Timestamp,
		// "tileURL": "https://cdn.example.com/precip/" + meta.Timestamp + "/{z}/{x}/{y}.png", // Old placeholder
		"cog_url": cogAccessURL, // This is the URL the frontend will use
		"bounds":  meta.Bounds,
		"width":   meta.Width,  // Include width from meta
		"height":  meta.Height, // Include height from meta
	})
}

// ---- Spawns Python ------------------------------------------------
func runGRIBtoCOG(ctx context.Context) (*PrecipMeta, error) {
	// --- Path Configuration ---
	// inFile: Relative to Go's working directory. Ensure this is correct.
	// Example: If Go runs from HMSBackend/, "gribFiles/test.grib2" is HMSBackend/gribFiles/test.grib2
	const inFile = "gribFiles/MRMS_RadarOnly_QPE_24H_00.00_20250516-150000.grib2"

	// outDir: Consider if this should be relative or if an absolute path is always guaranteed.
	// For an absolute path like "/data/cogs" on Windows, it might translate to "C:\data\cogs".
	// Using a relative path might be safer unless you configure an absolute one carefully.
	// Example of a relative output directory:
	const outDir = "data/cogs_output" // This will be relative to Go's working directory

	// Python script path: Relative to Go's working directory.
	scriptRelativePath := filepath.Join("python_scripts", "get_rainfall_accumulation", "grib_to_cog.py")

	// --- Ensure output directory exists ---
	// This is a common cause for Python script failures if it tries to write to a non-existent directory.
	if err := os.MkdirAll(outDir, 0755); err != nil {
		log.Printf("Error creating output directory '%s': %v", outDir, err)
		return nil, fmt.Errorf("failed to create output directory %s: %w", outDir, err)
	}

	tag := time.Now().UTC().Format("20060102_15Z")
	const py = "C:\\Users\\diego\\anaconda3\\envs\\grib2cog\\python.exe" // Python interpreter

	// Log the command and arguments for easier debugging
	log.Printf("Executing: %s %s %s %s %s", py, scriptRelativePath, inFile, tag, outDir)

	cmd := exec.CommandContext(ctx, py, scriptRelativePath, inFile, tag, outDir)

	// Capture stdout and stderr in separate buffers
	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	// Run the command
	err := cmd.Run() // Use Run() when you have separate buffers for stdout/stderr

	// --- Log Python's output ---
	// Log stderr (Python's error messages, tracebacks, or prints to sys.stderr)
	if stderrBuf.Len() > 0 {
		log.Printf("Python STDERR:\n%s", stderrBuf.String())
	}
	// Log stdout (Python's print() statements)
	// This will show you everything Python printed, including debug prints.
	if stdoutBuf.Len() > 0 {
		log.Printf("Python STDOUT:\n%s", stdoutBuf.String())
	}

	// --- Handle command execution error ---
	if err != nil {
		// The actual Python traceback should be in the stderr log above.
		return nil, fmt.Errorf("python script execution failed: %w. See STDERR log above for Python errors", err)
	}

	// --- Process Python's standard output (expected to be JSON) ---
	var meta PrecipMeta
	// json.Unmarshal expects stdoutBuf.Bytes() to be *only* the JSON.
	if err := json.Unmarshal(stdoutBuf.Bytes(), &meta); err != nil {
		log.Printf("Failed to unmarshal JSON from Python script. Raw STDOUT that caused error was:\n%s", stdoutBuf.String())
		return nil, fmt.Errorf("failed to unmarshal JSON from python script: %w", err)
	}

	return &meta, nil
}
