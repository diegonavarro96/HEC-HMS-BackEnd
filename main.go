package main

import (
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
	// Initialize logger
	logger, err := initLogger()
	if err != nil {
		panic("Failed to initialize logger: " + err.Error())
	}
	defer logger.Sync()

	sugar := logger.Sugar()

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
				"https://localhost:8443":         true,
				"https://floodaceserver.ai:8443": true,
				"https://floodaceserver.ai:8444": true,
				"https://localhost:3000":         true,
				"https://floodaceserver.ai:8442": true,
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

	sugar.Infow("✨ Server starting",
		"port", "\x1b[36m"+port+"\x1b[0m",
		"tls", "\x1b[32mtrue\x1b[0m",
	)

	// Start server with TLS
	if err := e.StartTLS(":"+port, "./server.crt", "./server.key"); err != nil {
		sugar.Fatalw("💥 Server failed to start",
			"error", "\x1b[31m"+err.Error()+"\x1b[0m",
		)
	}
}

