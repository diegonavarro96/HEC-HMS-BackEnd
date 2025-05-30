package main

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
)

// downloadMRMSForDate downloads all MRMS files for a specific date
func downloadMRMSForDate(date time.Time, outputDir string) error {
	// Construct base URL
	year := date.Format("2006")
	month := date.Format("01")
	day := date.Format("02")
	dateStr := date.Format("20060102")

	baseURL := fmt.Sprintf("https://mtarchive.geol.iastate.edu/%s/%s/%s/mrms/ncep/MultiSensor_QPE_01H_Pass2/", year, month, day)

	log.Printf("Downloading MRMS data from: %s", baseURL)

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Download files for each hour (00 to 23)
	for hour := 0; hour < 24; hour++ {
		// Construct filename
		hourStr := fmt.Sprintf("%02d", hour)
		filename := fmt.Sprintf("MultiSensor_QPE_01H_Pass2_00.00_%s-%s0000.grib2.gz", dateStr, hourStr)
		fileURL := baseURL + filename

		// Download file
		err := downloadAndExtractFile(client, fileURL, outputDir)
		if err != nil {
			log.Printf("Warning: Failed to download %s: %v", filename, err)
			// Continue with next file instead of failing completely
			continue
		}
	}

	return nil
}

// downloadAndExtractFile downloads a gzipped file and extracts it
func downloadAndExtractFile(client *http.Client, url string, outputDir string) error {
	// Make HTTP request
	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("failed to download file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned status %d", resp.StatusCode)
	}

	// Extract filename from URL
	filename := filepath.Base(url)
	// Remove .gz extension for output filename
	outputFilename := filename[:len(filename)-3]
	outputPath := filepath.Join(outputDir, outputFilename)

	// Create output file
	outFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer outFile.Close()

	// Create gzip reader
	gzReader, err := gzip.NewReader(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzReader.Close()

	// Copy uncompressed data to output file
	_, err = io.Copy(outFile, gzReader)
	if err != nil {
		return fmt.Errorf("failed to extract file: %w", err)
	}

	log.Printf("Successfully downloaded and extracted: %s", outputFilename)
	return nil
}

// roundTimeDown rounds time down to the nearest hour (e.g., 10:24 -> 10:00)
func roundTimeDown(timeStr string) string {
	if timeStr == "" {
		return "00:00"
	}

	// Parse time string (expecting HH:MM format)
	parts := strings.Split(timeStr, ":")
	if len(parts) != 2 {
		return "00:00"
	}

	hour, err := strconv.Atoi(parts[0])
	if err != nil || hour < 0 || hour > 23 {
		return "00:00"
	}

	return fmt.Sprintf("%02d:00", hour)
}

// roundTimeUp rounds time up to the next hour (e.g., 11:01 -> 12:00, 11:00 -> 11:00)
func roundTimeUp(timeStr string) string {
	if timeStr == "" {
		return "23:00"
	}

	// Parse time string (expecting HH:MM format)
	parts := strings.Split(timeStr, ":")
	if len(parts) != 2 {
		return "23:00"
	}

	hour, err := strconv.Atoi(parts[0])
	if err != nil || hour < 0 || hour > 23 {
		return "23:00"
	}

	minute, err := strconv.Atoi(parts[1])
	if err != nil || minute < 0 || minute > 59 {
		return "23:00"
	}

	// If minutes > 0, round up to next hour
	if minute > 0 {
		hour++
		if hour > 23 {
			hour = 23 // Cap at 23:00
		}
	}

	return fmt.Sprintf("%02d:00", hour)
}

// updateHistoricalControlFile updates the control file with the specified dates and times
func updateHistoricalControlFile(startDate, endDate time.Time, startTime, endTime string) error {
	// Path to the control file
	controlFilePath := "D:/FloodaceDocuments/HMS/HMSBackend/hms_models/LeonCreek/RainHistorical.control"

	// Format dates for the control file (e.g., "9 May 2025")
	startDateStr := startDate.Format("2 January 2006")
	endDateStr := endDate.Format("2 January 2006")

	// Round times appropriately
	startTimeRounded := roundTimeDown(startTime)
	endTimeRounded := roundTimeUp(endTime)

	log.Printf("Updating control file with: Start: %s %s, End: %s %s",
		startDateStr, startTimeRounded, endDateStr, endTimeRounded)

	// Read the control file
	content, err := os.ReadFile(controlFilePath)
	if err != nil {
		return fmt.Errorf("failed to read control file: %w", err)
	}

	lines := strings.Split(string(content), "\n")
	updatedLines := make([]string, 0, len(lines))

	// Update specific lines
	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)

		switch {
		case strings.HasPrefix(trimmedLine, "Start Date:"):
			updatedLines = append(updatedLines, fmt.Sprintf("     Start Date: %s", startDateStr))
		case strings.HasPrefix(trimmedLine, "Start Time:"):
			updatedLines = append(updatedLines, fmt.Sprintf("     Start Time: %s", startTimeRounded))
		case strings.HasPrefix(trimmedLine, "End Date:"):
			updatedLines = append(updatedLines, fmt.Sprintf("     End Date: %s", endDateStr))
		case strings.HasPrefix(trimmedLine, "End Time:"):
			updatedLines = append(updatedLines, fmt.Sprintf("     End Time: %s", endTimeRounded))
		default:
			updatedLines = append(updatedLines, line)
		}
	}

	// Write back to file
	updatedContent := strings.Join(updatedLines, "\n")
	err = os.WriteFile(controlFilePath, []byte(updatedContent), 0644)
	if err != nil {
		return fmt.Errorf("failed to write control file: %w", err)
	}

	log.Printf("Successfully updated control file: %s", controlFilePath)
	return nil
}

// runHMSPipelineHistorical orchestrates the complete historical HMS processing pipeline
func runHMSPipelineHistorical(ctx context.Context, req HistoricalDownloadRequest) error {
	log.Printf("INFO: Starting historical HMS pipeline from %s to %s", req.StartDate, req.EndDate)

	// Step 0: Delete existing DSS files if they exist
	// Delete RainHistorical.dss
	existingDSSPath1 := "D:\\FloodaceDocuments\\HMS\\HMSBackend\\hms_models\\LeonCreek\\RainHistorical.dss"
	if _, err := os.Stat(existingDSSPath1); err == nil {
		log.Printf("Deleting existing RainHistorical.dss file...")
		if err := os.Remove(existingDSSPath1); err != nil {
			log.Printf("Warning: Failed to delete existing DSS file: %v", err)
			// Continue anyway as it might get overwritten
		} else {
			log.Printf("Successfully deleted existing RainHistorical.dss file")
		}
	}

	// Delete RainfallHistorical.dss
	existingDSSPath2 := "D:\\FloodaceDocuments\\HMS\\HMSBackend\\hms_models\\LeonCreek\\Rainfall\\RainfallHistorical.dss"
	if _, err := os.Stat(existingDSSPath2); err == nil {
		log.Printf("Deleting existing RainfallHistorical.dss file...")
		if err := os.Remove(existingDSSPath2); err != nil {
			log.Printf("Warning: Failed to delete existing Rainfall DSS file: %v", err)
			// Continue anyway as it might get overwritten
		} else {
			log.Printf("Successfully deleted existing RainfallHistorical.dss file")
		}
	}

	// Step 1: Download historical MRMS data
	log.Printf("STEP 1: Downloading historical MRMS data...")

	// Validate dates
	startDate, err := time.Parse("20060102", req.StartDate)
	if err != nil {
		return fmt.Errorf("invalid start date format: %w", err)
	}

	endDate, err := time.Parse("20060102", req.EndDate)
	if err != nil {
		return fmt.Errorf("invalid end date format: %w", err)
	}

	// Check if dates are in valid range (2021 to current)
	minDate := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)
	maxDate := time.Now()

	if startDate.Before(minDate) || endDate.Before(minDate) {
		return fmt.Errorf("dates must be from 2021 to current date")
	}

	if startDate.After(maxDate) || endDate.After(maxDate) {
		return fmt.Errorf("dates cannot be in the future")
	}

	if startDate.After(endDate) {
		return fmt.Errorf("start date must be before or equal to end date")
	}

	// Check if date range is within 5 days
	daysDifference := int(endDate.Sub(startDate).Hours() / 24)
	if daysDifference > 4 {
		return fmt.Errorf("date range cannot exceed 5 days")
	}

	// Create output directory
	outputDir := filepath.Join("D:/FloodaceDocuments/HMS/HMSBackend/gribFiles", "historical", req.EndDate)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Download files for each day
	currentDate := startDate
	downloadedCount := 0
	failedDates := []string{}

	for !currentDate.After(endDate) {
		err := downloadMRMSForDate(currentDate, outputDir)
		if err != nil {
			log.Printf("Failed to download data for %s: %v", currentDate.Format("20060102"), err)
			failedDates = append(failedDates, currentDate.Format("20060102"))
		} else {
			downloadedCount++
		}
		currentDate = currentDate.AddDate(0, 0, 1)
	}

	if downloadedCount == 0 {
		return fmt.Errorf("failed to download any MRMS data")
	}

	log.Printf("STEP 1 COMPLETE: Downloaded MRMS data for %d days", downloadedCount)

	// Step 2: Merge GRIB files
	log.Printf("STEP 2: Merging GRIB files...")

	// For now, using a dummy output DSS file path as requested
	outputDSS := "D:\\FloodaceDocuments\\HMS\\HMSBackend\\hms_models\\LeonCreek\\Rainfall\\RainfallHistorical.dss"

	// Execute the merge GRIB files batch script
	err = executeBatchFile(ctx,
		"D:/FloodaceDocuments/HMS/HMSBackend/python_scripts/Jython_Scripts/batchScripts/MergeGRIBFilesRealTimePass2Batch.bat",
		outputDir,
		"", // Empty string for shapefile_path to use default
		outputDSS,
	)

	if err != nil {
		return fmt.Errorf("failed to merge GRIB files: %w", err)
	}

	log.Printf("STEP 2 COMPLETE: Successfully merged GRIB files to: %s", outputDSS)

	// Step 3: Update the control file
	log.Printf("STEP 3: Updating control file with dates and times...")

	err = updateHistoricalControlFile(startDate, endDate, req.StartTime, req.EndTime)
	if err != nil {
		return fmt.Errorf("failed to update control file: %w", err)
	}

	log.Printf("STEP 3 COMPLETE: Successfully updated control file")

	// Step 4: Run HMS historical computation
	log.Printf("STEP 4: Running HMS historical computation...")

	// Build the command
	hmsExePath := "C:\\Program Files\\HEC\\HEC-HMS\\4.12\\HEC-HMS.cmd"
	scriptPath := "D:\\FloodaceDocuments\\HMS\\HMSBackend\\HMSScripts\\computeHistorical.script"
	hmsDir := "C:\\Program Files\\HEC\\HEC-HMS\\4.12"

	// Execute the HMS command from its directory
	cmd := exec.CommandContext(ctx, hmsExePath, "-script", scriptPath)
	cmd.Dir = hmsDir // Set working directory to HEC-HMS installation

	// Run the command and capture output
	output, err := cmd.CombinedOutput()

	if err != nil {
		// Check if it's an exit error to get the exit code
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode := exitErr.ExitCode()
			log.Printf("HMS computation failed with exit code %d. Output: %s", exitCode, string(output))
			return fmt.Errorf("HMS computation failed with exit code %d", exitCode)
		}
		log.Printf("HMS computation failed: %v. Output: %s", err, string(output))
		return fmt.Errorf("failed to run HMS computation: %w", err)
	}

	log.Printf("STEP 4 COMPLETE: HMS historical computation completed successfully")
	log.Printf("HMS output:\n%s", indentOutput(string(output)))

	log.Printf("INFO: Historical HMS pipeline completed successfully")
	return nil
}

// handleRunHMSPipelineHistorical handles the request to run the historical HMS processing pipeline
func handleRunHMSPipelineHistorical(c echo.Context) error {
	// Parse request body - using the existing HistoricalDownloadRequest structure
	var req HistoricalDownloadRequest
	if err := c.Bind(&req); err != nil {
		log.Printf("Error parsing historical pipeline request: %v", err)
		return respondWithError(c, http.StatusBadRequest, "Invalid request format")
	}

	// Basic validation
	if req.StartDate == "" || req.EndDate == "" {
		return respondWithError(c, http.StatusBadRequest, "start_date and end_date are required")
	}

	log.Printf("Received historical HMS pipeline request: start=%s, end=%s, start_time=%s, end_time=%s",
		req.StartDate, req.EndDate, req.StartTime, req.EndTime)

	// Run the pipeline in a goroutine to avoid blocking the HTTP response
	go func() {
		// Create a new context with a timeout
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Minute)
		defer cancel()

		// Run the complete historical pipeline
		err := runHMSPipelineHistorical(ctx, req)
		if err != nil {
			log.Printf("Historical HMS pipeline failed: %v", err)
		} else {
			log.Printf("Historical HMS pipeline completed successfully")
		}
	}()

	// Return a success response immediately
	return respondWithJSON(c, http.StatusAccepted, map[string]string{
		"message":    "Historical HMS processing pipeline started",
		"status":     "accepted",
		"start_date": req.StartDate,
		"end_date":   req.EndDate,
	})
}

// ExtractDSSDataRequest represents the request body for extracting DSS data
type ExtractDSSDataRequest struct {
	TargetBPart string `json:"target_b_part"` // e.g., "CUL-041"
	Month       string `json:"month"`         // e.g., "January"
	Year        string `json:"year"`          // e.g., "2025"
}

// runExtractDSSDataJython runs the Jython script to extract DSS data
func runExtractDSSDataJython(ctx context.Context, targetBPart, month, year string) error {
	log.Printf("INFO: Extracting DSS data for %s in %s %s", targetBPart, month, year)
	
	// Paths
	jythonPath := "C:\\Program Files\\HEC\\HEC-DSSVue\\Jython.bat"
	scriptPath := "D:\\FloodaceDocuments\\HMS\\HMSBackend\\python_scripts\\Jython_Scripts\\extract_dss_data_historical.py"
	
	// Build command with arguments
	cmd := exec.CommandContext(ctx, jythonPath, scriptPath, targetBPart, month, year)
	
	// Run the command and capture output
	output, err := cmd.CombinedOutput()
	
	if err != nil {
		// Check if it's an exit error to get the exit code
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode := exitErr.ExitCode()
			log.Printf("Jython script failed with exit code %d. Output: %s", exitCode, string(output))
			return fmt.Errorf("Jython script failed with exit code %d", exitCode)
		}
		log.Printf("Jython script failed: %v. Output: %s", err, string(output))
		return fmt.Errorf("failed to run Jython script: %w", err)
	}
	
	log.Printf("Successfully extracted DSS data. Output:\n%s", indentOutput(string(output)))
	return nil
}

// handleExtractHistoricalDSSData handles the request to extract historical DSS data
func handleExtractHistoricalDSSData(c echo.Context) error {
	// Parse request body
	var req ExtractDSSDataRequest
	if err := c.Bind(&req); err != nil {
		log.Printf("Error parsing extract DSS data request: %v", err)
		return respondWithError(c, http.StatusBadRequest, "Invalid request format")
	}
	
	// Validate required fields
	if req.TargetBPart == "" || req.Month == "" || req.Year == "" {
		return respondWithError(c, http.StatusBadRequest, "target_b_part, month, and year are required")
	}
	
	log.Printf("Received extract DSS data request: target_b_part=%s, month=%s, year=%s", 
		req.TargetBPart, req.Month, req.Year)
	
	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	
	// Run the Jython script
	err := runExtractDSSDataJython(ctx, req.TargetBPart, req.Month, req.Year)
	if err != nil {
		log.Printf("Failed to extract DSS data: %v", err)
		return respondWithError(c, http.StatusInternalServerError, "Failed to extract DSS data")
	}
	
	// Read the generated JSON file
	jsonFilePath := "D:\\FloodaceDocuments\\HMS\\HMSBackend\\JSON\\outputHistorical.json"
	jsonData, err := os.ReadFile(jsonFilePath)
	if err != nil {
		log.Printf("Failed to read output JSON file: %v", err)
		return respondWithError(c, http.StatusInternalServerError, "Failed to read extracted data")
	}
	
	// Parse the JSON to verify it's valid
	var result map[string]interface{}
	if err := json.Unmarshal(jsonData, &result); err != nil {
		log.Printf("Failed to parse output JSON: %v", err)
		return respondWithError(c, http.StatusInternalServerError, "Invalid JSON output")
	}
	
	// Return the JSON data
	return c.JSON(http.StatusOK, result)
}
