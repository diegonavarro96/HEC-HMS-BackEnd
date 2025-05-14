package main

import (
	"os"
	"strconv"
	"strings"
	"time"

	// "floodaceBackEnd.com/sqlcdb" // Removed
	"github.com/joho/godotenv"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"golang.org/x/time/rate"
)

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
	enc.AppendString("[36m" + t.Format("2006-01-02 15:04:05.000") + "[0m")
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
				statusColor = "[31m" // Red
			case status >= 400:
				statusColor = "[33m" // Yellow
			case status >= 300:
				statusColor = "[36m" // Cyan
			default:
				statusColor = "[32m" // Green
			}

			method := c.Request().Method
			var methodColor string
			switch method {
			case "GET":
				methodColor = "[32m" // Green
			case "POST":
				methodColor = "[33m" // Yellow
			case "PUT":
				methodColor = "[36m" // Cyan
			case "DELETE":
				methodColor = "[31m" // Red
			default:
				methodColor = "[37m" // White
			}

			// Use fmt.Sprintf to format the message with colors
			sugar.Infof("HTTP Request: method=%s%s%s, uri=%s%s%s, status=%s%s%s, latency=%s%s%s, ip=%s%s%s",
				methodColor, method, "[0m",
				"[35m", v.URI, "[0m",
				statusColor, strconv.Itoa(v.Status), "[0m",
				"[37m", v.Latency.String(), "[0m",
				"[37m", c.RealIP(), "[0m",
			)
			return nil
		},
	})
}

func main() {
	// Initialize logger
	logger, err := initLogger()
	if err != nil {
		// Fallback to basic logging if zap initialization fails
		// log.Fatalf("Failed to initialize logger: %v", err)
		panic("Failed to initialize logger: " + err.Error())
	}
	defer logger.Sync()

	sugar := logger.Sugar()

	// Load environment variables
	err = godotenv.Load()
	if err != nil {
		sugar.Warnw("⚠️  Warning: Failed to load .env file. Proceeding with environment variables or defaults.",
			"error", err,
		)
	}

	port := os.Getenv("SERVER_PORT")
	if port == "" {
		port = "8080" // Default port if not set
		sugar.Infow("SERVER_PORT not set in .env, using default.",
			"port", "[36m"+port+"[0m",
		)
	}
	env := os.Getenv("ENV")
	if env == "" {
		env = "development" // Default environment
		sugar.Infow("ENV not set in .env, using default.",
			"environment", "[35m"+env+"[0m",
		)
	}

	sugar.Infow("🚀 Server configuration loaded",
		"port", "[36m"+port+"[0m",
		"environment", "[35m"+env+"[0m",
	)

	e := echo.New()

	// Use custom request logger
	e.Use(CustomRequestLogger(sugar))
	e.Use(middleware.Recover())
	e.Use(middleware.RateLimiter(middleware.NewRateLimiterMemoryStore(
		rate.Limit(20), // Example: 20 requests per second
	)))

	// CORS configuration
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOriginFunc: func(origin string) (bool, error) {
			// Example: Allow all origins for development, be more specific for production
			if env == "development" {
				sugar.Debugw("CORS check in development, allowing origin", "origin", origin)
				return true, nil
			}

			allowedOrigins := map[string]bool{
				"https://localhost:8443":         true,
				"https://floodaceserver.ai:8443": true,
				"https://floodaceserver.ai:8444": true,
				"https://localhost:3000":         true,
				"https://floodaceserver.ai:8442": true,
				// Add your production frontend URLs here
			}

			if allowedOrigins[origin] {
				return true, nil
			}

			// Define the two allowed IP ranges
			// Consider if this IP range logic is still needed or if it should be more configurable
			ranges := []string{"http://192.168.1.", "http://192.168."} // Example, adjust as necessary
			for _, allowedOriginPrefix := range ranges {
				if strings.HasPrefix(origin, allowedOriginPrefix) {
					ipPart := strings.TrimPrefix(origin, allowedOriginPrefix)
					portIndex := strings.Index(ipPart, ":")

					if portIndex > 0 {
						ipPart = ipPart[:portIndex]
					}

					// Ensure ipPart is not empty and contains only digits before attempting conversion
					if ipPart != "" && strings.Trim(ipPart, "0123456789") == "" {
						ip, err := strconv.Atoi(ipPart)
						if err == nil && ip >= 1 && ip <= 254 {
							return true, nil
						}
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

	// --- Database connection ---
	dbConn, err := dbConnection()
	if err != nil {
		sugar.Fatalw("❌ Failed to connect to database",
			"error", err,
		)
	}
	defer dbConn.Close()
	sugar.Info("Successfully connected to database! ✅")

	e.GET("/health", func(c echo.Context) error {
		return c.String(200, "OK")
	})

	sugar.Infow("✨ Server starting",
		"port", "[36m"+port+"[0m",
		"tls_enabled", "[32m"+strconv.FormatBool(os.Getenv("ENABLE_TLS") == "true")+"[0m",
	)

	// Start server with configurable TLS
	enableTLS := os.Getenv("ENABLE_TLS") == "true"
	serverCrt := os.Getenv("SERVER_CRT_PATH")
	serverKey := os.Getenv("SERVER_KEY_PATH")

	if enableTLS {
		if serverCrt == "" {
			serverCrt = "./server.crt" // Default cert path
			sugar.Warnw("SERVER_CRT_PATH not set, using default", "path", serverCrt)
		}
		if serverKey == "" {
			serverKey = "./server.key" // Default key path
			sugar.Warnw("SERVER_KEY_PATH not set, using default", "path", serverKey)
		}
		// Check if cert and key files exist
		if _, err := os.Stat(serverCrt); os.IsNotExist(err) {
			sugar.Fatalw("💥 Server certificate file not found. Please set SERVER_CRT_PATH or place server.crt in the root.", "path", serverCrt, "error", err)
		}
		if _, err := os.Stat(serverKey); os.IsNotExist(err) {
			sugar.Fatalw("💥 Server key file not found. Please set SERVER_KEY_PATH or place server.key in the root.", "path", serverKey, "error", err)
		}

		if err := e.StartTLS(":"+port, serverCrt, serverKey); err != nil {
			sugar.Fatalw("💥 Server failed to start with TLS",
				"error", "[31m"+err.Error()+"[0m",
			)
		}
	} else {
		if err := e.Start(":"+port); err != nil {
			sugar.Fatalw("💥 Server failed to start without TLS",
				"error", "[31m"+err.Error()+"[0m",
			)
		}
	}
}
