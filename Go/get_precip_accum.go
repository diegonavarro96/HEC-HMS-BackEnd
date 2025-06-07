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
	"strconv"
	"time"

	"github.com/labstack/echo/v4"
)

// mrmsDataSourceURL will be set by main.go after parsing flags.
// It's declared as a package-level variable in main.go.

// FetchLatestQPE fetches the latest QPE GRIB file from the MRMS source,
// decompressing it and saving it locally.
// It returns the path to the saved GRIB file or an error.
func FetchLatestQPE(ctx context.Context, accumulationPeriod string) (string, error) {
	// Build the URL based on accumulation period
	var dataSourceURL string
	baseURL := "https://mrms.ncep.noaa.gov/2D/RadarOnly_QPE_"

	switch accumulationPeriod {
	case "24H", "24":
		dataSourceURL = baseURL + "24H/"
	case "48H", "48":
		dataSourceURL = baseURL + "48H/"
	case "72H", "72":
		dataSourceURL = baseURL + "72H/"
	default:
		// Default to 24H if invalid period specified
		dataSourceURL = baseURL + "24H/"
		log.Printf("Invalid or missing accumulation period '%s', defaulting to 24H", accumulationPeriod)
	}

	log.Printf("Fetching GRIB index from: %s", dataSourceURL)

	// 1. Fetch the HTML index
	req, err := http.NewRequestWithContext(ctx, "GET", dataSourceURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request for MRMS index: %w", err)
	}
	// Use a client with a reasonable timeout for fetching the index page
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch MRMS index from %s: %w", dataSourceURL, err)
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
		return "", fmt.Errorf("no .grib2.gz files found in MRMS index at %s. HTML content might have changed or list is empty", dataSourceURL)
	}

	// The last match is considered the newest
	latestFileRelativePath := matches[len(matches)-1][1]
	log.Printf("Latest GRIB file found in index: %s", latestFileRelativePath)

	// Construct full download URL
	baseParsedURL, err := url.Parse(dataSourceURL)
	if err != nil {
		return "", fmt.Errorf("failed to parse base MRMS URL %s: %w", dataSourceURL, err)
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
	gribFilesDir := AppConfig.Paths.GribFilesDir
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

// PrecipRequest represents the JSON request body for precipitation data
type PrecipRequest struct {
	AccumulationPeriod string `json:"accumulation_period"`
}

// ---- Echo handler -------------------------------------------------
func handelGetLatestPrecip(c echo.Context) error {
	ctx, cancel := context.WithTimeout(c.Request().Context(), 2*time.Minute)
	defer cancel()

	// Parse JSON body
	var req PrecipRequest
	if err := c.Bind(&req); err != nil {
		// If binding fails or no JSON, default to 24H
		log.Printf("Failed to bind request body or no JSON provided, defaulting to 24H: %v", err)
		req.AccumulationPeriod = "24H"
	}

	// If accumulation period is empty, default to 24H
	if req.AccumulationPeriod == "" {
		req.AccumulationPeriod = "24H"
	}

	// Log the requested period
	log.Printf("Processing precipitation request for accumulation period: %s", req.AccumulationPeriod)

	meta, err := runGRIBtoCOG(ctx, req.AccumulationPeriod) // Pass the accumulation period
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
func runGRIBtoCOG(ctx context.Context, accumulationPeriod string) (*PrecipMeta, error) {
	// --- Path Configuration ---
	// inFile is now determined by FetchLatestQPE
	latestGribFilePath, err := FetchLatestQPE(ctx, accumulationPeriod)
	if err != nil {
		// FetchLatestQPE already logs detailed errors.
		return nil, fmt.Errorf("failed to fetch latest QPE GRIB file for COG conversion: %w", err)
	}
	log.Printf("Using GRIB file for COG conversion: %s", latestGribFilePath)

	// outDir: Use configured path
	outDir := AppConfig.Paths.StaticCogDir

	// Python script path: Use configured path
	scriptRelativePath := GetPythonScriptPath(filepath.Join("get_rainfall_accumulation", "grib_to_cog.py"))

	// --- Ensure output directory exists ---
	if err := os.MkdirAll(outDir, 0755); err != nil {
		log.Printf("Error creating output directory '%s': %v", outDir, err)
		return nil, fmt.Errorf("failed to create output directory %s: %w", outDir, err)
	}

	tag := time.Now().UTC().Format("20060102_15Z")
	py := GetPythonPath("grib2cog") // Python interpreter

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

// HistoricalPrecipRequest represents the JSON request body for historical precipitation data
type HistoricalPrecipRequest struct {
	Date string `json:"date"` // Expected format: YYYYMMDD
}

// FetchHistoricalQPE fetches a historical QPE GRIB file from the mtarchive.geol.iastate.edu archive
func FetchHistoricalQPE(ctx context.Context, dateStr string) (string, error) {
	// Validate date format (YYYYMMDD)
	if len(dateStr) != 8 {
		return "", fmt.Errorf("invalid date format: expected YYYYMMDD, got %s", dateStr)
	}

	// Parse the date
	year := dateStr[0:4]
	month := dateStr[4:6]
	day := dateStr[6:8]

	// Validate date components
	yearInt, err := strconv.Atoi(year)
	if err != nil || yearInt < 2000 || yearInt > 2100 {
		return "", fmt.Errorf("invalid year in date: %s", year)
	}
	monthInt, err := strconv.Atoi(month)
	if err != nil || monthInt < 1 || monthInt > 12 {
		return "", fmt.Errorf("invalid month in date: %s", month)
	}
	dayInt, err := strconv.Atoi(day)
	if err != nil || dayInt < 1 || dayInt > 31 {
		return "", fmt.Errorf("invalid day in date: %s", day)
	}

	// Construct the archive URL
	archiveURL := fmt.Sprintf("%s%s/%s/%s/mrms/ncep/MultiSensor_QPE_72H_Pass2/",
		AppConfig.URLs.MRMSArchive, year, month, day)

	log.Printf("Fetching historical GRIB index from: %s", archiveURL)

	// 1. Fetch the HTML index
	req, err := http.NewRequestWithContext(ctx, "GET", archiveURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request for historical MRMS index: %w", err)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch historical MRMS index from %s: %w", archiveURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to fetch historical MRMS index, status: %s (data might not be available for this date)", resp.Status)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read historical MRMS index body: %w", err)
	}
	bodyString := string(bodyBytes)

	// 2. Parse HTML to find the latest *.grib2.gz file
	re := regexp.MustCompile(`<a\s+(?:[^>]*?\s+)?href="([^"]*?\.grib2\.gz)"`)
	matches := re.FindAllStringSubmatch(bodyString, -1)

	if len(matches) == 0 {
		return "", fmt.Errorf("no .grib2.gz files found in historical MRMS index at %s", archiveURL)
	}

	// The last match is considered the newest for that day
	latestFileRelativePath := matches[len(matches)-1][1]
	log.Printf("Latest historical GRIB file found: %s", latestFileRelativePath)

	// Construct full download URL
	baseParsedURL, err := url.Parse(archiveURL)
	if err != nil {
		return "", fmt.Errorf("failed to parse base historical MRMS URL %s: %w", archiveURL, err)
	}
	fileDownloadURL := baseParsedURL.ResolveReference(&url.URL{Path: latestFileRelativePath})
	log.Printf("Constructed full historical download URL: %s", fileDownloadURL.String())

	// 3. Download the selected .grib2.gz file
	log.Printf("Downloading historical GRIB file from: %s", fileDownloadURL.String())
	fileReq, err := http.NewRequestWithContext(ctx, "GET", fileDownloadURL.String(), nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request for historical GRIB file download: %w", err)
	}

	fileClient := &http.Client{Timeout: 10 * time.Minute}
	fileResp, err := fileClient.Do(fileReq)
	if err != nil {
		return "", fmt.Errorf("failed to download historical GRIB file %s: %w", fileDownloadURL.String(), err)
	}
	defer fileResp.Body.Close()

	if fileResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(fileResp.Body)
		return "", fmt.Errorf("failed to download historical GRIB file, status: %s. Response: %s", fileResp.Status, string(body))
	}

	// 4. Stream-decompress (GZIP) and save
	gzReader, err := gzip.NewReader(fileResp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to create gzip reader for historical file: %w", err)
	}
	defer gzReader.Close()

	// Ensure gribFiles directory exists
	gribFilesDir := AppConfig.Paths.GribFilesDir
	if err := os.MkdirAll(gribFilesDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create directory %s: %w", gribFilesDir, err)
	}

	// Save as historical_qpe.grib2 (will be overwritten each time)
	outputFilePath := filepath.Join(gribFilesDir, "historical_qpe.grib2")
	outFile, err := os.Create(outputFilePath)
	if err != nil {
		return "", fmt.Errorf("failed to create historical output file %s: %w", outputFilePath, err)
	}
	defer outFile.Close()

	log.Printf("Decompressing and saving historical GRIB data to: %s", outputFilePath)
	_, err = io.Copy(outFile, gzReader)
	if err != nil {
		return "", fmt.Errorf("failed to decompress and save historical GRIB file to %s: %w", outputFilePath, err)
	}

	log.Printf("Successfully downloaded, decompressed, and saved historical GRIB file to %s", outputFilePath)
	return outputFilePath, nil
}

// handelGetHistoricalPrecip handles requests for historical precipitation data
func handelGetHistoricalPrecip(c echo.Context) error {
	ctx, cancel := context.WithTimeout(c.Request().Context(), 3*time.Minute)
	defer cancel()

	// Parse JSON body
	var req HistoricalPrecipRequest
	if err := c.Bind(&req); err != nil {
		return respondWithError(c, http.StatusBadRequest, "Invalid request body: "+err.Error())
	}

	// Validate that date is provided
	if req.Date == "" {
		return respondWithError(c, http.StatusBadRequest, "Date field is required (format: YYYYMMDD)")
	}

	// Log the requested date
	log.Printf("Processing historical precipitation request for date: %s", req.Date)

	// Run the historical processing
	meta, err := runHistoricalGRIBtoCOG(ctx, req.Date)
	if err != nil {
		return respondWithError(c, http.StatusInternalServerError, err.Error())
	}

	// Construct the URL for the COG file
	fileName := filepath.Base(meta.COGPath)
	cogURLPrefix := "/cogs/"
	cogAccessURL := cogURLPrefix + fileName

	log.Printf("Constructed historical COG access URL for frontend: %s", cogAccessURL)

	// Return the response
	return respondWithJSON(c, http.StatusOK, echo.Map{
		"timestamp": meta.Timestamp,
		"date":      req.Date,
		"cog_url":   cogAccessURL,
		"bounds":    meta.Bounds,
		"width":     meta.Width,
		"height":    meta.Height,
	})
}

// runHistoricalGRIBtoCOG processes historical GRIB data to COG format
func runHistoricalGRIBtoCOG(ctx context.Context, dateStr string) (*PrecipMeta, error) {
	// Fetch the historical GRIB file
	historicalGribFilePath, err := FetchHistoricalQPE(ctx, dateStr)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch historical QPE GRIB file: %w", err)
	}
	log.Printf("Using historical GRIB file for COG conversion: %s", historicalGribFilePath)

	// Output directory
	outDir := AppConfig.Paths.StaticCogDir

	// Python script path
	scriptRelativePath := GetPythonScriptPath(filepath.Join("get_rainfall_accumulation", "grib_to_cog.py"))

	// Ensure output directory exists
	if err := os.MkdirAll(outDir, 0755); err != nil {
		log.Printf("Error creating output directory '%s': %v", outDir, err)
		return nil, fmt.Errorf("failed to create output directory %s: %w", outDir, err)
	}

	// Use the date as the tag for the output file
	tag := fmt.Sprintf("historical_%s", dateStr)
	py := GetPythonPath("grib2cog")

	log.Printf("Executing Python script for historical COG conversion: %s %s %s %s %s", py, scriptRelativePath, historicalGribFilePath, tag, outDir)

	cmd := exec.CommandContext(ctx, py, scriptRelativePath, historicalGribFilePath, tag, outDir)

	// Capture stdout and stderr
	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	// Run the command
	err = cmd.Run()

	// Log Python's output
	if stderrBuf.Len() > 0 {
		log.Printf("Python STDERR:\n%s", stderrBuf.String())
	}
	if stdoutBuf.Len() > 0 {
		log.Printf("Python STDOUT:\n%s", stdoutBuf.String())
	}

	// Handle command execution error
	if err != nil {
		return nil, fmt.Errorf("python script execution failed: %w. See STDERR log above for Python errors", err)
	}

	// Process Python's standard output (expected to be JSON)
	var meta PrecipMeta
	if err := json.Unmarshal(stdoutBuf.Bytes(), &meta); err != nil {
		log.Printf("Failed to unmarshal JSON from Python script. Raw STDOUT was:\n%s", stdoutBuf.String())
		return nil, fmt.Errorf("failed to unmarshal JSON from python script: %w", err)
	}

	return &meta, nil
}
