package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const (
	sourceFilePath      = `D:\FloodaceDocuments\HMS\HMSGit\HEC-HMS-Floodace\hms_models\LeonCreek\RainrealTime.dss`
	archiveDirectory    = `D:\FloodaceDocuments\HMS\HMSGit\HEC-HMS-Floodace\hms_models\LeonCreek\dssArchive`
	hmsPipelineEndpoint = "/api/run-hms-pipeline" // Relative to server base URL
)

// archiveFileAndTriggerPipeline archives the specified file, deletes the original,
// and then triggers the HMS pipeline.
func archiveFileAndTriggerPipeline() {
	log.Println("Scheduler: Starting archive and pipeline trigger process...")

	// Ensure archive directory exists
	if err := os.MkdirAll(archiveDirectory, 0755); err != nil {
		log.Printf("Scheduler: Error creating archive directory %s: %v\n", archiveDirectory, err)
		return
	}

	// Get current hour for the filename
	currentTime := time.Now()
	hourStr := currentTime.Format("15") // HH format (24-hour)

	// Construct archive file name: RainrealTime_HH.dss
	archiveFileName := fmt.Sprintf("%s_%s%s", filepath.Base(sourceFilePath[:len(sourceFilePath)-len(filepath.Ext(sourceFilePath))]), hourStr, filepath.Ext(sourceFilePath))
	archiveFilePath := filepath.Join(archiveDirectory, archiveFileName)

	log.Printf("Scheduler: Archiving %s to %s\n", sourceFilePath, archiveFilePath)

	// 1. Copy the file
	srcFile, err := os.Open(sourceFilePath)
	if err != nil {
		log.Printf("Scheduler: Error opening source file %s: %v\n", sourceFilePath, err)
		return // Don't proceed if source file can't be opened
	}

	dstFile, err := os.Create(archiveFilePath)
	if err != nil {
		srcFile.Close()
		log.Printf("Scheduler: Error creating archive file %s: %v\n", archiveFilePath, err)
		return
	}

	_, err = io.Copy(dstFile, srcFile)
	if err != nil {
		log.Printf("Scheduler: Error copying file to archive %s: %v\n", archiveFilePath, err)
		// Attempt to clean up partially created archive file
		srcFile.Close()
		dstFile.Close()
		os.Remove(archiveFilePath)
		return
	}
	
	// Close both files before attempting deletion
	srcFile.Close()
	dstFile.Close()
	
	log.Printf("Scheduler: File successfully archived to %s\n", archiveFilePath)

	// 2. Delete the original file with retry mechanism for Windows
	var deleteErr error
	for attempts := 0; attempts < 5; attempts++ {
		deleteErr = os.Remove(sourceFilePath)
		if deleteErr == nil {
			log.Printf("Scheduler: Original file %s deleted successfully\n", sourceFilePath)
			break
		}
		
		// If it's a "file in use" error, wait a bit and retry
		if os.IsPermission(deleteErr) || os.IsNotExist(deleteErr) {
			// File doesn't exist or permanent permission issue, no point retrying
			break
		}
		
		log.Printf("Scheduler: Attempt %d to delete file failed: %v. Retrying...\n", attempts+1, deleteErr)
		time.Sleep(100 * time.Millisecond) // Wait 100ms before retry
	}
	
	if deleteErr != nil {
		log.Printf("Scheduler: Error deleting original file %s after retries: %v\n", sourceFilePath, deleteErr)
		// Log error but proceed to call API, as archiving was successful.
		// Depending on requirements, this could be a critical failure.
	}

	// 3. Trigger the HMS pipeline API
	serverPort := os.Getenv("SERVER_PORT")
	if serverPort == "" {
		log.Println("Scheduler: Error: SERVER_PORT environment variable not set. Cannot call API.")
		return
	}
	apiURL := fmt.Sprintf("https://localhost:%s%s", serverPort, hmsPipelineEndpoint)

	log.Printf("Scheduler: Calling HMS pipeline API: %s\n", apiURL)
	// We can send an empty JSON body or specific parameters if the API expects them.
	// The current handleRunHMSPipeline in main.go can accept an empty body.
	reqBody := []byte("{}")
	resp, err := http.Post(apiURL, "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		log.Printf("Scheduler: Error calling HMS pipeline API %s: %v\n", apiURL, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusAccepted || resp.StatusCode == http.StatusOK {
		log.Printf("Scheduler: HMS pipeline API called successfully, status: %s\n", resp.Status)
	} else {
		bodyBytes, _ := io.ReadAll(resp.Body)
		log.Printf("Scheduler: HMS pipeline API call failed, status: %s, response: %s\n", resp.Status, string(bodyBytes))
	}
	log.Println("Scheduler: Archive and pipeline trigger process finished.")
}

// StartScheduler runs a task at HH:15 every hour.
func StartScheduler() {
	log.Println("Scheduler: Initializing...")
	go func() {
		for {
			now := time.Now()
			nextRun := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), 15, 0, 0, now.Location())
			//nextRun := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), 15, 0, 0, now.Location())

			if now.Minute() >= 15 {
				// If current time is past HH:15, schedule for next hour's HH:15
				nextRun = nextRun.Add(time.Hour)
			}

			sleepDuration := nextRun.Sub(now)
			log.Printf("Scheduler: Next run at %s (sleeping for %v)\n", nextRun.Format("2006-01-02 15:04:05"), sleepDuration)

			time.Sleep(sleepDuration)

			// Check if source file exists before running
			if _, err := os.Stat(sourceFilePath); os.IsNotExist(err) {
				log.Printf("Scheduler: Source file %s does not exist. Skipping this run.\n", sourceFilePath)
			} else {
				archiveFileAndTriggerPipeline()
			}
		}
	}()
	log.Println("Scheduler: Goroutine started. Will run tasks at HH:15.")
}
