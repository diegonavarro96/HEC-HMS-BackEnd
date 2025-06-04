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
PROJECT_DIR="$SCRIPT_DIR"  # Use current directory instead of ~/hms-backend
LOG_FILE="$SCRIPT_DIR/setup_log_$(date +%Y%m%d_%H%M%S).log"

# Parse command line arguments
SKIP_STEPS=""
INTERACTIVE_MODE=false
SHOW_HELP=false
AUTO_DETECTED_SKIPS=""

while [[ $# -gt 0 ]]; do
    case $1 in
        --skip)
            shift
            SKIP_STEPS="$1"
            ;;
        --interactive|-i)
            INTERACTIVE_MODE=true
            ;;
        --help|-h)
            SHOW_HELP=true
            ;;
        *)
            echo "Unknown option: $1"
            SHOW_HELP=true
            ;;
    esac
    shift
done

# Help function
show_help() {
    echo "Usage: $0 [OPTIONS]"
    echo ""
    echo "HEC-HMS Backend Automated Setup Script"
    echo ""
    echo "This script automatically detects completed installation steps and skips them."
    echo ""
    echo "OPTIONS:"
    echo "  --skip STEPS        Skip specified steps (comma-separated list)"
    echo "                      Example: --skip 1,3,5"
    echo "  -i, --interactive   Interactive mode - select which steps to run"
    echo "  -h, --help          Show this help message"
    echo ""
    echo "STEPS:"
    echo "  1  - Install system dependencies"
    echo "  2  - Install Go"
    echo "  3  - Configure Java"
    echo "  4  - Setup PostgreSQL"
    echo "  5  - Install Miniconda"
    echo "  6  - Create Conda environments"
    echo "  7  - Install Jython"
    echo "  8  - Install HEC-HMS"
    echo "  9  - Setup project structure"
    echo "  10 - Download HMS models from Google Drive"
    echo "  11 - Fix script line endings"
    echo "  12 - Create configuration files"
    echo "  13 - Setup database schema"
    echo "  14 - Build Go backend"
    echo "  15 - Set file permissions"
    echo "  16 - Setup environment variables"
    echo "  17 - Create systemd service"
    echo ""
    echo "Example:"
    echo "  $0 --skip 2,5      # Skip Go and Miniconda installation"
    echo "  $0 --interactive   # Choose steps interactively"
    exit 0
}

if $SHOW_HELP; then
    show_help
fi

# Convert skip steps to array
IFS=',' read -ra SKIP_ARRAY <<< "$SKIP_STEPS"

# Function to check if step should be skipped
should_skip_step() {
    local step_num=$1
    for skip in "${SKIP_ARRAY[@]}"; do
        if [[ "$skip" == "$step_num" ]]; then
            return 0
        fi
    done
    return 1
}

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

# Function to extract Google Drive ID from URL or return the ID if already in correct format
extract_google_drive_id() {
    local input="$1"
    
    # If empty, return empty
    if [[ -z "$input" ]]; then
        echo ""
        return
    fi
    
    # Check if it's already just an ID (no slashes or URL components)
    if [[ ! "$input" =~ [/:] ]]; then
        echo "$input"
        return
    fi
    
    # Extract ID from various Google Drive URL formats
    # Format 1: https://drive.google.com/file/d/FILE_ID/view
    # Format 2: https://drive.google.com/open?id=FILE_ID
    # Format 3: https://drive.google.com/drive/folders/FILE_ID
    
    local id=""
    
    # Try to extract from /d/FILE_ID/ format
    if [[ "$input" =~ /d/([a-zA-Z0-9_-]+) ]]; then
        id="${BASH_REMATCH[1]}"
    # Try to extract from ?id=FILE_ID format
    elif [[ "$input" =~ \?id=([a-zA-Z0-9_-]+) ]]; then
        id="${BASH_REMATCH[1]}"
    # Try to extract from &id=FILE_ID format
    elif [[ "$input" =~ \&id=([a-zA-Z0-9_-]+) ]]; then
        id="${BASH_REMATCH[1]}"
    # Try to extract from /folders/FILE_ID format
    elif [[ "$input" =~ /folders/([a-zA-Z0-9_-]+) ]]; then
        id="${BASH_REMATCH[1]}"
    # If no pattern matches, assume the input might be the ID itself
    else
        id="$input"
    fi
    
    echo "$id"
}

# Auto-detection functions for each step
check_step_1_completed() {
    # Check if key system packages are installed
    local packages=("curl" "wget" "git" "postgresql" "gdal-bin" "dos2unix")
    for pkg in "${packages[@]}"; do
        if ! command_exists "$pkg" && ! dpkg -l | grep -q "^ii  $pkg"; then
            return 1
        fi
    done
    return 0
}

check_step_2_completed() {
    # Check if Go 1.21 is installed
    if command_exists go && go version | grep -q "go1.21"; then
        return 0
    fi
    return 1
}

check_step_3_completed() {
    # Check if Java 8 is set as default
    if java -version 2>&1 | grep -q "1.8.0"; then
        return 0
    fi
    return 1
}

check_step_4_completed() {
    # Check if PostgreSQL database and user exist
    if sudo -u postgres psql -lqt 2>/dev/null | cut -d \| -f 1 | grep -qw "hms_backend"; then
        if sudo -u postgres psql -c "\du" 2>/dev/null | grep -q "hms_user"; then
            return 0
        fi
    fi
    return 1
}

check_step_5_completed() {
    # Check if Miniconda is installed
    if [ -d "$HOME_DIR/miniconda3" ] && [ -f "$HOME_DIR/miniconda3/bin/conda" ]; then
        return 0
    fi
    return 1
}

check_step_6_completed() {
    # Check if conda environments exist
    if [ -f "$HOME_DIR/miniconda3/bin/conda" ]; then
        eval "$($HOME_DIR/miniconda3/bin/conda shell.bash hook)"
        if conda env list | grep -q "hechmsfloodace" && conda env list | grep -q "grib2cog"; then
            return 0
        fi
    fi
    return 1
}

check_step_7_completed() {
    # Check if Jython is installed
    if [ -f "/opt/jython.jar" ]; then
        return 0
    fi
    return 1
}

check_step_8_completed() {
    # Check if HEC-HMS is installed
    if [ -d "/opt/hms" ] && [ -f "/opt/hms/hec-hms.sh" ]; then
        return 0
    fi
    return 1
}

check_step_9_completed() {
    # Check if project structure exists with key folders
    if [ -d "$PROJECT_DIR" ] && 
       [ -d "$PROJECT_DIR/hms_models" ] && 
       [ -d "$PROJECT_DIR/Go" ] && 
       [ -d "$PROJECT_DIR/python_scripts" ]; then
        return 0
    fi
    return 1
}

check_step_10_completed() {
    # Check if HMS models are downloaded
    if [ -f "$PROJECT_DIR/hms_models/RealTime/LeonCreek/LeonCreek.hms" ] && 
       [ -f "$PROJECT_DIR/hms_models/Historical/LeonCreek/LeonCreek.hms" ]; then
        return 0
    fi
    return 1
}

check_step_11_completed() {
    # Step 11 (line endings) should always run - it's quick
    return 1
}

check_step_12_completed() {
    # Check if config files exist - both .env AND config.yaml must exist
    # If config.yaml is missing, we need to create it from config.example.yaml
    if [ -f "$PROJECT_DIR/Go/.env" ] && [ -f "$PROJECT_DIR/Go/config.yaml" ]; then
        return 0
    fi
    return 1
}

check_step_13_completed() {
    # Check if database tables exist
    if [ -n "$DB_PASSWORD" ] && [ "$DB_PASSWORD" != "skipped" ]; then
        if PGPASSWORD="$DB_PASSWORD" psql -U hms_user -h localhost -d hms_backend -c "\dt" 2>/dev/null | grep -q "users"; then
            return 0
        fi
    fi
    return 1
}

check_step_14_completed() {
    # Check if Go binary exists
    if [ -f "$PROJECT_DIR/Go/hms-backend" ]; then
        return 0
    fi
    return 1
}

check_step_15_completed() {
    # Step 15 (permissions) should always run - it's quick
    return 1
}

check_step_16_completed() {
    # Check if environment variables are in bashrc
    if grep -q "HMS_HOME=/opt/hms" ~/.bashrc && 
       grep -q "JYTHON_JAR=/opt/jython.jar" ~/.bashrc; then
        return 0
    fi
    return 1
}

check_step_17_completed() {
    # Check if systemd service file exists
    if [ -f "/etc/systemd/system/hms-backend.service" ] || [ -f "$PROJECT_DIR/hms-backend.service" ]; then
        return 0
    fi
    return 1
}

# Function to auto-detect completed steps
auto_detect_completed_steps() {
    log "Auto-detecting completed steps..."
    local detected_skips=()
    
    for i in {1..17}; do
        if check_step_${i}_completed; then
            detected_skips+=("$i")
        fi
    done
    
    if [ ${#detected_skips[@]} -gt 0 ]; then
        AUTO_DETECTED_SKIPS=$(IFS=','; echo "${detected_skips[*]}")
        info "Auto-detected completed steps: $AUTO_DETECTED_SKIPS"
        
        # Show which steps are already completed
        echo ""
        echo "The following steps appear to be already completed:"
        for step in "${detected_skips[@]}"; do
            case $step in
                1) echo "  Step 1 - System dependencies (key packages installed)" ;;
                2) echo "  Step 2 - Go 1.21 (already installed)" ;;
                3) echo "  Step 3 - Java 8 configuration (already default)" ;;
                4) echo "  Step 4 - PostgreSQL (database and user exist)" ;;
                5) echo "  Step 5 - Miniconda (already installed)" ;;
                6) echo "  Step 6 - Conda environments (already created)" ;;
                7) echo "  Step 7 - Jython (already installed)" ;;
                8) echo "  Step 8 - HEC-HMS (already installed)" ;;
                9) echo "  Step 9 - Project structure (already exists)" ;;
                10) echo "  Step 10 - HMS models (already downloaded)" ;;
                12) echo "  Step 12 - Config files (already exist)" ;;
                13) echo "  Step 13 - Database schema (tables exist)" ;;
                14) echo "  Step 14 - Go backend (binary exists)" ;;
                16) echo "  Step 16 - Environment variables (already set)" ;;
                17) echo "  Step 17 - Systemd service (file exists)" ;;
            esac
        done
        echo ""
    else
        info "No completed steps detected - this appears to be a fresh installation."
    fi
}

# Interactive mode - select steps
if $INTERACTIVE_MODE; then
    # Run auto-detection first
    DB_PASSWORD="hms_secure_password_2024"  # Use default for detection
    auto_detect_completed_steps
    
    echo "================================================================="
    echo "   Interactive Mode - Select Steps to Run"
    echo "================================================================="
    echo ""
    echo "Available steps:"
    echo "  1  - Install system dependencies"
    echo "  2  - Install Go"
    echo "  3  - Configure Java"
    echo "  4  - Setup PostgreSQL"
    echo "  5  - Install Miniconda"
    echo "  6  - Create Conda environments"
    echo "  7  - Install Jython"
    echo "  8  - Install HEC-HMS"
    echo "  9  - Setup project structure"
    echo "  10 - Download HMS models from Google Drive"
    echo "  11 - Fix script line endings"
    echo "  12 - Create configuration files"
    echo "  13 - Setup database schema"
    echo "  14 - Build Go backend"
    echo "  15 - Set file permissions"
    echo "  16 - Setup environment variables"
    echo "  17 - Create systemd service"
    echo ""
    if [[ -n "$AUTO_DETECTED_SKIPS" ]]; then
        echo "Auto-detected completed steps: $AUTO_DETECTED_SKIPS"
        echo ""
    fi
    echo "Enter the steps you want to SKIP (comma-separated, or press Enter to run all):"
    echo "(Auto-detected steps will be automatically included)"
    read -p "Skip steps: " SKIP_INPUT
    
    # Merge with auto-detected skips
    if [[ -n "$SKIP_INPUT" ]]; then
        if [[ -n "$AUTO_DETECTED_SKIPS" ]]; then
            SKIP_STEPS="$SKIP_INPUT,$AUTO_DETECTED_SKIPS"
        else
            SKIP_STEPS="$SKIP_INPUT"
        fi
    else
        SKIP_STEPS="$AUTO_DETECTED_SKIPS"
    fi
    
    # Remove duplicates and sort
    if [[ -n "$SKIP_STEPS" ]]; then
        SKIP_STEPS=$(echo "$SKIP_STEPS" | tr ',' '\n' | sort -u | tr '\n' ',' | sed 's/,$//')
    fi
    IFS=',' read -ra SKIP_ARRAY <<< "$SKIP_STEPS"
fi

# Start setup
clear

# Auto-detect completed steps before showing banner
DB_PASSWORD="hms_secure_password_2024"  # Use default for detection
auto_detect_completed_steps

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

if [[ -n "$SKIP_STEPS" ]]; then
    log "Skipping steps: $SKIP_STEPS"
fi

# Prompt for critical information
echo ""
info "Setup Options:"
echo ""

# Merge auto-detected skips with command line skips
if [[ -n "$AUTO_DETECTED_SKIPS" ]]; then
    if [[ -n "$SKIP_STEPS" ]]; then
        # Merge and remove duplicates
        SKIP_STEPS="$SKIP_STEPS,$AUTO_DETECTED_SKIPS"
        SKIP_STEPS=$(echo "$SKIP_STEPS" | tr ',' '\n' | sort -u | tr '\n' ',' | sed 's/,$//')
    else
        SKIP_STEPS="$AUTO_DETECTED_SKIPS"
    fi
    IFS=',' read -ra SKIP_ARRAY <<< "$SKIP_STEPS"
fi

# Ask if user wants to skip any steps
if [[ -n "$SKIP_STEPS" ]]; then
    echo "Steps to be skipped (from command line and auto-detection): $SKIP_STEPS"
fi
read -p "Do you want to skip any additional steps? (y/n): " -n 1 -r
echo ""
if [[ $REPLY =~ ^[Yy]$ ]]; then
    echo ""
    echo "Available steps to skip:"
    echo "  1  - Install system dependencies"
    echo "  2  - Install Go"
    echo "  3  - Configure Java"
    echo "  4  - Setup PostgreSQL"
    echo "  5  - Install Miniconda"
    echo "  6  - Create Conda environments"
    echo "  7  - Install Jython"
    echo "  8  - Install HEC-HMS"
    echo "  9  - Setup project structure"
    echo "  10 - Download HMS models from Google Drive"
    echo "  11 - Fix script line endings"
    echo "  12 - Create configuration files"
    echo "  13 - Setup database schema"
    echo "  14 - Build Go backend"
    echo "  15 - Set file permissions"
    echo "  16 - Setup environment variables"
    echo "  17 - Create systemd service"
    echo ""
    read -p "Enter steps to skip (comma-separated, e.g., 1,5,10): " SKIP_INPUT
    if [[ -n "$SKIP_INPUT" ]]; then
        # Merge with existing skip steps if any
        if [[ -n "$SKIP_STEPS" ]]; then
            SKIP_STEPS="$SKIP_STEPS,$SKIP_INPUT"
        else
            SKIP_STEPS="$SKIP_INPUT"
        fi
        # Remove duplicates and sort
        SKIP_STEPS=$(echo "$SKIP_STEPS" | tr ',' '\n' | sort -u | tr '\n' ',' | sed 's/,$//')
        IFS=',' read -ra SKIP_ARRAY <<< "$SKIP_STEPS"
        log "Will skip steps: $SKIP_STEPS"
    fi
fi

echo ""
info "Please provide the following information:"
echo ""

# Only ask for PostgreSQL password if not skipping step 4
if ! should_skip_step 4; then
    DB_PASSWORD=$(prompt_with_default "PostgreSQL password for hms_user" "hms_secure_password_2024")
else
    DB_PASSWORD="skipped"
fi

# Only ask for Google Drive IDs if not skipping step 10
if ! should_skip_step 10; then
    info "You can paste either the Google Drive file ID or the full URL"
    temp_id=$(prompt_with_default "Google Drive file ID or URL for RealTimeZip.zip" "")
    GOOGLE_DRIVE_REALTIME_ID=$(extract_google_drive_id "$temp_id")
    
    temp_id=$(prompt_with_default "Google Drive file ID or URL for HistoricalZip.zip" "")
    GOOGLE_DRIVE_HISTORICAL_ID=$(extract_google_drive_id "$temp_id")
    
    temp_id=$(prompt_with_default "Google Drive file ID or URL for Bexar_County_shapefile.zip" "")
    GOOGLE_DRIVE_SHAPEFILE_ID=$(extract_google_drive_id "$temp_id")
else
    GOOGLE_DRIVE_REALTIME_ID=""
    GOOGLE_DRIVE_HISTORICAL_ID=""
    GOOGLE_DRIVE_SHAPEFILE_ID=""
fi

# Only ask for HMS tar path if not skipping step 8
if ! should_skip_step 8; then
    HMS_TAR_PATH=$(prompt_with_default "Path to HEC-HMS Linux tar file (or 'download' to download)" "download")
else
    HMS_TAR_PATH=""
fi

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
if should_skip_step 1; then
    log "Step 1: SKIPPED - Installing system dependencies"
else
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
fi

# Step 2: Install Go
if should_skip_step 2; then
    log "Step 2: SKIPPED - Installing Go 1.21.5"
else
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
fi

# Step 3: Configure Java
if should_skip_step 3; then
    log "Step 3: SKIPPED - Configuring Java alternatives"
else
    log "Step 3: Configuring Java alternatives"
    
    # Set Java 8 as default
    sudo update-alternatives --set java /usr/lib/jvm/java-8-openjdk-amd64/jre/bin/java || warning "Could not set Java 8 as default"
fi

# Step 4: PostgreSQL Setup
if should_skip_step 4; then
    log "Step 4: SKIPPED - Setting up PostgreSQL"
else
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
fi

# Step 5: Install Miniconda
if should_skip_step 5; then
    log "Step 5: SKIPPED - Installing Miniconda"
else
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
fi

# Step 6: Create Conda Environments
if should_skip_step 6; then
    log "Step 6: SKIPPED - Creating Conda environments"
else
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
fi

# Step 7: Install Jython
if should_skip_step 7; then
    log "Step 7: SKIPPED - Installing Jython"
else
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
fi

# Step 8: Install HEC-HMS
if should_skip_step 8; then
    log "Step 8: SKIPPED - Installing HEC-HMS"
else
    log "Step 8: Installing HEC-HMS"

if [ ! -d "/opt/hms" ]; then
    if [ "$HMS_TAR_PATH" = "download" ]; then
        # Download HEC-HMS 4.12
        log "Downloading HEC-HMS 4.12..."
        wget https://github.com/HydrologicEngineeringCenter/hec-downloads/releases/download/1.0.32/HEC-HMS-4.12-linux64.tar.gz -O /tmp/hec-hms.tar.gz || error "Failed to download HEC-HMS"
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
fi

# Step 9: Setup Project Structure
if should_skip_step 9; then
    log "Step 9: SKIPPED - Setting up project structure"
else
    log "Step 9: Setting up project structure"

# Create required directories in current location
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

cd "$PROJECT_DIR"
fi

# Step 10: Download HMS Models from Google Drive
if should_skip_step 10; then
    log "Step 10: SKIPPED - Downloading HMS models from Google Drive"
else
    log "Step 10: Downloading HMS models from Google Drive"

# Install gdown
pip install --user gdown

if [ -n "$GOOGLE_DRIVE_REALTIME_ID" ] && [ -n "$GOOGLE_DRIVE_HISTORICAL_ID" ]; then
    # Download RealTime model
    if [ ! -d "hms_models/RealTime/LeonCreek/LeonCreek.hms" ]; then
        log "Downloading RealTime HMS model..."
        ~/.local/bin/gdown --id "$GOOGLE_DRIVE_REALTIME_ID" -O RealTimeZip.zip || warning "Failed to download RealTime model"
        
        if [ -f "RealTimeZip.zip" ]; then
            log "Extracting RealTime HMS model..."
            mkdir -p temp_realtime
            unzip -q RealTimeZip.zip -d temp_realtime/
            cd temp_realtime
            
            # Find and extract any ZIP files in the extracted content
            for zipfile in *.zip; do
                if [ -f "$zipfile" ]; then
                    log "Extracting $zipfile..."
                    unzip -q "$zipfile"
                fi
            done
            
            # Copy LeonCreek folder to the correct location
            if [ -d "LeonCreek" ]; then
                mkdir -p ../hms_models/RealTime/
                cp -r LeonCreek ../hms_models/RealTime/
                log "RealTime model extracted successfully"
            else
                warning "LeonCreek folder not found in RealTime archive"
            fi
            
            cd ..
            rm -rf temp_realtime RealTimeZip.zip
        fi
    fi
    
    # Download Historical model
    if [ ! -d "hms_models/Historical/LeonCreek/LeonCreek.hms" ]; then
        log "Downloading Historical HMS model..."
        ~/.local/bin/gdown --id "$GOOGLE_DRIVE_HISTORICAL_ID" -O HistoricalZip.zip || warning "Failed to download Historical model"
        
        if [ -f "HistoricalZip.zip" ]; then
            log "Extracting Historical HMS model..."
            mkdir -p temp_historical
            unzip -q HistoricalZip.zip -d temp_historical/
            cd temp_historical
            
            # Find and extract any ZIP files in the extracted content
            for zipfile in *.zip; do
                if [ -f "$zipfile" ]; then
                    log "Extracting $zipfile..."
                    unzip -q "$zipfile"
                fi
            done
            
            # Copy LeonCreek folder to the correct location
            if [ -d "LeonCreek" ]; then
                mkdir -p ../hms_models/Historical/
                cp -r LeonCreek ../hms_models/Historical/
                log "Historical model extracted successfully"
            else
                warning "LeonCreek folder not found in Historical archive"
            fi
            
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
            mkdir -p gis_data/shapefiles/
            unzip -q Bexar_County_shapefile.zip -d gis_data/shapefiles/
            rm Bexar_County_shapefile.zip
        fi
    fi
else
        warning "Shapefile Google Drive ID not provided. Please manually download shapefile."
    fi
fi

# Step 11: Fix Script Line Endings
if should_skip_step 11; then
    log "Step 11: SKIPPED - Fixing script line endings"
else
    log "Step 11: Fixing script line endings"
    
    find . -name "*.sh" -type f -exec dos2unix {} + 2>/dev/null || true
    find . -name "*.sh" -type f -exec chmod +x {} + || true
fi

# Step 12: Create Configuration Files
if should_skip_step 12; then
    log "Step 12: SKIPPED - Creating configuration files"
else
    log "Step 12: Creating configuration files"

# Create .env file for Go
cat > "$PROJECT_DIR/Go/.env" << EOF
DB_HOST=localhost
DB_PORT=5432
DB_USER=hms_user
DB_PASSWORD=$DB_PASSWORD
DB_NAME=hms_backend
EOF

# Create or update Go config.yaml with correct paths
if [ -f "$PROJECT_DIR/Go/config.example.yaml" ]; then
    log "Creating config.yaml from config.example.yaml with Linux paths..."
    
    # Copy the example config and update the paths for Linux
    cp "$PROJECT_DIR/Go/config.example.yaml" "$PROJECT_DIR/Go/config.yaml"
    
    # Update Windows paths to Linux paths using sed
    sed -i "s|hms_models_dir: \".*\"|hms_models_dir: \"$PROJECT_DIR/hms_models\"|g" "$PROJECT_DIR/Go/config.yaml"
    sed -i "s|grib_files_dir: \".*\"|grib_files_dir: \"../gribFiles\"|g" "$PROJECT_DIR/Go/config.yaml"
    sed -i "s|dss_archive_dir: \".*\"|dss_archive_dir: \"$PROJECT_DIR/hms_models/RealTime/LeonCreek/dssArchive\"|g" "$PROJECT_DIR/Go/config.yaml"
    sed -i "s|grb_downloads_dir: \".*\"|grb_downloads_dir: \"$PROJECT_DIR/grb_downloads\"|g" "$PROJECT_DIR/Go/config.yaml"
    sed -i "s|shapefile_path: \".*\"|shapefile_path: \"$PROJECT_DIR/gis_data/shapefiles/Bexar_County.shp\"|g" "$PROJECT_DIR/Go/config.yaml"
    sed -i "s|hms_historical_models_dir: \".*\"|hms_historical_models_dir: \"$PROJECT_DIR/hms_models/Historical\"|g" "$PROJECT_DIR/Go/config.yaml"
    
    # Update Python paths - handle both ${USER} variable and actual username
    sed -i "s|hms_env_path: \".*\"|hms_env_path: \"$HOME_DIR/miniconda3/envs/hechmsfloodace/bin/python\"|g" "$PROJECT_DIR/Go/config.yaml"
    sed -i "s|grib2cog_env_path: \".*\"|grib2cog_env_path: \"$HOME_DIR/miniconda3/envs/grib2cog/bin/python\"|g" "$PROJECT_DIR/Go/config.yaml"
    
    # Update Jython paths
    sed -i "s|executable_path: \".*jython.*\"|executable_path: \"java -jar /opt/jython.jar\"|g" "$PROJECT_DIR/Go/config.yaml"
    sed -i "s|batch_scripts_dir: \".*\"|batch_scripts_dir: \"$PROJECT_DIR/python_scripts/Jython_Scripts/batchScripts\"|g" "$PROJECT_DIR/Go/config.yaml"
    
    # Update HMS paths - handle both HEC-HMS and hec-hms patterns
    sed -i "s|executable_path: \".*HEC-HMS.*\"|executable_path: \"/opt/hms/hec-hms.sh\"|g" "$PROJECT_DIR/Go/config.yaml"
    sed -i "s|executable_path: \".*hec-hms.*\"|executable_path: \"/opt/hms/hec-hms.sh\"|g" "$PROJECT_DIR/Go/config.yaml"
    sed -i "s|realtime_control_file: \".*\"|realtime_control_file: \"$PROJECT_DIR/hms_models/RealTime/LeonCreek/RainRealTime.control\"|g" "$PROJECT_DIR/Go/config.yaml"
    sed -i "s|historical_control_file: \".*\"|historical_control_file: \"$PROJECT_DIR/hms_models/Historical/LeonCreek/RainHistorical.control\"|g" "$PROJECT_DIR/Go/config.yaml"
    
    # Update Leon Creek model paths
    sed -i "s|rainfall_dir: \".*\"|rainfall_dir: \"$PROJECT_DIR/hms_models/RealTime/LeonCreek/Rainfall\"|g" "$PROJECT_DIR/Go/config.yaml"
    sed -i "s|realtime_dss: \".*\"|realtime_dss: \"$PROJECT_DIR/hms_models/RealTime/LeonCreek/RainrealTime.dss\"|g" "$PROJECT_DIR/Go/config.yaml"
    sed -i "s|historical_dss: \".*\"|historical_dss: \"$PROJECT_DIR/hms_models/Historical/LeonCreek/RainHistorical.dss\"|g" "$PROJECT_DIR/Go/config.yaml"
    
    # Update files_to_delete paths - handle both Windows and WSL/mounted paths
    sed -i "s|\"D:/.*LeonCreek/Rainfall/|\"$PROJECT_DIR/hms_models/RealTime/LeonCreek/Rainfall/|g" "$PROJECT_DIR/Go/config.yaml"
    sed -i "s|\"/mnt/.*/LeonCreek/Rainfall/|\"$PROJECT_DIR/hms_models/RealTime/LeonCreek/Rainfall/|g" "$PROJECT_DIR/Go/config.yaml"
    
    log "config.yaml created successfully with Linux paths"
elif [ -f "$PROJECT_DIR/Go/config.yaml" ]; then
    log "config.yaml already exists, skipping creation"
else
    warning "Neither config.yaml nor config.example.yaml found in Go directory"
    fi
fi

# Step 13: Setup Database
if should_skip_step 13; then
    log "Step 13: SKIPPED - Setting up database schema"
else
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
fi

# Step 14: Build Go Backend
if should_skip_step 14; then
    log "Step 14: SKIPPED - Building Go backend"
else
    log "Step 14: Building Go backend"
    
    go mod download || error "Failed to download Go dependencies"
    go build -o hms-backend . || error "Failed to build Go backend"
fi

# Step 15: Set Permissions
if should_skip_step 15; then
    log "Step 15: SKIPPED - Setting file permissions"
else
    log "Step 15: Setting file permissions"

cd "$PROJECT_DIR"
find . -type d -exec chmod 755 {} +
find . -type f -exec chmod 644 {} +
find . -name "*.sh" -type f -exec chmod +x {} +
chmod +x Go/hms-backend
fi

# Step 16: Environment Variables
if should_skip_step 16; then
    log "Step 16: SKIPPED - Setting up environment variables"
else
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
fi

# Step 17: Create systemd service (optional)
if should_skip_step 17; then
    log "Step 17: SKIPPED - Creating systemd service file"
else
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
fi

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

if [[ -n "$AUTO_DETECTED_SKIPS" ]]; then
    echo "  Note: The following steps were automatically skipped as they were already completed:"
    echo "        $AUTO_DETECTED_SKIPS"
    echo ""
fi

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