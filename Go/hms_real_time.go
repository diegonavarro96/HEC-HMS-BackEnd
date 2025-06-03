package main

import (
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
)

// pythonExePath and jythonExePath are now retrieved from config
// Use GetPythonPath("hms") and GetJythonPath() instead

// executePythonScript is a helper function to execute a Python script
func executePythonScript(ctx context.Context, scriptPath string, scriptArgs ...string) error {
	absScriptPath, err := filepath.Abs(scriptPath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path for script %s: %w", scriptPath, err)
	}

	cmdArgs := append([]string{absScriptPath}, scriptArgs...)
	cmd := exec.CommandContext(ctx, GetPythonPath("hms"), cmdArgs...)

	log.Printf("INFO: Executing command: %s %s", GetPythonPath("hms"), strings.Join(cmdArgs, " "))

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

	cmd := exec.CommandContext(ctx, GetJythonPath(), absScriptPath)

	log.Printf("INFO: Executing command: %s %s", GetJythonPath(), absScriptPath)

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

// executeBatchFile is a helper function to execute a batch file or shell script
func executeBatchFile(ctx context.Context, batchPath string, batchArgs ...string) error {
	absBatchPath, err := filepath.Abs(batchPath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path for batch file %s: %w", batchPath, err)
	}

	// Determine the appropriate shell command based on the operating system
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		// Windows: expect .bat files
		if !strings.HasSuffix(absBatchPath, ".bat") {
			return fmt.Errorf("on Windows, expected .bat file but got: %s", batchPath)
		}
		cmdArgs := append([]string{"/c", absBatchPath}, batchArgs...)
		cmd = exec.CommandContext(ctx, "cmd.exe", cmdArgs...)
	} else {
		// Linux/Unix: expect .sh files
		if !strings.HasSuffix(absBatchPath, ".sh") {
			return fmt.Errorf("on Linux/Unix, expected .sh file but got: %s", batchPath)
		}
		// Make sure the script is executable
		if err := os.Chmod(absBatchPath, 0755); err != nil {
			log.Printf("Warning: Failed to set executable permission on %s: %v", absBatchPath, err)
		}
		cmdArgs := append([]string{absBatchPath}, batchArgs...)
		cmd = exec.CommandContext(ctx, "bash", cmdArgs...)
	}

	// Set working directory to the directory containing the batch file
	// This ensures relative paths in the batch file work correctly
	cmd.Dir = filepath.Dir(absBatchPath)

	log.Printf("INFO: Executing script: %s", cmd.String())

	output, err := cmd.CombinedOutput() // Captures both stdout and stderr

	if len(output) > 0 {
		// Log output, prefixing each line for clarity
		log.Printf("INFO: Output from %s:\n%s", batchPath, indentOutput(string(output)))
	}

	if err != nil {
		// If there was an error, CombinedOutput() might still contain useful error messages
		return fmt.Errorf("failed to execute script %s (resolved to %s): %w. Output: %s", batchPath, absBatchPath, err, string(output))
	}

	log.Printf("INFO: Script %s (resolved to %s) completed successfully.", batchPath, absBatchPath)
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

// GRIBDownloadConfig holds configuration for GRIB file downloads
type GRIBDownloadConfig struct {
	BaseURLRealtime string
	BaseURLArchive  string
	OutputDir       string
	HoursBack       int
	DaysBack        int
}

// downloadAndExtractGzFile downloads a gzipped file and extracts it
func downloadAndExtractGzFile(url string, destPath string) error {
	// Create the destination directory if it doesn't exist
	destDir := filepath.Dir(destPath)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Download the file
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to download file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned status %d", resp.StatusCode)
	}

	// Check if content is HTML (error page)
	contentType := resp.Header.Get("Content-Type")
	if strings.Contains(contentType, "text/html") {
		return fmt.Errorf("received HTML instead of GRIB file")
	}

	// Determine if the file is gzipped based on extension
	isGzipped := strings.HasSuffix(url, ".gz")
	finalPath := destPath

	if isGzipped {
		// If gzipped, remove .gz extension from final path
		finalPath = strings.TrimSuffix(destPath, ".gz")

		// Create gzip reader
		gzReader, err := gzip.NewReader(resp.Body)
		if err != nil {
			return fmt.Errorf("failed to create gzip reader: %w", err)
		}
		defer gzReader.Close()

		// Create output file
		outFile, err := os.Create(finalPath)
		if err != nil {
			return fmt.Errorf("failed to create output file: %w", err)
		}
		defer outFile.Close()

		// Copy uncompressed data
		if _, err := io.Copy(outFile, gzReader); err != nil {
			return fmt.Errorf("failed to extract file: %w", err)
		}
	} else {
		// Not gzipped, save directly
		outFile, err := os.Create(destPath)
		if err != nil {
			return fmt.Errorf("failed to create output file: %w", err)
		}
		defer outFile.Close()

		if _, err := io.Copy(outFile, resp.Body); err != nil {
			return fmt.Errorf("failed to save file: %w", err)
		}
	}

	log.Printf("Successfully downloaded and extracted: %s", filepath.Base(finalPath))
	return nil
}

// parseGRIBFilename extracts timestamp from GRIB filename
func parseGRIBFilename(filename string) (time.Time, error) {
	// Pattern: _YYYYMMDD-HHMMSS.grib2
	re := regexp.MustCompile(`_(\d{8})-(\d{6})\.grib2`)
	matches := re.FindStringSubmatch(filename)
	if len(matches) != 3 {
		return time.Time{}, fmt.Errorf("filename doesn't match expected pattern")
	}

	timeStr := matches[1] + matches[2]
	return time.Parse("20060102150405", timeStr)
}

// fetchDirectoryListing fetches and parses directory listing from URL
func fetchDirectoryListing(url string) ([]string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch directory listing: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Parse HTML to find links
	var links []string
	// Simple regex to find href links - could use html parser for more robustness
	re := regexp.MustCompile(`href="([^"]+\.grib2(?:\.gz)?)"`)
	matches := re.FindAllStringSubmatch(string(body), -1)

	for _, match := range matches {
		if len(match) > 1 {
			links = append(links, match[1])
		}
	}

	return links, nil
}

// downloadGRIBFilesRealtime downloads GRIB files from real-time source
func downloadGRIBFilesRealtime(config GRIBDownloadConfig, dateStr string) error {
	log.Printf("INFO: Downloading real-time GRIB files for date: %s", dateStr)
	log.Printf("INFO: Real-time window: last %d hours", config.HoursBack)

	// Clear existing files in output directory
	if _, err := os.Stat(config.OutputDir); err == nil {
		log.Printf("INFO: Clearing existing files in %s", config.OutputDir)
		files, _ := os.ReadDir(config.OutputDir)
		for _, file := range files {
			filePath := filepath.Join(config.OutputDir, file.Name())
			if err := os.Remove(filePath); err != nil {
				log.Printf("Warning: Failed to remove %s: %v", filePath, err)
			}
		}
	}

	// Fetch directory listing
	links, err := fetchDirectoryListing(config.BaseURLRealtime)
	if err != nil {
		return fmt.Errorf("failed to fetch real-time directory listing: %w", err)
	}

	if len(links) == 0 {
		log.Printf("INFO: No files found in real-time directory")
		return nil
	}

	// Calculate cutoff time
	cutoffTime := time.Now().UTC().Add(-time.Duration(config.HoursBack) * time.Hour)
	downloadCount := 0

	for _, link := range links {
		// Parse timestamp from filename
		fileTime, err := parseGRIBFilename(link)
		if err != nil {
			continue
		}

		// Skip if older than cutoff
		if fileTime.Before(cutoffTime) {
			continue
		}

		// Construct full URL and destination path
		fileURL := config.BaseURLRealtime + link
		destPath := filepath.Join(config.OutputDir, link)

		// Check if already exists (without .gz extension if applicable)
		finalPath := strings.TrimSuffix(destPath, ".gz")
		if _, err := os.Stat(finalPath); err == nil {
			continue
		}

		// Download and extract
		if err := downloadAndExtractGzFile(fileURL, destPath); err != nil {
			log.Printf("Warning: Failed to download %s: %v", link, err)
			continue
		}
		downloadCount++
	}

	log.Printf("INFO: Downloaded %d real-time files", downloadCount)
	return nil
}

// downloadGRIBFilesArchive downloads GRIB files from archive source
func downloadGRIBFilesArchive(config GRIBDownloadConfig, dateStr string) error {
	log.Printf("INFO: Downloading archive GRIB files")
	log.Printf("INFO: Archive window: 24-48 hours ago")

	baseDate, err := time.Parse("20060102", dateStr)
	if err != nil {
		return fmt.Errorf("invalid date format: %w", err)
	}

	totalDownloaded := 0

	// Calculate time window for archive files (24-48 hours ago)
	now := time.Now().UTC()
	cutoffStart := now.Add(-48 * time.Hour)
	cutoffEnd := now.Add(-24 * time.Hour)

	// Download for each day going back
	for d := 0; d <= config.DaysBack; d++ {
		targetDate := baseDate.AddDate(0, 0, -d)

		// Construct archive URL with date
		year := targetDate.Format("2006")
		month := targetDate.Format("01")
		day := targetDate.Format("02")
		dayURL := fmt.Sprintf("%s%s/%s/%s/mrms/ncep/MultiSensor_QPE_01H_Pass2/", config.BaseURLArchive, year, month, day)
		log.Printf("Day URL: %s", dayURL)

		log.Printf("INFO: Checking archive for %s", targetDate.Format("2006-01-02"))

		// Fetch directory listing
		links, err := fetchDirectoryListing(dayURL)
		if err != nil {
			log.Printf("Warning: Failed to fetch archive listing for %s: %v", targetDate.Format("2006-01-02"), err)
			continue
		}

		if len(links) == 0 {
			log.Printf("INFO: No files found for %s", targetDate.Format("2006-01-02"))
			continue
		}

		// Download each file
		for _, link := range links {
			// Parse timestamp from filename to filter by time window
			fileTime, err := parseGRIBFilename(link)
			if err != nil {
				continue
			}

			// Only download files within the 24-48 hour window
			if fileTime.Before(cutoffStart) || fileTime.After(cutoffEnd) {
				continue
			}

			fileURL := dayURL + link
			destPath := filepath.Join(config.OutputDir, link)

			// Check if already exists
			finalPath := strings.TrimSuffix(destPath, ".gz")
			if _, err := os.Stat(finalPath); err == nil {
				continue
			}

			// Download and extract
			if err := downloadAndExtractGzFile(fileURL, destPath); err != nil {
				log.Printf("Warning: Failed to download %s: %v", link, err)
				continue
			}
			totalDownloaded++
		}
	}

	log.Printf("INFO: Downloaded %d archive files", totalDownloaded)
	return nil
}

// downloadGRIBFiles is the main function that replaces the Python script
func downloadGRIBFiles(dateStr string, includeYesterday bool) error {
	// Use current date if not provided
	if dateStr == "" {
		dateStr = time.Now().Format("20060102")
	}

	// Configure download parameters
	config := GRIBDownloadConfig{
		BaseURLRealtime: AppConfig.URLs.MRMSPass1,
		BaseURLArchive:  AppConfig.URLs.MRMSArchive,
		OutputDir:       GetGribDownloadPath(dateStr),
		HoursBack:       24, // Real-time: last 24 hours
		DaysBack:        2,  // Archive: need to check 2 days back to ensure we cover 24-48 hours ago
	}

	if !includeYesterday {
		config.DaysBack = 0
	}

	// Download from real-time source
	if err := downloadGRIBFilesRealtime(config, dateStr); err != nil {
		log.Printf("Error downloading real-time files: %v", err)
	}

	// Download from archive source
	if err := downloadGRIBFilesArchive(config, dateStr); err != nil {
		log.Printf("Error downloading archive files: %v", err)
	}

	return nil
}

// downloadHRRRForecastGRIB downloads HRRR forecast GRIB files for a specific date and run hour
func downloadHRRRForecastGRIB(dateStr string, runHour string) error {
	// Validate inputs
	if len(dateStr) != 8 {
		return fmt.Errorf("invalid date format: %s, expected YYYYMMDD", dateStr)
	}

	if len(runHour) != 2 {
		return fmt.Errorf("invalid run hour format: %s, expected HH", runHour)
	}

	hour, err := strconv.Atoi(runHour)
	if err != nil || hour < 0 || hour > 23 {
		return fmt.Errorf("invalid run hour: %s, must be 00-23", runHour)
	}

	// Create output directory
	outputDir := GetGribDownloadPath(dateStr)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	log.Printf("INFO: Downloading HRRR forecast files for date=%s, run_hour=%s", dateStr, runHour)

	// Base URL for HRRR data
	baseURL := fmt.Sprintf("%shrrr.%s/conus/", AppConfig.URLs.HRRRDataSource, dateStr)

	// Download forecast hours 02 through 12
	downloadedCount := 0
	totalFiles := 11 // hours 02 through 12 inclusive

	for fh := 2; fh <= 12; fh++ {
		// Format filename
		filename := fmt.Sprintf("hrrr.t%sz.wrfsfcf%02d.grib2", runHour, fh)
		fileURL := baseURL + filename
		localPath := filepath.Join(outputDir, filename)

		// Check if file already exists
		if _, err := os.Stat(localPath); err == nil {
			log.Printf("File already exists, skipping: %s", localPath)
			downloadedCount++
			continue
		}

		// Download file
		log.Printf("Downloading HRRR forecast hour %02d: %s", fh, filename)

		resp, err := http.Get(fileURL)
		if err != nil {
			log.Printf("Warning: Error downloading %s: %v", filename, err)
			continue // Skip to next file instead of breaking
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusNotFound {
			log.Printf("Warning: File not found (404) for %s - this is normal if the forecast hasn't been generated yet", filename)
			resp.Body.Close()
			continue // Skip to next file, this is expected for recent forecasts
		}

		if resp.StatusCode != http.StatusOK {
			log.Printf("Warning: Failed to download %s: server returned status %d", filename, resp.StatusCode)
			resp.Body.Close()
			continue // Skip to next file instead of breaking
		}

		// Create output file
		outFile, err := os.Create(localPath)
		if err != nil {
			log.Printf("Failed to create file %s: %v", localPath, err)
			resp.Body.Close()
			break
		}

		// Copy data
		_, err = io.Copy(outFile, resp.Body)
		outFile.Close()
		resp.Body.Close()

		if err != nil {
			log.Printf("Failed to save file %s: %v", localPath, err)
			os.Remove(localPath) // Clean up partial file
			break
		}

		log.Printf("Successfully downloaded: %s", filename)
		downloadedCount++
	}

	if downloadedCount == totalFiles {
		log.Printf("INFO: All %d HRRR forecast files downloaded successfully for %s t%sz", downloadedCount, dateStr, runHour)
	} else {
		log.Printf("WARNING: Downloaded %d out of %d HRRR forecast files for %s t%sz", downloadedCount, totalFiles, dateStr, runHour)
	}

	return nil
}

// updateControlFile updates the HMS control file with current date and time settings
func updateControlFile() error {
	controlFilePath := GetHMSControlFile("realtime")

	log.Printf("setControlFile: Updating control file at: %s", controlFilePath)

	// Get the current time in UTC and round down to the hour
	nowUTC := time.Now().UTC().Truncate(time.Hour)
	log.Printf("setControlFile: Current UTC time (rounded down): %s", nowUTC.Format("2006-01-02 15:04:05"))

	// Calculate start datetime (47 hours before current UTC time)
	startDateTime := nowUTC.Add(-47 * time.Hour)
	startTimeStr := startDateTime.Format("15:04")
	startDateStr := startDateTime.Format("2 January 2006") // Day without leading zero

	// Calculate end datetime (12 hours after current UTC time)
	endDateTime := nowUTC.Add(12 * time.Hour)
	endTimeStr := endDateTime.Format("15:04")
	endDateStr := endDateTime.Format("2 January 2006") // Day without leading zero

	log.Printf("setControlFile: Calculated Start: %s %s (UTC-47h)", startDateStr, startTimeStr)
	log.Printf("setControlFile: Calculated End:   %s %s (UTC+12h)", endDateStr, endTimeStr)

	// Read the control file
	content, err := os.ReadFile(controlFilePath)
	if err != nil {
		return fmt.Errorf("failed to read control file: %w", err)
	}

	// Process the file line by line
	lines := strings.Split(string(content), "\n")
	var updatedLines []string

	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmedLine, "Start Date:"):
			updatedLines = append(updatedLines, fmt.Sprintf("     Start Date: %s", startDateStr))
		case strings.HasPrefix(trimmedLine, "End Date:"):
			updatedLines = append(updatedLines, fmt.Sprintf("     End Date: %s", endDateStr))
		case strings.HasPrefix(trimmedLine, "Start Time:"):
			updatedLines = append(updatedLines, fmt.Sprintf("     Start Time: %s", startTimeStr))
		case strings.HasPrefix(trimmedLine, "End Time:"):
			updatedLines = append(updatedLines, fmt.Sprintf("     End Time: %s", endTimeStr))
		default:
			updatedLines = append(updatedLines, line)
		}
	}

	// Write the updated content back to the file
	updatedContent := strings.Join(updatedLines, "\n")
	err = os.WriteFile(controlFilePath, []byte(updatedContent), 0644)
	if err != nil {
		return fmt.Errorf("failed to write control file: %w", err)
	}

	log.Printf("setControlFile: Successfully updated control file")
	return nil
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

	// Step 1: Download GRIB files using Go function
	log.Printf("STEP 1: Running 'Get GRIB2 Files RealTime'...")
	err = downloadGRIBFiles(dateToUse, true) // includeYesterday = true
	if err != nil {
		return fmt.Errorf("failed at step 1 (Get GRIB2 Files RealTime): %w", err)
	}
	log.Printf("STEP 1: 'Get GRIB2 Files RealTime' completed successfully.")
	log.Printf("INFO: Waiting 300ms before next task...")
	time.Sleep(1000 * time.Millisecond)

	// Step 2: Download HRRR forecast GRIB files using Go function
	log.Printf("STEP 2: Running 'Get HRRR Forecast GRIB'...")
	err = downloadHRRRForecastGRIB(dateToUse, runHourToUse)
	if err != nil {
		return fmt.Errorf("failed at step 2 (Get HRRR Forecast GRIB): %w", err)
	}
	log.Printf("STEP 2: 'Get HRRR Forecast GRIB' completed successfully.")
	log.Printf("INFO: Waiting 300ms before next task...")
	time.Sleep(1000 * time.Millisecond)

	// Script execution steps (starting from step 3)
	scriptsToRun := []struct {
		name     string
		path     string
		isBatch  bool
		argsFunc func() []string // Function to generate args, allows use of dateToUse/runHourToUse
	}{
		{
			name:    "Merge GRIB Files RealTime",
			path:    GetJythonBatchScriptPath("MergeGRIBFilesRealTimeBatch.bat"),
			isBatch: true,
			argsFunc: func() []string {
				// Pass the full folder path to the batch file
				return []string{GetGribDownloadPath(dateToUse)}
			},
		},
		{
			name:    "Merge GRIB Files RealTime Pass 2",
			path:    GetJythonBatchScriptPath("MergeGRIBFilesRealTimePass2Batch.bat"),
			isBatch: true,
			argsFunc: func() []string {
				// Pass the arguments as separate elements
				return []string{
					GetGribDownloadPath(dateToUse),
					"", // Empty string for shapefile_path to use default
					GetDSSPath("RainfallRealTimePass2.dss"),
				}
			},
		},
		{
			name:    "Merge GRIB Files Forcast",
			path:    GetJythonBatchScriptPath("MergeGRIBFilesRealTimeHRRBatch.bat"),
			isBatch: true,
			argsFunc: func() []string {
				// Pass the full folder path to the batch file
				return []string{GetGribDownloadPath(dateToUse)}
			},
		},
		{
			name:    "Combine DSS Records Pass1 Pass2",
			path:    GetJythonBatchScriptPath("CombineTwoDssFilesPass1Pass2Batch.bat"),
			isBatch: true,
			argsFunc: func() []string {
				return []string{
					GetDSSPath("RainfallRealTime.dss"),
					GetDSSPath("RainfallRealTimePass2.dss"),
					GetDSSPath("RainfallRealTimePass1And2.dss"),
				}
			},
		},
		{
			name:    "Combine DSS Records Realtime Pass1 Pass2 and HRR",
			path:    GetJythonBatchScriptPath("CombineTwoDssFilesRealTimeAndHRRBatch.bat"),
			isBatch: true,
			argsFunc: func() []string {
				return []string{
					GetDSSPath("RainfallRealTimePass1And2.dss"),
					GetDSSPath("HRR.dss"),
					GetDSSPath("RainfallRealTimeAndForcast.dss"),
				}
			},
		},
		// Step removed - HMS execution will be done separately after the loop
	}

	for i, script := range scriptsToRun {
		stepNum := i + 3 // Starting from step 3 since steps 1 and 2 are now handled by Go
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

	// Step: Update Control File using Go function
	controlFileStepNum := len(scriptsToRun) + 3
	log.Printf("STEP %d: Running 'Set Control File'...", controlFileStepNum)
	err = updateControlFile()
	if err != nil {
		return fmt.Errorf("failed at step %d (Set Control File): %w", controlFileStepNum, err)
	}
	log.Printf("STEP %d: 'Set Control File' completed successfully.", controlFileStepNum)
	log.Printf("INFO: Waiting 300ms before next task...")
	time.Sleep(1000 * time.Millisecond)

	// Final step: Run HMS RealTime computation
	finalStepNum := controlFileStepNum + 1
	log.Printf("STEP %d: Running 'HMS RealTime Computation'...", finalStepNum)

	// Use batch/shell script for HMS execution
	// GetHMSBatchScriptPath will automatically choose .bat or .sh based on OS
	batchPath := GetHMSBatchScriptPath("HMSRealTimeBatch.bat")
	scriptPath := GetHMSScript("realtime")
	
	err = executeBatchFile(ctx, batchPath, scriptPath)
	if err != nil {
		return fmt.Errorf("failed at step %d (HMS RealTime Computation): %w", finalStepNum, err)
	}
	
	log.Printf("STEP %d: 'HMS RealTime Computation' completed successfully.", finalStepNum)

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
