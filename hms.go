package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io" // Added for io.ReadAll
	"log"
	"net/http"
	"os"
	"time"
)

// RunProcessingPipeline orchestrates a sequence of HTTP POST requests to a Flask backend
// It accepts an optional date in YYYYMMDD format and an optional run hour in HH format
func RunProcessingPipeline(ctx context.Context, optionalDateYYYYMMDD string, optionalRunHourHH string) error {
	// Get environment variables for each endpoint URL
	downloadGribURL := os.Getenv("PYTHON_DOWNLOAD_GRIB_URL")
	downloadHrrrGribURL := os.Getenv("PYTHON_DOWNLOAD_HRRR_GRIB_URL")
	mergeGribURL := os.Getenv("PYTHON_MERGE_GRIB_URL")
	mergeHrrrGribURL := os.Getenv("PYTHON_MERGE_HRRR_GRIB_URL")
	combineDssURL := os.Getenv("PYTHON_COMBINE_DSS_URL")
	updateControlURL := os.Getenv("PYTHON_UPDATE_CONTROL_URL")
	runHmsURL := os.Getenv("PYTHON_RUN_HMS_URL")

	// Check if all required environment variables are set
	missingVars := []string{}
	if downloadGribURL == "" {
		missingVars = append(missingVars, "PYTHON_DOWNLOAD_GRIB_URL")
	}
	if downloadHrrrGribURL == "" {
		missingVars = append(missingVars, "PYTHON_DOWNLOAD_HRRR_GRIB_URL")
	}
	if mergeGribURL == "" {
		missingVars = append(missingVars, "PYTHON_MERGE_GRIB_URL")
	}
	if mergeHrrrGribURL == "" {
		missingVars = append(missingVars, "PYTHON_MERGE_HRRR_GRIB_URL")
	}
	if combineDssURL == "" {
		missingVars = append(missingVars, "PYTHON_COMBINE_DSS_URL")
	}
	if updateControlURL == "" {
		missingVars = append(missingVars, "PYTHON_UPDATE_CONTROL_URL")
	}
	if runHmsURL == "" {
		missingVars = append(missingVars, "PYTHON_RUN_HMS_URL")
	}

	if len(missingVars) > 0 {
		return fmt.Errorf("missing required environment variables: %v", missingVars)
	}

	// Create HTTP client with a reasonable timeout
	client := &http.Client{
		Timeout: 30 * time.Minute, // Increased timeout for potentially very long operations
	}

	// --- Date Calculation (used for download steps if not provided) ---
	dateToUse := optionalDateYYYYMMDD
	if dateToUse == "" {
		// Default to current local date
		dateToUse = time.Now().Format("20060102") // YYYYMMDD format
		log.Printf("INFO: No date provided for downloads, using current local date: %s", dateToUse)
	} else {
		log.Printf("INFO: Using provided date for downloads: %s", dateToUse)
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

	var err error // Declare error variable to be reused

	// 1. Download GRIB Files
	log.Println("STEP 1: Downloading GRIB files...")
	gribPayload := map[string]string{"date": dateToUse}
	err = makePostRequest(ctx, client, downloadGribURL, gribPayload)
	if err != nil {
		return fmt.Errorf("failed at step 1 (download_grib): %w", err)
	}
	log.Println("STEP 1: Download GRIB files completed successfully.")

	// 2. Download HRRR GRIB Files
	log.Println("STEP 2: Downloading HRRR GRIB files...")
	hrrrDownloadPayload := map[string]string{
		"target_date": dateToUse,
		"run_hour":    runHourToUse,
	}
	err = makePostRequest(ctx, client, downloadHrrrGribURL, hrrrDownloadPayload)
	if err != nil {
		return fmt.Errorf("failed at step 2 (download_hrrr_grib): %w", err)
	}
	log.Println("STEP 2: Download HRRR GRIB files completed successfully.")

	// Define an empty payload for steps that don't require specific data
	emptyPayload := map[string]interface{}{} // Use interface{} for truly empty JSON {}

	// 3. Merge GRIB Files
	log.Println("STEP 3: Merging GRIB files...")
	// Sending an empty payload as the date logic is handled by the Flask endpoint.
	err = makePostRequest(ctx, client, mergeGribURL, emptyPayload)
	if err != nil {
		return fmt.Errorf("failed at step 3 (merge_grib): %w", err)
	}
	log.Println("STEP 3: Merge GRIB files completed successfully.")

	// 4. Merge HRRR GRIB Files
	log.Println("STEP 4: Merging HRRR GRIB files...")
	// Sending an empty payload as the date logic is handled by the Flask endpoint.
	err = makePostRequest(ctx, client, mergeHrrrGribURL, emptyPayload)
	if err != nil {
		return fmt.Errorf("failed at step 4 (merge_hrrr_grib): %w", err)
	}
	log.Println("STEP 4: Merge HRRR GRIB files completed successfully.")

	// 5. Combine DSS Records
	log.Println("STEP 5: Combining DSS records...")
	err = makePostRequest(ctx, client, combineDssURL, emptyPayload) // Reuse emptyPayload
	if err != nil {
		return fmt.Errorf("failed at step 5 (combine_dss): %w", err)
	}
	log.Println("STEP 5: Combine DSS records completed successfully.")

	// 6. Update Control File
	log.Println("STEP 6: Updating control file...")
	err = makePostRequest(ctx, client, updateControlURL, emptyPayload) // Reuse emptyPayload
	if err != nil {
		return fmt.Errorf("failed at step 6 (update_control): %w", err)
	}
	log.Println("STEP 6: Update control file completed successfully.")

	// 7. Run HMS
	log.Println("STEP 7: Running HMS...")
	// Sending an empty payload as the run configuration is handled by the Flask endpoint.
	err = makePostRequest(ctx, client, runHmsURL, emptyPayload) // Reuse emptyPayload
	if err != nil {
		return fmt.Errorf("failed at step 7 (run_hms): %w", err)
	}
	log.Println("STEP 7: Run HMS completed successfully (or with partial success if status was 207).")

	log.Println("INFO: All processing steps triggered successfully!")
	return nil
}

// makePostRequest is a helper function to make a POST request with a JSON payload
func makePostRequest(ctx context.Context, client *http.Client, url string, payload interface{}) error {
	// Convert payload to JSON
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON payload for %s: %w", url, err)
	}

	log.Printf("INFO: Sending POST request to %s with payload: %s", url, string(jsonPayload))

	// Create request with context
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return fmt.Errorf("failed to create request for %s: %w", url, err)
	}
	req.Header.Set("Content-Type", "application/json") // Crucial for Flask's request.get_json()

	// Send request
	resp, err := client.Do(req)
	if err != nil {
		// This typically catches network errors, DNS resolution errors, etc.
		return fmt.Errorf("request to %s failed: %w", url, err)
	}
	defer resp.Body.Close() // Ensure the response body is always closed

	log.Printf("INFO: Received response from %s with status: %s", url, resp.Status)

	// Check status code (2xx is considered success, includes 207 Multi-Status for HMS)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		responseBodyBytes, readErr := io.ReadAll(resp.Body) // Read the full error body
		if readErr != nil {
			log.Printf("WARN: Failed to read error response body from %s (status %d): %v", url, resp.StatusCode, readErr)
			return fmt.Errorf("request to %s returned status %d, and failed to read response body", url, resp.StatusCode)
		}
		log.Printf("ERROR: Request to %s failed with status %d. Response body: %s", url, resp.StatusCode, string(responseBodyBytes))
		return fmt.Errorf("request to %s returned status %d: %s", url, resp.StatusCode, string(responseBodyBytes))
	}

	// If successful, it's good practice to drain the response body to allow connection reuse,
	// especially if not reading the body content for success cases.
	_, err = io.Copy(io.Discard, resp.Body)
	if err != nil {
		log.Printf("WARN: Failed to discard response body from %s: %v", url, err)
		// Not returning error here as the main operation was successful.
	}

	return nil
}
