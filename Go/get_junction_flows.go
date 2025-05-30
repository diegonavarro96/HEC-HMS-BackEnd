package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
)

// handleGetJunctionFlow handles the request to get flow data for a junction
func handleGetJunctionFlow(c echo.Context) error {
	// Define a struct for the request body
	type JunctionRequest struct {
		BJunctionPart string `json:"b_part_junction"`
	}

	// Parse request body
	var req JunctionRequest
	if err := c.Bind(&req); err != nil {
		log.Printf("Error parsing junction flow request body: %v", err)
		return respondWithError(c, http.StatusBadRequest, "Invalid request format")
	}

	// Log the received parameter
	log.Printf("Received junction flow request for: %s", req.BJunctionPart)

	// Create context with timeout for the script execution
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Execute the Python script directly
	scriptPath := "D:/FloodaceDocuments/HMS/HMSBackend/python_scripts/RealTime/getDataFromDSSJython.py"
	log.Printf("python script path: %s", scriptPath)
	err := executePythonScript(ctx, scriptPath, req.BJunctionPart)
	if err != nil {
		log.Printf("Error executing Python script for junction flow data: %v", err)
		return respondWithError(c, http.StatusInternalServerError, "Failed to process junction flow data")
	}

	// Read the CSV file (assuming the Python script creates/updates this file)
	csvPath := "../CSV/output.csv"
	csvData, err := os.ReadFile(csvPath)
	if err != nil {
		log.Printf("Error reading CSV file after script execution: %v", err)
		return respondWithError(c, http.StatusInternalServerError, "Failed to read flow data results")
	}

	// Parse CSV data
	lines := strings.Split(string(csvData), "\n")
	var dataPoints []map[string]interface{}

	// Skip header row if it exists and process data rows
	startRow := 0
	if len(lines) > 0 && (strings.Contains(lines[0], "time") || strings.Contains(lines[0], "Time") ||
		strings.Contains(lines[0], "DATE") || strings.Contains(lines[0], "Date")) {
		startRow = 1
	}

	for i := startRow; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}

		parts := strings.Split(line, ",")
		if len(parts) >= 2 {
			timeStr := strings.TrimSpace(parts[0])
			valueStr := strings.TrimSpace(parts[1])

			// Use the time string directly from the CSV
			formattedTime := timeStr

			value, err := strconv.ParseFloat(valueStr, 64)
			if err != nil {
				log.Printf("Warning: Could not parse value '%s' as float", valueStr)
				continue
			}

			dataPoints = append(dataPoints, map[string]interface{}{
				"time":  formattedTime,
				"value": value,
			})
		}
	}

	// Create response with America/Monterrey timezone as specified
	response := map[string]interface{}{
		"series": []map[string]interface{}{
			{
				"name":     req.BJunctionPart,
				"unit":     "cfs",
				"timezone": "UTC", // As specified in requirements
				"data":     dataPoints,
			},
		},
	}

	return respondWithJSON(c, http.StatusOK, response)
}

// ProcessAllJunctionFlows executes the Jython script to generate all junction flow data
func ProcessAllJunctionFlows() error {
	// Execute the Jython script to generate all junction flows
	scriptPath := "D:/FloodaceDocuments/HMS/HMSGit/HEC-HMS-Floodace/scripts/jythonScripts/extract_all_dss_data.py"
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
	jsonPath := "D:/FloodaceDocuments/HMS/HMSBackend/JSON/output.json"
	jsonData, err := os.ReadFile(jsonPath)
	if err != nil {
		log.Printf("Error reading JSON file: %v", err)
		return respondWithError(c, http.StatusInternalServerError, "Failed to read junction flow data")
	}

	// Return the JSON data directly
	return c.JSONBlob(http.StatusOK, jsonData)
}
