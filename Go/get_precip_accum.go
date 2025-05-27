package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"time"

	"github.com/labstack/echo/v4"
)

// mrmsDataSourceURL will be set by main.go after parsing flags.
// It's declared as a package-level variable in main.go.

// FetchLatestQPE fetches the latest QPE GRIB file from the MRMS source,
// decompressing it and saving it locally.
// It returns the path to the saved GRIB file or an error.
func FetchLatestQPE(ctx context.Context) (string, error) {
	// mrmsDataSourceURL is a package-level var set in main.go
	if mrmsDataSourceURL == "" {
		// This case should ideally not be hit if main.go initializes it with a default.
		log.Println("CRITICAL: mrmsDataSourceURL is not set. This indicates an initialization issue.")
		return "", fmt.Errorf("mrmsDataSourceURL is not configured")
	}

	log.Printf("Fetching GRIB index from: %s", mrmsDataSourceURL)

	// 1. Fetch the HTML index
	req, err := http.NewRequestWithContext(ctx, "GET", mrmsDataSourceURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request for MRMS index: %w", err)
	}
	// Use a client with a reasonable timeout for fetching the index page
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch MRMS index from %s: %w", mrmsDataSourceURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to fetch MRMS index, status: %s", resp.Status)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read MRMS index body: %w", err)
	}
	bodyString := string(bodyBytes)

	// 2. Parse HTML to find the latest *.grib2.gz file
	// Regex to find <a href="...grib2.gz">. This captures the href content.
	re := regexp.MustCompile(`<a\s+(?:[^>]*?\s+)?href="([^"]*?\.grib2\.gz)"`)
	matches := re.FindAllStringSubmatch(bodyString, -1)

	if len(matches) == 0 {
		return "", fmt.Errorf("no .grib2.gz files found in MRMS index at %s. HTML content might have changed or list is empty", mrmsDataSourceURL)
	}

	// The last match is considered the newest
	latestFileRelativePath := matches[len(matches)-1][1]
	log.Printf("Latest GRIB file found in index: %s", latestFileRelativePath)

	// Construct full download URL
	baseParsedURL, err := url.Parse(mrmsDataSourceURL)
	if err != nil {
		return "", fmt.Errorf("failed to parse base MRMS URL %s: %w", mrmsDataSourceURL, err)
	}
	// ResolveReference correctly handles joining, whether latestFileRelativePath is absolute or relative,
	// and whether mrmsDataSourceURL ends with a slash.
	fileDownloadURL := baseParsedURL.ResolveReference(&url.URL{Path: latestFileRelativePath})
	log.Printf("Constructed full download URL: %s", fileDownloadURL.String())

	// 3. Download the selected .grib2.gz file
	log.Printf("Downloading GRIB file from: %s", fileDownloadURL.String())
	fileReq, err := http.NewRequestWithContext(ctx, "GET", fileDownloadURL.String(), nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request for GRIB file download: %w", err)
	}
	// Use a client with a potentially longer timeout for file download
	fileClient := &http.Client{Timeout: 10 * time.Minute} // Increased timeout for large files
	fileResp, err := fileClient.Do(fileReq)
	if err != nil {
		return "", fmt.Errorf("failed to download GRIB file %s: %w", fileDownloadURL.String(), err)
	}
	defer fileResp.Body.Close()

	if fileResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(fileResp.Body) // Try to read body for more error info
		return "", fmt.Errorf("failed to download GRIB file, status: %s. Response: %s", fileResp.Status, string(body))
	}

	// 4. Stream-decompress (GZIP) and save
	gzReader, err := gzip.NewReader(fileResp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to create gzip reader for downloaded file: %w", err)
	}
	defer gzReader.Close()

	// Ensure gribFiles directory exists
	gribFilesDir := "gribFiles"
	if err := os.MkdirAll(gribFilesDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create directory %s: %w", gribFilesDir, err)
	}

	outputFilePath := filepath.Join(gribFilesDir, "latest_qpe.grib2")
	outFile, err := os.Create(outputFilePath)
	if err != nil {
		return "", fmt.Errorf("failed to create output file %s: %w", outputFilePath, err)
	}
	defer outFile.Close()

	log.Printf("Decompressing and saving GRIB data to: %s", outputFilePath)
	_, err = io.Copy(outFile, gzReader)
	if err != nil {
		return "", fmt.Errorf("failed to decompress and save GRIB file to %s: %w", outputFilePath, err)
	}

	log.Printf("Successfully downloaded, decompressed, and saved GRIB file to %s", outputFilePath)
	return outputFilePath, nil
}

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
	// inFile is now determined by FetchLatestQPE
	latestGribFilePath, err := FetchLatestQPE(ctx)
	if err != nil {
		// FetchLatestQPE already logs detailed errors.
		return nil, fmt.Errorf("failed to fetch latest QPE GRIB file for COG conversion: %w", err)
	}
	log.Printf("Using GRIB file for COG conversion: %s", latestGribFilePath)

	// outDir: This will be relative to Go's working directory
	const outDir = "../data/cogs_output"

	// Python script path: Relative to Go's working directory.
	scriptRelativePath := filepath.Join("../python_scripts", "get_rainfall_accumulation", "grib_to_cog.py")

	// --- Ensure output directory exists ---
	if err := os.MkdirAll(outDir, 0755); err != nil {
		log.Printf("Error creating output directory '%s': %v", outDir, err)
		return nil, fmt.Errorf("failed to create output directory %s: %w", outDir, err)
	}

	tag := time.Now().UTC().Format("20060102_15Z")
	const py = "C:\\Users\\diego\\anaconda3\\envs\\grib2cog\\python.exe" // Python interpreter

	// Log the command and arguments for easier debugging
	// Use latestGribFilePath as the inFile argument for the Python script
	log.Printf("Executing Python script for COG conversion: %s %s %s %s %s", py, scriptRelativePath, latestGribFilePath, tag, outDir)

	cmd := exec.CommandContext(ctx, py, scriptRelativePath, latestGribFilePath, tag, outDir)

	// Capture stdout and stderr in separate buffers
	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	// Run the command
	err = cmd.Run() // Use Run() when you have separate buffers for stdout/stderr

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
