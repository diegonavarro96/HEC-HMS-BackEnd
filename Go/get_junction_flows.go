package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/labstack/echo/v4"
)

// ProcessAllJunctionFlows executes the Jython script to generate all junction flow data
func ProcessAllJunctionFlows() error {
	// Execute the Jython script to generate all junction flows
	scriptPath := GetPythonScriptPath("Jython_Scripts/extract_all_dss_data.py")
	log.Printf("Executing Jython script: %s", scriptPath)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute) // Increased timeout for processing all junctions
	defer cancel()

	err := executeJythonScript(ctx, scriptPath)
	if err != nil {
		log.Printf("Error executing Jython script for all junction flow data: %v", err)
		return err
	}

	log.Printf("Successfully executed Jython script for all junction flows")
	return nil
}

// handleGetAllJunctionFlows serves the output.json file
func handleGetAllJunctionFlows(c echo.Context) error {
	// Read the JSON file
	jsonPath := GetJSONOutputPath("output.json")
	jsonData, err := os.ReadFile(jsonPath)
	if err != nil {
		log.Printf("Error reading JSON file: %v", err)
		return respondWithError(c, http.StatusInternalServerError, "Failed to read junction flow data")
	}

	// Return the JSON data directly
	return c.JSONBlob(http.StatusOK, jsonData)
}
