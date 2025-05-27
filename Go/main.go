package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"HMSBackend/sqlcdb"

	"github.com/joho/godotenv"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"golang.org/x/time/rate"
)

// mrmsDataSourceURL will be set by a command-line flag.
// It's used by FetchLatestQPE in get_precip_accum.go.
var mrmsDataSourceURL string

// initLogger configures and creates a new zap logger
func initLogger() (*zap.Logger, error) {
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "timestamp",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		FunctionKey:    zapcore.OmitKey,
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.CapitalColorLevelEncoder, // Adds color to log levels
		EncodeTime:     CustomTimeEncoder,
		EncodeDuration: zapcore.StringDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	// Create custom configuration
	config := zap.Config{
		Development:      false,
		Level:            zap.NewAtomicLevelAt(zap.InfoLevel),
		OutputPaths:      []string{"stdout", "logs/server.log"},
		ErrorOutputPaths: []string{"stderr"},
		Encoding:         "console", // Use console encoding for better readability
		EncoderConfig:    encoderConfig,
	}

	// Create logs directory if it doesn't exist
	if _, err := os.Stat("logs"); os.IsNotExist(err) {
		os.Mkdir("logs", 0755)
	}

	return config.Build(zap.AddCaller())
}

// CustomTimeEncoder formats the time with colors and better formatting
func CustomTimeEncoder(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
	enc.AppendString("\x1b[36m" + t.Format("2006-01-02 15:04:05.000") + "\x1b[0m")
}

// CustomRequestLogger creates a custom request logger middleware
func CustomRequestLogger(sugar *zap.SugaredLogger) echo.MiddlewareFunc {
	return middleware.RequestLoggerWithConfig(middleware.RequestLoggerConfig{
		LogURI:     true,
		LogStatus:  true,
		LogLatency: true,
		LogValuesFunc: func(c echo.Context, v middleware.RequestLoggerValues) error {
			status := v.Status
			var statusColor string
			switch {
			case status >= 500:
				statusColor = "\x1b[31m" // Red
			case status >= 400:
				statusColor = "\x1b[33m" // Yellow
			case status >= 300:
				statusColor = "\x1b[36m" // Cyan
			default:
				statusColor = "\x1b[32m" // Green
			}

			method := c.Request().Method
			var methodColor string
			switch method {
			case "GET":
				methodColor = "\x1b[32m" // Green
			case "POST":
				methodColor = "\x1b[33m" // Yellow
			case "PUT":
				methodColor = "\x1b[36m" // Cyan
			case "DELETE":
				methodColor = "\x1b[31m" // Red
			default:
				methodColor = "\x1b[37m" // White
			}

			// Use fmt.Sprintf to format the message with colors
			sugar.Infof("HTTP Request: method=%s%s%s, uri=%s%s%s, status=%s%s%s, latency=%s%s%s, ip=%s%s%s",
				methodColor, method, "\x1b[0m",
				"\x1b[35m", v.URI, "\x1b[0m",
				statusColor, strconv.Itoa(v.Status), "\x1b[0m",
				"\x1b[37m", v.Latency.String(), "\x1b[0m",
				"\x1b[37m", c.RealIP(), "\x1b[0m",
			)
			return nil
		},
	})
}

func main() {
	// Define and parse command-line flags first
	// The default URL for the MRMS QPE data source.
	// This will populate the package-level mrmsDataSourceURL variable.
	flag.StringVar(&mrmsDataSourceURL, "url", "https://mtarchive.geol.iastate.edu/2025/05/05/mrms/ncep/MultiSensor_QPE_24H_Pass2/", "URL for the MRMS QPE data source. Used by the /api/precip/latest endpoint.")
	//flag.StringVar(&mrmsDataSourceURL, "url", "https://mrms.ncep.noaa.gov/2D/RadarOnly_QPE_24H/", "URL for the MRMS QPE data source. Used by the /api/precip/latest endpoint.")
	flag.Parse()

	// Initialize logger
	logger, err := initLogger()
	if err != nil {
		panic("Failed to initialize logger: " + err.Error())
	}
	defer logger.Sync()

	sugar := logger.Sugar()

	// Log the MRMS data source URL being used
	sugar.Infow("MRMS Data Source Configuration",
		"url", "\x1b[36m"+mrmsDataSourceURL+"\x1b[0m",
	)

	// Load environment variables
	err = godotenv.Load()
	if err != nil {
		sugar.Fatalw("❌ Failed to load .env file",
			"error", err,
		)
	}

	port := os.Getenv("SERVER_PORT")
	sugar.Infow("🚀 Server configuration loaded",
		"port", "\x1b[36m"+port+"\x1b[0m",
		"environment", "\x1b[35m"+os.Getenv("ENV")+"\x1b[0m",
	)

	e := echo.New()

	// Use custom request logger
	e.Use(CustomRequestLogger(sugar))
	e.Use(middleware.Recover())
	e.Use(middleware.RateLimiter(middleware.NewRateLimiterMemoryStore(
		rate.Limit(20),
	)))

	// CORS configuration
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOriginFunc: func(origin string) (bool, error) {
			allowedOrigins := map[string]bool{
				"https://localhost:8442":                true,
				"https://floodaceserver.ai:8443":        true,
				"https://floodaceserver.ai:8444":        true,
				"https://localhost:3000":                true,
				"https://floodaceserver.ai:8442":        true,
				"https://diegon.tail779ff5.ts.net:8442": true,
			}

			if allowedOrigins[origin] {
				return true, nil
			}

			// Define the two allowed IP ranges
			ranges := []string{"http://192.168.1.", "http://192.168."}
			for _, allowedOriginPrefix := range ranges {
				if strings.HasPrefix(origin, allowedOriginPrefix) {
					ipPart := strings.TrimPrefix(origin, allowedOriginPrefix)
					portIndex := strings.Index(ipPart, ":")

					if portIndex > 0 {
						ipPart = ipPart[:portIndex]
					}

					ip, err := strconv.Atoi(ipPart)
					if err == nil && ip >= 1 && ip <= 254 {
						return true, nil
					}
				}
			}

			sugar.Infow("Rejected CORS origin",
				"origin", origin,
			)
			return false, nil
		},
		AllowMethods:     []string{echo.GET, echo.POST, echo.PUT, echo.DELETE, echo.OPTIONS},
		AllowHeaders:     []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept, echo.HeaderAuthorization},
		AllowCredentials: true,
	}))

	staticCogDir := "../data/cogs_output" // Define it once
	log.Printf("Serving static COG files from local directory: %s under URL prefix /cogs", staticCogDir)
	e.Static("/cogs", staticCogDir)
	// Serve the specific test TIF file at /cogs_test
	// The file path is relative to the application's root directory.
	testTifFilePath := "cogs_output/reprojectv5.tif"
	log.Printf("Serving static file %s at URL prefix /cogs_test", testTifFilePath)
	e.File("/cogs_test", testTifFilePath)

	// Database connection
	dbConn, err := dbConnection()
	if err != nil {
		sugar.Fatalw("Failed to connect to database",
			"error", err,
		)
	}
	defer dbConn.Close()

	queries := sqlcdb.New(dbConn)
	sugar.Info("Database connection established successfully")

	// Health check endpoint
	e.GET("/health", func(c echo.Context) error {
		return c.String(200, "OK")
	})

	// User management endpoints
	e.POST("/api/validate/user", handleValidateUser(queries))
	e.GET("/api/auth/callback", handleCallback)
	e.GET("/api/session", handleUserSession)
	e.GET("/api/get/all_users", handleGetAllUsers(queries))
	e.POST("/api/modify/user", handleModifyUser(queries))

	// HMS processing pipeline endpoint
	e.POST("/api/run-hms-pipeline", handleRunHMSPipeline)

	// Junction flow data endpoint
	e.POST("/api/get-junction-flow", handleGetJunctionFlow)

	e.GET("/api/precip/latest", handelGetLatestPrecip)

	//Historical API Calls
	//e.POST("/api/run-hms-pipeline-historical", handleRunHMSPipelineHistorical)

	sugar.Infow("✨ Server starting",
		"port", "\x1b[36m"+port+"\x1b[0m",
		"tls", "\x1b[32mtrue\x1b[0m",
	)

	// Start the scheduler
	StartScheduler() // This will run the archive and pipeline trigger task at HH:15

	// Start server with TLS
	if err := e.StartTLS(":"+port, "./server.crt", "./server.key"); err != nil {
		sugar.Fatalw("💥 Server failed to start",
			"error", "\x1b[31m"+err.Error()+"\x1b[0m",
		)
	}
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

	// Get the URL from environment variable
	junctionFlowURL := os.Getenv("PYTHON_GET_DSS_JUNCTION_FLOW_URL")
	if junctionFlowURL == "" {
		log.Printf("Missing required environment variable: PYHTON_GET_DSS_JUNCTION_FLOW_URL")
		return respondWithError(c, http.StatusInternalServerError, "Server configuration error")
	}

	// Create HTTP client
	client := &http.Client{
		Timeout: 5 * time.Minute,
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Forward the request to the Python endpoint
	payload := map[string]string{"b_part_junction": req.BJunctionPart}
	err := MakePostRequest(ctx, client, junctionFlowURL, payload)
	if err != nil {
		log.Printf("Error getting junction flow data: %v", err)
		return respondWithError(c, http.StatusInternalServerError, "Failed to process junction flow data")
	}

	// Read the CSV file
	csvPath := "../CSV/output.csv"
	csvData, err := os.ReadFile(csvPath)
	if err != nil {
		log.Printf("Error reading CSV file: %v", err)
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

// parseTimeString attempts to parse a time string in various formats
func parseTimeString(timeStr string) (time.Time, error) {
	formats := []string{
		"2006-01-02T15:04:05", // ISO format without timezone
		"2006-01-02 15:04:05", // Common date time format
		"02 Jan 2006T15:04",   // DD Mon YYYYTHH:MM format
		"01/02/2006 15:04:05", // MM/DD/YYYY format
		"02/01/2006 15:04:05", // DD/MM/YYYY format
		"2006/01/02 15:04:05", // YYYY/MM/DD format
		"01-02-2006 15:04:05", // MM-DD-YYYY format
		"02-01-2006 15:04:05", // DD-MM-YYYY format
		"2006-01-02",          // YYYY-MM-DD date only
		"01/02/2006",          // MM/DD/YYYY date only
		"02/01/2006",          // DD/MM/YYYY date only
		"2006/01/02",          // YYYY/MM/DD date only
		"20060102150405",      // YYYYMMDDhhmmss
		"20060102",            // YYYYMMDD date only
		"01022006",            // MMDDYYYY date only
		"02012006",            // DDMMYYYY date only
		"15:04:05",            // Time only
		"15:04",               // Hours and minutes only
	}

	for _, format := range formats {
		t, err := time.Parse(format, timeStr)
		if err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("no matching time format found for: %s", timeStr)
}
