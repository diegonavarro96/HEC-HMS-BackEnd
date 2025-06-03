package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
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
		OutputPaths:      []string{"stdout", filepath.Join(AppConfig.Paths.LogDir, "server.log")},
		ErrorOutputPaths: []string{"stderr"},
		Encoding:         "console", // Use console encoding for better readability
		EncoderConfig:    encoderConfig,
	}

	// Create logs directory if it doesn't exist
	if _, err := os.Stat(AppConfig.Paths.LogDir); os.IsNotExist(err) {
		os.MkdirAll(AppConfig.Paths.LogDir, 0755)
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
	//flag.StringVar(&mrmsDataSourceURL, "url", "https://mtarchive.geol.iastate.edu/2025/05/05/mrms/ncep/MultiSensor_QPE_24H_Pass2/", "URL for the MRMS QPE data source. Used by the /api/precip/latest endpoint.")
	flag.StringVar(&mrmsDataSourceURL, "url", "https://mrms.ncep.noaa.gov/2D/RadarOnly_QPE_24H/", "URL for the MRMS QPE data source. Used by the /api/precip/latest endpoint.")
	flag.Parse()

	// Load configuration
	if err := LoadConfig(""); err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize logger
	logger, err := initLogger()
	if err != nil {
		panic("Failed to initialize logger: " + err.Error())
	}
	defer logger.Sync()

	sugar := logger.Sugar()

	// Update mrmsDataSourceURL if not provided via flag
	if mrmsDataSourceURL == AppConfig.URLs.MRMSDataSource {
		mrmsDataSourceURL = AppConfig.URLs.MRMSDataSource
	}

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

	// Use config for port, fallback to env var if set
	port := AppConfig.Server.Port
	if envPort := os.Getenv("SERVER_PORT"); envPort != "" {
		port = envPort
	}

	sugar.Infow("🚀 Server configuration loaded",
		"port", "\x1b[36m"+port+"\x1b[0m",
		"environment", "\x1b[35m"+AppConfig.Server.Environment+"\x1b[0m",
	)

	e := echo.New()

	// Use custom request logger
	e.Use(CustomRequestLogger(sugar))
	e.Use(middleware.Recover())
	e.Use(middleware.RateLimiter(middleware.NewRateLimiterMemoryStore(
		rate.Limit(AppConfig.Server.RateLimitBurst),
	)))

	// CORS configuration
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOriginFunc: func(origin string) (bool, error) {
			allowedOrigins := make(map[string]bool)
			for _, origin := range AppConfig.CORS.AllowedOrigins {
				allowedOrigins[origin] = true
			}

			if allowedOrigins[origin] {
				return true, nil
			}

			// Use configured IP ranges
			ranges := AppConfig.CORS.AllowedIPRanges
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

	log.Printf("Serving static COG files from local directory: %s under URL prefix /cogs", AppConfig.Paths.StaticCogDir)
	e.Static("/cogs", AppConfig.Paths.StaticCogDir)
	// Serve the specific test TIF file at /cogs_test
	log.Printf("Serving static file %s at URL prefix /cogs_test", AppConfig.Paths.TestTifFile)
	e.File("/cogs_test", AppConfig.Paths.TestTifFile)

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

	e.GET("/api/get-all-junction-flows", handleGetAllJunctionFlows)

	e.GET("/api/precip/latest", handelGetLatestPrecip)

	//Historical API Calls
	e.POST("/api/run-hms-pipeline-historical", handleRunHMSPipelineHistorical)
	e.POST("/api/extract-historical-dss-data", handleExtractHistoricalDSSData)
	
	// SMS API endpoint
	e.POST("/api/send-sms", handleSendSMS)

	sugar.Infow("✨ Server starting",
		"port", "\x1b[36m"+port+"\x1b[0m",
		"tls", "\x1b[32mtrue\x1b[0m",
	)

	// Start the scheduler
	StartScheduler() // This will run the archive and pipeline trigger task at HH:15

	// Start server with TLS
	if err := e.StartTLS(":"+port, AppConfig.Server.TLSCertPath, AppConfig.Server.TLSKeyPath); err != nil {
		sugar.Fatalw("💥 Server failed to start",
			"error", "\x1b[31m"+err.Error()+"\x1b[0m",
		)
	}
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
