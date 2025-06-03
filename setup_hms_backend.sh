#!/bin/bash

# HEC-HMS Backend Automated Setup Script for Ubuntu Linux
# This script automates the deployment of HEC-HMS Backend on a fresh Ubuntu system
# Designed to run on Ubuntu 20.04+ via SSH without GUI

set -e  # Exit on error
set -u  # Exit on undefined variable

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Script variables
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
HOME_DIR="$HOME"
PROJECT_NAME="hms-backend"
PROJECT_DIR="$HOME_DIR/$PROJECT_NAME"
LOG_FILE="$SCRIPT_DIR/setup_log_$(date +%Y%m%d_%H%M%S).log"

# Function to log messages
log() {
    echo -e "${GREEN}[$(date +'%Y-%m-%d %H:%M:%S')]${NC} $1" | tee -a "$LOG_FILE"
}

error() {
    echo -e "${RED}[ERROR]${NC} $1" | tee -a "$LOG_FILE"
    exit 1
}

warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1" | tee -a "$LOG_FILE"
}

info() {
    echo -e "${BLUE}[INFO]${NC} $1" | tee -a "$LOG_FILE"
}

# Function to check if running as root
check_not_root() {
    if [ "$EUID" -eq 0 ]; then
        error "Please do not run this script as root. Use sudo where needed."
    fi
}

# Function to prompt for user input with default value
prompt_with_default() {
    local prompt="$1"
    local default="$2"
    local response
    
    read -p "$prompt [$default]: " response
    echo "${response:-$default}"
}

# Function to check command existence
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# Start setup
clear
echo "================================================================="
echo "   HEC-HMS Backend Automated Setup Script for Ubuntu Linux"
echo "================================================================="
echo ""

# Check not running as root
check_not_root

# Create log file
touch "$LOG_FILE"
log "Starting HEC-HMS Backend setup"
log "Log file: $LOG_FILE"

# Prompt for critical information
echo ""
info "Please provide the following information:"
echo ""

DB_PASSWORD=$(prompt_with_default "PostgreSQL password for hms_user" "hms_secure_password_2024")
GOOGLE_DRIVE_REALTIME_ID=$(prompt_with_default "Google Drive file ID for RealTimeZip.zip" "")
GOOGLE_DRIVE_HISTORICAL_ID=$(prompt_with_default "Google Drive file ID for HistoricalZip.zip" "")
GOOGLE_DRIVE_SHAPEFILE_ID=$(prompt_with_default "Google Drive file ID for Bexar_County_shapefile.zip" "")
HMS_TAR_PATH=$(prompt_with_default "Path to HEC-HMS Linux tar file (or 'download' to download)" "download")

echo ""
info "Setup will proceed with the following configuration:"
echo "  Project Directory: $PROJECT_DIR"
echo "  Database Password: [hidden]"
echo "  Log File: $LOG_FILE"
echo ""
read -p "Continue? (y/n): " -n 1 -r
echo ""
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    error "Setup cancelled by user"
fi

# Step 1: System Dependencies
log "Step 1: Installing system dependencies"

log "Updating system packages..."
sudo apt update && sudo apt upgrade -y || error "Failed to update system packages"

log "Installing basic dependencies..."
sudo apt install -y \
    build-essential \
    curl \
    wget \
    git \
    postgresql \
    postgresql-contrib \
    openjdk-8-jdk \
    openjdk-11-jdk \
    openjdk-17-jdk \
    gdal-bin \
    libgdal-dev \
    python3-gdal \
    dos2unix \
    unzip \
    tree \
    openssl || error "Failed to install system dependencies"

# Step 2: Install Go
log "Step 2: Installing Go 1.21.5"

if ! command_exists go || ! go version | grep -q "go1.21"; then
    wget https://go.dev/dl/go1.21.5.linux-amd64.tar.gz -O /tmp/go1.21.5.linux-amd64.tar.gz
    sudo rm -rf /usr/local/go
    sudo tar -C /usr/local -xzf /tmp/go1.21.5.linux-amd64.tar.gz
    
    # Add Go to PATH if not already there
    if ! grep -q "/usr/local/go/bin" ~/.bashrc; then
        echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
    fi
    export PATH=$PATH:/usr/local/go/bin
    
    rm /tmp/go1.21.5.linux-amd64.tar.gz
    log "Go installed successfully"
else
    log "Go is already installed"
fi

# Step 3: Configure Java
log "Step 3: Configuring Java alternatives"

# Set Java 8 as default
sudo update-alternatives --set java /usr/lib/jvm/java-8-openjdk-amd64/jre/bin/java || warning "Could not set Java 8 as default"

# Step 4: PostgreSQL Setup
log "Step 4: Setting up PostgreSQL"

# Ensure PostgreSQL is running
sudo service postgresql start || sudo systemctl start postgresql

# Wait for PostgreSQL to be ready
sleep 5

# Create database and user
log "Creating PostgreSQL database and user..."
sudo -u postgres psql << EOF || warning "Database may already exist"
CREATE USER hms_user WITH PASSWORD '$DB_PASSWORD';
CREATE DATABASE hms_backend OWNER hms_user;
GRANT ALL PRIVILEGES ON DATABASE hms_backend TO hms_user;
\q
EOF

# Step 5: Install Miniconda
log "Step 5: Installing Miniconda"

if [ ! -d "$HOME_DIR/miniconda3" ]; then
    wget https://repo.anaconda.com/miniconda/Miniconda3-latest-Linux-x86_64.sh -O /tmp/miniconda.sh
    bash /tmp/miniconda.sh -b -p "$HOME_DIR/miniconda3"
    rm /tmp/miniconda.sh
    
    # Initialize conda
    eval "$($HOME_DIR/miniconda3/bin/conda shell.bash hook)"
    
    # Add conda to bashrc if not already there
    if ! grep -q "miniconda3/bin/conda" ~/.bashrc; then
        echo 'eval "$($HOME/miniconda3/bin/conda shell.bash hook)"' >> ~/.bashrc
    fi
    
    log "Miniconda installed successfully"
else
    log "Miniconda already installed"
    eval "$($HOME_DIR/miniconda3/bin/conda shell.bash hook)"
fi

# Step 6: Create Conda Environments
log "Step 6: Creating Conda environments"

# Create hechmsfloodace environment
if ! conda env list | grep -q "hechmsfloodace"; then
    log "Creating hechmsfloodace environment..."
    conda create -n hechmsfloodace python=3.10 -y
    conda run -n hechmsfloodace conda install -c conda-forge requests beautifulsoup4 watchdog pyyaml flask flask-cors -y
else
    log "hechmsfloodace environment already exists"
fi

# Create grib2cog environment
if ! conda env list | grep -q "grib2cog"; then
    log "Creating grib2cog environment..."
    conda create -n grib2cog python=3.10 -y
    conda run -n grib2cog conda install -c conda-forge xarray rioxarray cfgrib eccodes -y
else
    log "grib2cog environment already exists"
fi

# Step 7: Install Jython
log "Step 7: Installing Jython"

if [ ! -f "/opt/jython.jar" ]; then
    wget https://repo1.maven.org/maven2/org/python/jython-standalone/2.7.3/jython-standalone-2.7.3.jar -O /tmp/jython.jar
    sudo mkdir -p /opt
    sudo mv /tmp/jython.jar /opt/jython.jar
    sudo chmod 755 /opt/jython.jar
    log "Jython installed successfully"
else
    log "Jython already installed"
fi

# Test Jython
java -jar /opt/jython.jar --version || warning "Jython test failed"

# Step 8: Install HEC-HMS
log "Step 8: Installing HEC-HMS"

if [ ! -d "/opt/hms" ]; then
    if [ "$HMS_TAR_PATH" = "download" ]; then
        # Download HEC-HMS 4.12
        log "Downloading HEC-HMS 4.12..."
        wget https://github.com/HydrologicEngineeringCenter/hec-downloads/releases/download/1.0.27/hec-hms-4.12-linux-x64.tar.gz -O /tmp/hec-hms.tar.gz || error "Failed to download HEC-HMS"
        HMS_TAR_PATH="/tmp/hec-hms.tar.gz"
    fi
    
    if [ -f "$HMS_TAR_PATH" ]; then
        sudo mkdir -p /opt/hms
        sudo tar -xf "$HMS_TAR_PATH" -C /opt/hms --strip-components=1
        sudo chmod -R 755 /opt/hms
        log "HEC-HMS installed successfully"
    else
        error "HEC-HMS tar file not found at: $HMS_TAR_PATH"
    fi
else
    log "HEC-HMS already installed"
fi

# Test HEC-HMS
/opt/hms/hec-hms.sh -help >/dev/null 2>&1 || warning "HEC-HMS test failed"

# Step 9: Setup Project Structure
log "Step 9: Setting up project structure"

# Create project directory
mkdir -p "$PROJECT_DIR"

# Copy current project files to new location
log "Copying project files..."
rsync -av --exclude='*.log' --exclude='setup_hms_backend.sh' "$SCRIPT_DIR/" "$PROJECT_DIR/"

cd "$PROJECT_DIR"

# Create required directories
log "Creating required directories..."
mkdir -p hms_models/LeonCreek/{Rainfall,dssArchive}
mkdir -p hms_models/RealTime/LeonCreek/{Rainfall,dssArchive}
mkdir -p hms_models/Historical/LeonCreek
mkdir -p grb_downloads
mkdir -p data/cogs_output
mkdir -p gis_data/shapefiles
mkdir -p dss_files/incoming
mkdir -p logs
mkdir -p CSV
mkdir -p JSON
mkdir -p gribFiles/{historical,realtime}
mkdir -p temp

# Step 10: Download HMS Models from Google Drive
log "Step 10: Downloading HMS models from Google Drive"

# Install gdown
pip install --user gdown

if [ -n "$GOOGLE_DRIVE_REALTIME_ID" ] && [ -n "$GOOGLE_DRIVE_HISTORICAL_ID" ]; then
    # Download RealTime model
    if [ ! -d "hms_models/RealTime/LeonCreek/LeonCreek.hms" ]; then
        log "Downloading RealTime HMS model..."
        ~/.local/bin/gdown --id "$GOOGLE_DRIVE_REALTIME_ID" -O RealTimeZip.zip || warning "Failed to download RealTime model"
        
        if [ -f "RealTimeZip.zip" ]; then
            unzip -q RealTimeZip.zip -d temp_realtime/
            cd temp_realtime
            unzip -q LeonCreek.zip
            cp -r LeonCreek ../hms_models/RealTime/
            cd ..
            rm -rf temp_realtime RealTimeZip.zip
        fi
    fi
    
    # Download Historical model
    if [ ! -d "hms_models/Historical/LeonCreek/LeonCreek.hms" ]; then
        log "Downloading Historical HMS model..."
        ~/.local/bin/gdown --id "$GOOGLE_DRIVE_HISTORICAL_ID" -O HistoricalZip.zip || warning "Failed to download Historical model"
        
        if [ -f "HistoricalZip.zip" ]; then
            unzip -q HistoricalZip.zip -d temp_historical/
            cd temp_historical
            unzip -q LeonCreek.zip
            cp -r LeonCreek ../hms_models/Historical/
            cd ..
            rm -rf temp_historical HistoricalZip.zip
        fi
    fi
else
    warning "Google Drive IDs not provided. Please manually download HMS models."
fi

# Download shapefile
if [ -n "$GOOGLE_DRIVE_SHAPEFILE_ID" ]; then
    if [ ! -f "gis_data/shapefiles/Bexar_County.shp" ]; then
        log "Downloading shapefile..."
        ~/.local/bin/gdown --id "$GOOGLE_DRIVE_SHAPEFILE_ID" -O Bexar_County_shapefile.zip || warning "Failed to download shapefile"
        
        if [ -f "Bexar_County_shapefile.zip" ]; then
            unzip -q Bexar_County_shapefile.zip -d gis_data/shapefiles/
            rm Bexar_County_shapefile.zip
        fi
    fi
else
    warning "Shapefile Google Drive ID not provided. Please manually download shapefile."
fi

# Step 11: Fix Script Line Endings
log "Step 11: Fixing script line endings"

find . -name "*.sh" -type f -exec dos2unix {} + 2>/dev/null || true
find . -name "*.sh" -type f -exec chmod +x {} + || true

# Step 12: Create Configuration Files
log "Step 12: Creating configuration files"

# Create .env file for Go
cat > "$PROJECT_DIR/Go/.env" << EOF
DB_HOST=localhost
DB_PORT=5432
DB_USER=hms_user
DB_PASSWORD=$DB_PASSWORD
DB_NAME=hms_backend
EOF

# Update Go config.yaml with correct paths
if [ -f "$PROJECT_DIR/Go/config.yaml" ]; then
    log "Updating Go config.yaml..."
    # This is a placeholder - in production, you'd use sed or a proper YAML parser
    cat > "$PROJECT_DIR/Go/config_updated.yaml" << EOF
paths:
  hms_models_dir: "$PROJECT_DIR/hms_models"
  python_scripts_dir: "../python_scripts"
  grib_files_dir: "../gribFiles"
  shapefile_path: "$PROJECT_DIR/gis_data/shapefiles/Bexar_County.shp"
  hms_historical_models_dir: "$PROJECT_DIR/hms_models/Historical"

python:
  hms_env_path: "$HOME_DIR/miniconda3/envs/hechmsfloodace/bin/python"
  grib2cog_env_path: "$HOME_DIR/miniconda3/envs/grib2cog/bin/python"

jython:
  executable_path: "java -jar /opt/jython.jar"
  batch_scripts_dir: "$PROJECT_DIR/python_scripts/Jython_Scripts/batchScripts"

hms:
  executable_path: "/opt/hms/hec-hms.sh"
EOF
    
    # Merge with existing config (you'll need to implement proper YAML merging)
    warning "Please manually update $PROJECT_DIR/Go/config.yaml with the paths in config_updated.yaml"
fi

# Step 13: Setup Database
log "Step 13: Setting up database schema"

cd "$PROJECT_DIR/Go"

if [ -f "sql/schema.sql" ]; then
    PGPASSWORD="$DB_PASSWORD" psql -U hms_user -h localhost -d hms_backend -f sql/schema.sql || warning "Database schema may already exist"
fi

# Generate SSL certificates
if [ ! -f "server.crt" ]; then
    log "Generating SSL certificates..."
    openssl req -x509 -newkey rsa:4096 -keyout server.key -out server.crt -days 365 -nodes \
        -subj "/C=US/ST=State/L=City/O=Organization/OU=Unit/CN=localhost"
fi

# Step 14: Build Go Backend
log "Step 14: Building Go backend"

go mod download || error "Failed to download Go dependencies"
go build -o hms-backend . || error "Failed to build Go backend"

# Step 15: Set Permissions
log "Step 15: Setting file permissions"

cd "$PROJECT_DIR"
find . -type d -exec chmod 755 {} +
find . -type f -exec chmod 644 {} +
find . -name "*.sh" -type f -exec chmod +x {} +
chmod +x Go/hms-backend

# Step 16: Environment Variables
log "Step 16: Setting up environment variables"

# Add environment variables to bashrc if not already there
if ! grep -q "HMS_HOME" ~/.bashrc; then
    cat >> ~/.bashrc << EOF

# HEC-HMS Backend Environment Variables
export HMS_HOME=/opt/hms
export JYTHON_JAR=/opt/jython.jar
export PATH=\$PATH:/usr/local/go/bin
EOF
fi

# Step 17: Create systemd service (optional)
log "Step 17: Creating systemd service file"

cat > "$PROJECT_DIR/hms-backend.service" << EOF
[Unit]
Description=HEC-HMS Backend Service
After=network.target postgresql.service

[Service]
Type=simple
User=$USER
WorkingDirectory=$PROJECT_DIR/Go
ExecStart=$PROJECT_DIR/Go/hms-backend
Restart=on-failure
RestartSec=10
StandardOutput=append:$PROJECT_DIR/logs/hms-backend.log
StandardError=append:$PROJECT_DIR/logs/hms-backend-error.log

[Install]
WantedBy=multi-user.target
EOF

log "Systemd service file created at: $PROJECT_DIR/hms-backend.service"
log "To install as a service, run:"
log "  sudo cp $PROJECT_DIR/hms-backend.service /etc/systemd/system/"
log "  sudo systemctl daemon-reload"
log "  sudo systemctl enable hms-backend"
log "  sudo systemctl start hms-backend"

# Final Summary
echo ""
echo "================================================================="
echo "                    Setup Complete!"
echo "================================================================="
echo ""
info "Installation Summary:"
echo "  - Project Directory: $PROJECT_DIR"
echo "  - Go Backend Binary: $PROJECT_DIR/Go/hms-backend"
echo "  - Log File: $LOG_FILE"
echo ""
info "Next Steps:"
echo "  1. Review and update configuration files:"
echo "     - $PROJECT_DIR/Go/config.yaml"
echo "     - HMS model paths in control files"
echo ""
echo "  2. Start the backend:"
echo "     cd $PROJECT_DIR/Go"
echo "     ./hms-backend"
echo ""
echo "  3. Test the pipelines:"
echo "     # Historical Pipeline:"
echo "     curl -k -X POST https://localhost:8443/api/run-hms-pipeline-historical \\"
echo "       -H \"Content-Type: application/json\" \\"
echo "       -d '{\"startDate\":\"2025-05-08\",\"endDate\":\"2025-05-08\",\"startTime\":\"00:00\",\"endTime\":\"23:00\"}'"
echo ""
echo "     # Real-Time Pipeline:"
echo "     curl -k -X POST https://localhost:8443/api/run-hms-pipeline \\"
echo "       -H \"Content-Type: application/json\""
echo ""

if [ -n "$GOOGLE_DRIVE_REALTIME_ID" ] || [ -n "$GOOGLE_DRIVE_HISTORICAL_ID" ] || [ -n "$GOOGLE_DRIVE_SHAPEFILE_ID" ]; then
    warning "Manual Steps Required:"
    [ -z "$GOOGLE_DRIVE_REALTIME_ID" ] && echo "  - Download RealTime HMS model"
    [ -z "$GOOGLE_DRIVE_HISTORICAL_ID" ] && echo "  - Download Historical HMS model"
    [ -z "$GOOGLE_DRIVE_SHAPEFILE_ID" ] && echo "  - Download Bexar County shapefile"
    echo "  - Update HMS model internal paths"
    echo "  - Update config.yaml with correct paths"
fi

echo ""
log "Setup completed successfully!"