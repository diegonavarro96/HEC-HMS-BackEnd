package main

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Database DatabaseConfig `mapstructure:"database"`
	Paths    PathsConfig    `mapstructure:"paths"`
	URLs     URLsConfig     `mapstructure:"urls"`
	Python   PythonConfig   `mapstructure:"python"`
	Jython   JythonConfig   `mapstructure:"jython"`
	HMS      HMSConfig      `mapstructure:"hms"`
	CORS     CORSConfig     `mapstructure:"cors"`
}

type ServerConfig struct {
	Port           string `mapstructure:"port"`
	TLSCertPath    string `mapstructure:"tls_cert_path"`
	TLSKeyPath     string `mapstructure:"tls_key_path"`
	Environment    string `mapstructure:"environment"`
	LogLevel       string `mapstructure:"log_level"`
	RateLimitBurst int    `mapstructure:"rate_limit_burst"`
}

type DatabaseConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	Name     string `mapstructure:"name"`
	SSLMode  string `mapstructure:"ssl_mode"`
}

type PathsConfig struct {
	LogDir                 string `mapstructure:"log_dir"`
	StaticCogDir           string `mapstructure:"static_cog_dir"`
	TestTifFile            string `mapstructure:"test_tif_file"`
	GribFilesDir           string `mapstructure:"grib_files_dir"`
	HMSModelsDir           string `mapstructure:"hms_models_dir"`
	HMSHistoricalModelsDir string `mapstructure:"hms_historical_models_dir"`
	PythonScriptsDir       string `mapstructure:"python_scripts_dir"`
	JSONOutputDir          string `mapstructure:"json_output_dir"`
	CSVDir                 string `mapstructure:"csv_dir"`
	DataDir                string `mapstructure:"data_dir"`
	DSSArchiveDir          string `mapstructure:"dss_archive_dir"`
	GrbDownloadsDir        string `mapstructure:"grb_downloads_dir"`
	HMSScriptsDir          string `mapstructure:"hms_scripts_dir"`
	ShapefilePath          string `mapstructure:"shapefile_path"`
}

type URLsConfig struct {
	MRMSDataSource      string `mapstructure:"mrms_data_source"`
	MRMSArchive         string `mapstructure:"mrms_archive"`
	MRMSPass1           string `mapstructure:"mrms_pass1"`
	HRRRDataSource      string `mapstructure:"hrrr_data_source"`
	ArcGISTokenEndpoint string `mapstructure:"arcgis_token_endpoint"`
	ArcGISSelfEndpoint  string `mapstructure:"arcgis_self_endpoint"`
}

type PythonConfig struct {
	HMSEnvPath      string `mapstructure:"hms_env_path"`
	Grib2CogEnvPath string `mapstructure:"grib2cog_env_path"`
}

type JythonConfig struct {
	ExecutablePath  string `mapstructure:"executable_path"`
	BatchScriptsDir string `mapstructure:"batch_scripts_dir"`
}

type HMSConfig struct {
	ExecutablePath        string          `mapstructure:"executable_path"`
	Version               string          `mapstructure:"version"`
	RealTimeControlFile   string          `mapstructure:"realtime_control_file"`
	HistoricalControlFile string          `mapstructure:"historical_control_file"`
	RealTimeScript        string          `mapstructure:"realtime_script"`
	HistoricalScript      string          `mapstructure:"historical_script"`
	LeonCreekModel        LeonCreekConfig `mapstructure:"leon_creek_model"`
}

type LeonCreekConfig struct {
	RainfallDir   string   `mapstructure:"rainfall_dir"`
	RealTimeDSS   string   `mapstructure:"realtime_dss"`
	HistoricalDSS string   `mapstructure:"historical_dss"`
	FilesToDelete []string `mapstructure:"files_to_delete"`
}

type CORSConfig struct {
	AllowedOrigins  []string `mapstructure:"allowed_origins"`
	AllowedIPRanges []string `mapstructure:"allowed_ip_ranges"`
}

var AppConfig Config

func LoadConfig(configPath string) error {
	viper.SetConfigType("yaml")

	if configPath != "" {
		viper.SetConfigFile(configPath)
	} else {
		viper.SetConfigName("config")
		viper.AddConfigPath(".")
		viper.AddConfigPath("./Go")
		viper.AddConfigPath("..")
	}

	// Set default values
	setDefaults()

	// Enable environment variable override
	viper.SetEnvPrefix("HMS")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	// Read config file
	if err := viper.ReadInConfig(); err != nil {
		return fmt.Errorf("error reading config file: %w", err)
	}

	// Unmarshal config
	if err := viper.Unmarshal(&AppConfig); err != nil {
		return fmt.Errorf("error unmarshaling config: %w", err)
	}

	// Process paths for OS compatibility
	processPathsForOS()

	// Convert relative paths to absolute paths
	if err := resolveRelativePaths(); err != nil {
		return fmt.Errorf("error resolving relative paths: %w", err)
	}

	return nil
}

func setDefaults() {
	// Server defaults
	viper.SetDefault("server.port", "8443")
	viper.SetDefault("server.environment", "development")
	viper.SetDefault("server.log_level", "info")
	viper.SetDefault("server.rate_limit_burst", 20)

	// Database defaults
	viper.SetDefault("database.host", "localhost")
	viper.SetDefault("database.port", 5432)
	viper.SetDefault("database.ssl_mode", "disable")

	// Path defaults (relative paths)
	viper.SetDefault("paths.log_dir", "logs")
	viper.SetDefault("paths.grib_files_dir", "gribFiles")
	viper.SetDefault("paths.json_output_dir", "../JSON")
	viper.SetDefault("paths.csv_dir", "../CSV")
}

func processPathsForOS() {
	// Convert Windows paths to proper format based on runtime OS
	if runtime.GOOS != "windows" {
		// Convert Windows paths to Unix paths
		AppConfig.Paths = convertPathsToUnix(AppConfig.Paths)
		AppConfig.Python.HMSEnvPath = filepath.ToSlash(AppConfig.Python.HMSEnvPath)
		AppConfig.Python.Grib2CogEnvPath = filepath.ToSlash(AppConfig.Python.Grib2CogEnvPath)
		AppConfig.Jython.ExecutablePath = filepath.ToSlash(AppConfig.Jython.ExecutablePath)
		AppConfig.HMS.ExecutablePath = filepath.ToSlash(AppConfig.HMS.ExecutablePath)
	}
}

func convertPathsToUnix(paths PathsConfig) PathsConfig {
	paths.LogDir = filepath.ToSlash(paths.LogDir)
	paths.StaticCogDir = filepath.ToSlash(paths.StaticCogDir)
	paths.TestTifFile = filepath.ToSlash(paths.TestTifFile)
	paths.GribFilesDir = filepath.ToSlash(paths.GribFilesDir)
	paths.HMSModelsDir = filepath.ToSlash(paths.HMSModelsDir)
	paths.PythonScriptsDir = filepath.ToSlash(paths.PythonScriptsDir)
	paths.JSONOutputDir = filepath.ToSlash(paths.JSONOutputDir)
	paths.CSVDir = filepath.ToSlash(paths.CSVDir)
	paths.DataDir = filepath.ToSlash(paths.DataDir)
	paths.DSSArchiveDir = filepath.ToSlash(paths.DSSArchiveDir)
	paths.GrbDownloadsDir = filepath.ToSlash(paths.GrbDownloadsDir)
	paths.HMSScriptsDir = filepath.ToSlash(paths.HMSScriptsDir)
	paths.ShapefilePath = filepath.ToSlash(paths.ShapefilePath)
	return paths
}

func resolveRelativePaths() error {
	// Convert all relative paths to absolute paths
	var err error
	
	// Convert PathsConfig fields
	if AppConfig.Paths.LogDir, err = filepath.Abs(AppConfig.Paths.LogDir); err != nil {
		return fmt.Errorf("failed to resolve LogDir: %w", err)
	}
	if AppConfig.Paths.StaticCogDir, err = filepath.Abs(AppConfig.Paths.StaticCogDir); err != nil {
		return fmt.Errorf("failed to resolve StaticCogDir: %w", err)
	}
	if AppConfig.Paths.TestTifFile, err = filepath.Abs(AppConfig.Paths.TestTifFile); err != nil {
		return fmt.Errorf("failed to resolve TestTifFile: %w", err)
	}
	if AppConfig.Paths.GribFilesDir, err = filepath.Abs(AppConfig.Paths.GribFilesDir); err != nil {
		return fmt.Errorf("failed to resolve GribFilesDir: %w", err)
	}
	if AppConfig.Paths.HMSModelsDir, err = filepath.Abs(AppConfig.Paths.HMSModelsDir); err != nil {
		return fmt.Errorf("failed to resolve HMSModelsDir: %w", err)
	}
	if AppConfig.Paths.HMSHistoricalModelsDir, err = filepath.Abs(AppConfig.Paths.HMSHistoricalModelsDir); err != nil {
		return fmt.Errorf("failed to resolve HMSHistoricalModelsDir: %w", err)
	}
	if AppConfig.Paths.PythonScriptsDir, err = filepath.Abs(AppConfig.Paths.PythonScriptsDir); err != nil {
		return fmt.Errorf("failed to resolve PythonScriptsDir: %w", err)
	}
	if AppConfig.Paths.JSONOutputDir, err = filepath.Abs(AppConfig.Paths.JSONOutputDir); err != nil {
		return fmt.Errorf("failed to resolve JSONOutputDir: %w", err)
	}
	if AppConfig.Paths.CSVDir, err = filepath.Abs(AppConfig.Paths.CSVDir); err != nil {
		return fmt.Errorf("failed to resolve CSVDir: %w", err)
	}
	if AppConfig.Paths.DataDir, err = filepath.Abs(AppConfig.Paths.DataDir); err != nil {
		return fmt.Errorf("failed to resolve DataDir: %w", err)
	}
	if AppConfig.Paths.DSSArchiveDir, err = filepath.Abs(AppConfig.Paths.DSSArchiveDir); err != nil {
		return fmt.Errorf("failed to resolve DSSArchiveDir: %w", err)
	}
	if AppConfig.Paths.GrbDownloadsDir, err = filepath.Abs(AppConfig.Paths.GrbDownloadsDir); err != nil {
		return fmt.Errorf("failed to resolve GrbDownloadsDir: %w", err)
	}
	if AppConfig.Paths.HMSScriptsDir, err = filepath.Abs(AppConfig.Paths.HMSScriptsDir); err != nil {
		return fmt.Errorf("failed to resolve HMSScriptsDir: %w", err)
	}
	if AppConfig.Paths.ShapefilePath, err = filepath.Abs(AppConfig.Paths.ShapefilePath); err != nil {
		return fmt.Errorf("failed to resolve ShapefilePath: %w", err)
	}

	// Convert JythonConfig fields
	if AppConfig.Jython.BatchScriptsDir, err = filepath.Abs(AppConfig.Jython.BatchScriptsDir); err != nil {
		return fmt.Errorf("failed to resolve BatchScriptsDir: %w", err)
	}

	// Convert HMSConfig fields
	if AppConfig.HMS.RealTimeControlFile, err = filepath.Abs(AppConfig.HMS.RealTimeControlFile); err != nil {
		return fmt.Errorf("failed to resolve RealTimeControlFile: %w", err)
	}
	if AppConfig.HMS.HistoricalControlFile, err = filepath.Abs(AppConfig.HMS.HistoricalControlFile); err != nil {
		return fmt.Errorf("failed to resolve HistoricalControlFile: %w", err)
	}

	// Convert LeonCreekConfig fields
	if AppConfig.HMS.LeonCreekModel.RainfallDir, err = filepath.Abs(AppConfig.HMS.LeonCreekModel.RainfallDir); err != nil {
		return fmt.Errorf("failed to resolve RainfallDir: %w", err)
	}
	if AppConfig.HMS.LeonCreekModel.RealTimeDSS, err = filepath.Abs(AppConfig.HMS.LeonCreekModel.RealTimeDSS); err != nil {
		return fmt.Errorf("failed to resolve RealTimeDSS: %w", err)
	}
	if AppConfig.HMS.LeonCreekModel.HistoricalDSS, err = filepath.Abs(AppConfig.HMS.LeonCreekModel.HistoricalDSS); err != nil {
		return fmt.Errorf("failed to resolve HistoricalDSS: %w", err)
	}

	// Convert FilesToDelete slice
	for i, filePath := range AppConfig.HMS.LeonCreekModel.FilesToDelete {
		if AppConfig.HMS.LeonCreekModel.FilesToDelete[i], err = filepath.Abs(filePath); err != nil {
			return fmt.Errorf("failed to resolve FilesToDelete[%d]: %w", i, err)
		}
	}

	return nil
}
