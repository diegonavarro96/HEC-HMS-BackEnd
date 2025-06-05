#!/bin/bash

# HEC-HMS Backend Automated Setup Script for Ubuntu Linux
# This script automates the deployment of HEC-HMS Backend on AWS EC2 Ubuntu instances
# Designed to run on Ubuntu 20.04+ for production deployment

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
CACHE_FILE="$SCRIPT_DIR/.setup_cache"

# AWS deployment variables
IS_AWS_DEPLOYMENT=false
AWS_INSTANCE_DOMAIN=""
LETSENCRYPT_EMAIL=""

# Parse command line arguments
SHOW_HELP=false
CLEAR_CACHE=false

while [[ $# -gt 0 ]]; do
    case $1 in
        --aws)
            IS_AWS_DEPLOYMENT=true
            ;;
        --domain)
            shift
            AWS_INSTANCE_DOMAIN="$1"
            ;;
        --email)
            shift
            LETSENCRYPT_EMAIL="$1"
            ;;
        --clear-cache)
            CLEAR_CACHE=true
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
    echo "HEC-HMS Backend Automated Setup Script for AWS Deployment"
    echo ""
    echo "This script will interactively ask whether to skip each step during installation."
    echo ""
    echo "OPTIONS:"
    echo "  --aws               Enable AWS deployment mode (configures for production)"
    echo "  --domain DOMAIN     Set domain for SSL certificate (e.g., hms.example.com)"
    echo "  --email EMAIL       Email for Let's Encrypt SSL certificate"
    echo "  --clear-cache       Clear cached values (Google Drive IDs, etc.)"
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
    echo "  8  - Install HEC-HMS from Google Drive"
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
    echo "AWS Deployment Example:"
    echo "  $0 --aws --domain hms.example.com --email admin@example.com"
    echo ""
    echo "Standard Deployment Example:"
    echo "  $0"
    exit 0
}

if $SHOW_HELP; then
    show_help
fi

# Handle clear cache option
if $CLEAR_CACHE; then
    if [ -f "$CACHE_FILE" ]; then
        rm -f "$CACHE_FILE"
        echo -e "${GREEN}Cache cleared successfully${NC}"
    else
        echo -e "${YELLOW}No cache file found${NC}"
    fi
    exit 0
fi

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

# Function to save value to cache
save_to_cache() {
    local key="$1"
    local value="$2"
    
    # Create cache file if it doesn't exist
    if [ ! -f "$CACHE_FILE" ]; then
        touch "$CACHE_FILE"
        chmod 600 "$CACHE_FILE"  # Only owner can read/write
    fi
    
    # Remove existing entry if present
    if [ -f "$CACHE_FILE" ]; then
        grep -v "^$key=" "$CACHE_FILE" > "$CACHE_FILE.tmp" || true
        mv "$CACHE_FILE.tmp" "$CACHE_FILE"
    fi
    
    # Add new entry
    echo "$key=$value" >> "$CACHE_FILE"
}

# Function to read value from cache
read_from_cache() {
    local key="$1"
    
    if [ -f "$CACHE_FILE" ]; then
        grep "^$key=" "$CACHE_FILE" 2>/dev/null | cut -d'=' -f2- || echo ""
    else
        echo ""
    fi
}

# Function to prompt with default value from cache
prompt_with_cache() {
    local prompt="$1"
    local cache_key="$2"
    local default="$3"
    local response
    
    # Try to get cached value
    local cached_value=$(read_from_cache "$cache_key")
    
    # Use cached value as default if available
    if [ -n "$cached_value" ]; then
        default="$cached_value"
        echo -e "${GREEN}(Using cached value)${NC}"
    fi
    
    read -p "$prompt [$default]: " response
    response="${response:-$default}"
    
    # Save to cache if not empty
    if [ -n "$response" ]; then
        save_to_cache "$cache_key" "$response"
    fi
    
    echo "$response"
}

# Function to ask if user wants to run a step
ask_to_run_step() {
    local step_name="$1"
    local step_description="$2"
    local auto_detected_complete="$3"
    
    echo ""
    echo "================================================================="
    echo " Step: $step_name"
    echo "================================================================="
    echo "Description: $step_description"
    
    if [ "$auto_detected_complete" = "true" ]; then
        echo -e "${GREEN}✓ This step appears to be already completed${NC}"
        read -p "Do you want to run it anyway? (y/N): " -n 1 -r
        echo ""
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            return 1  # Skip
        fi
    else
        read -p "Do you want to run this step? (Y/n): " -n 1 -r
        echo ""
        if [[ $REPLY =~ ^[Nn]$ ]]; then
            return 1  # Skip
        fi
    fi
    return 0  # Run
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

# Function to download file from Google Drive using wget (alternative method)
download_google_drive_wget() {
    local file_id="$1"
    local output_file="$2"
    
    log "Attempting download with wget method..."
    
    # First attempt - direct download
    wget --quiet --save-cookies /tmp/cookies.txt --keep-session-cookies --no-check-certificate \
        "https://docs.google.com/uc?export=download&id=$file_id" -O- | \
        sed -rn 's/.*confirm=([0-9A-Za-z_]+).*/\1\n/p' > /tmp/confirm.txt
    
    local confirm=$(cat /tmp/confirm.txt)
    
    if [ -n "$confirm" ]; then
        # Large file - needs confirmation
        wget --load-cookies /tmp/cookies.txt \
            "https://docs.google.com/uc?export=download&confirm=$confirm&id=$file_id" \
            -O "$output_file" || return 1
    else
        # Small file - direct download
        wget "https://docs.google.com/uc?export=download&id=$file_id" -O "$output_file" || return 1
    fi
    
    rm -f /tmp/cookies.txt /tmp/confirm.txt
    return 0
}

# Function to download file from Google Drive with progress
download_from_google_drive() {
    local file_id="$1"
    local output_file="$2"
    
    log "Downloading from Google Drive (ID: $file_id)..."
    
    # First try with gdown
    if ! command_exists gdown; then
        log "Installing gdown..."
        pip install --user gdown >/dev/null 2>&1
        # Add local bin to PATH for this session
        export PATH="$HOME/.local/bin:$PATH"
    fi
    
    # Try gdown first
    if command_exists gdown || [ -f "$HOME/.local/bin/gdown" ]; then
        log "Attempting download with gdown..."
        if gdown "$file_id" -O "$output_file" --fuzzy 2>/dev/null || \
           "$HOME/.local/bin/gdown" "$file_id" -O "$output_file" --fuzzy 2>/dev/null; then
            # Verify it's a valid ZIP
            if [ -f "$output_file" ] && file "$output_file" | grep -q -E "Zip archive|ZIP archive|Java archive"; then
                log "Download successful with gdown"
                return 0
            else
                warning "gdown download failed or file is not a valid archive"
                rm -f "$output_file"
            fi
        fi
    fi
    
    # If gdown failed, try wget method
    log "Trying alternative download method..."
    if download_google_drive_wget "$file_id" "$output_file"; then
        # Verify the downloaded file
        if [ -f "$output_file" ]; then
            if file "$output_file" | grep -q -E "Zip archive|ZIP archive|Java archive"; then
                log "Download successful with wget method"
                return 0
            else
                # Check if it's an HTML error page
                if head -n 1 "$output_file" | grep -q "<!DOCTYPE html"; then
                    error "Downloaded file is an HTML error page, not the expected archive"
                    error "This usually means the file ID is incorrect or the file requires manual download"
                    error "Please download the file manually and place it in the correct location"
                else
                    error "Downloaded file is not a valid archive"
                fi
                rm -f "$output_file"
                return 1
            fi
        fi
    fi
    
    error "Failed to download file from Google Drive"
    error "Please verify the file ID is correct or download manually"
    return 1
}

# Start setup
clear

echo "================================================================="
echo "   HEC-HMS Backend Automated Setup Script"
echo "================================================================="
echo ""

if $IS_AWS_DEPLOYMENT; then
    echo -e "${BLUE}AWS Deployment Mode Enabled${NC}"
    
    # If domain/email not provided via command line, prompt with cache
    if [ -z "$AWS_INSTANCE_DOMAIN" ]; then
        AWS_INSTANCE_DOMAIN=$(prompt_with_cache "AWS instance domain (e.g., hms.example.com)" "aws_domain" "")
    fi
    
    if [ -z "$LETSENCRYPT_EMAIL" ] && [ -n "$AWS_INSTANCE_DOMAIN" ]; then
        LETSENCRYPT_EMAIL=$(prompt_with_cache "Email for Let's Encrypt SSL certificate" "letsencrypt_email" "")
    fi
    
    echo "Domain: ${AWS_INSTANCE_DOMAIN:-Not specified}"
    echo "Email: ${LETSENCRYPT_EMAIL:-Not specified}"
    echo ""
fi

# Check not running as root
check_not_root

# Create log file
touch "$LOG_FILE"
log "Starting HEC-HMS Backend setup"
log "Log file: $LOG_FILE"

if $IS_AWS_DEPLOYMENT; then
    log "AWS deployment mode enabled"
fi

# Initial configuration gathering
echo ""
info "Initial Configuration"
echo "===================="
echo ""

# Initialize DB_PASSWORD variable (will be set later if needed)
DB_PASSWORD=""

echo ""
info "Google Drive Files Information"
echo "You can paste either the Google Drive file ID or the full URL"
echo "Previously used values will be suggested automatically"
echo ""

temp_id=$(prompt_with_cache "Google Drive file ID/URL for HEC-HMS ZIP file" "gdrive_hms_id" "")
HMS_GOOGLE_DRIVE_ID=$(extract_google_drive_id "$temp_id")

temp_id=$(prompt_with_cache "Google Drive file ID/URL for RealTime models ZIP" "gdrive_realtime_id" "")
GOOGLE_DRIVE_REALTIME_ID=$(extract_google_drive_id "$temp_id")

temp_id=$(prompt_with_cache "Google Drive file ID/URL for Historical models ZIP" "gdrive_historical_id" "")
GOOGLE_DRIVE_HISTORICAL_ID=$(extract_google_drive_id "$temp_id")

temp_id=$(prompt_with_cache "Google Drive file ID/URL for Bexar County shapefile ZIP" "gdrive_shapefile_id" "")
GOOGLE_DRIVE_SHAPEFILE_ID=$(extract_google_drive_id "$temp_id")

echo ""
info "Setup will proceed with the following configuration:"
echo "  Project Directory: $PROJECT_DIR"
echo "  Database Password: [hidden]"
if $IS_AWS_DEPLOYMENT; then
    echo "  AWS Domain: ${AWS_INSTANCE_DOMAIN:-Will use self-signed certificates}"
fi
echo "  Log File: $LOG_FILE"
echo ""
read -p "Continue? (y/n): " -n 1 -r
echo ""
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    error "Setup cancelled by user"
fi

# Step 1: System Dependencies
if ask_to_run_step "Step 1" "Install system dependencies (build tools, Java, PostgreSQL, GDAL, etc.)" "$(check_step_1_completed && echo true || echo false)"; then
    log "Installing system dependencies..."
    
    # Fix any interrupted dpkg operations
    if sudo dpkg --audit 2>/dev/null | grep -q .; then
        log "Fixing interrupted package installations..."
        sudo dpkg --configure -a || warning "Could not fix dpkg automatically"
    fi
    
    log "Updating system packages..."
    sudo apt update || error "Failed to update package list"
    
    # Clean up any partial installations
    sudo apt-get -f install -y || warning "Could not fix broken dependencies"
    
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
        openssl \
        python3-pip || error "Failed to install system dependencies"
    
    if $IS_AWS_DEPLOYMENT; then
        log "Installing AWS-specific dependencies..."
        sudo apt install -y certbot nginx || error "Failed to install AWS dependencies"
    fi
else
    log "Skipping Step 1: System dependencies"
fi

# Step 2: Install Go
if ask_to_run_step "Step 2" "Install Go 1.21.5" "$(check_step_2_completed && echo true || echo false)"; then
    log "Installing Go 1.21.5..."
    
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
    log "Skipping Step 2: Go installation"
fi

# Step 3: Configure Java
if ask_to_run_step "Step 3" "Configure Java 8 as default" "$(check_step_3_completed && echo true || echo false)"; then
    log "Configuring Java alternatives..."
    
    # Set Java 8 as default
    sudo update-alternatives --set java /usr/lib/jvm/java-8-openjdk-amd64/jre/bin/java || warning "Could not set Java 8 as default"
    
    # Verify Java version
    java -version
else
    log "Skipping Step 3: Java configuration"
fi

# Step 4: PostgreSQL Setup
if ask_to_run_step "Step 4" "Setup PostgreSQL database and user" "$(check_step_4_completed && echo true || echo false)"; then
    log "Setting up PostgreSQL..."
    
    # Ask for database password only when needed
    if [ -z "$DB_PASSWORD" ]; then
        DB_PASSWORD=$(prompt_with_default "PostgreSQL password for hms_user" "hms_secure_password_2024")
    fi
    
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

    log "PostgreSQL setup complete"
else
    log "Skipping Step 4: PostgreSQL setup"
    # Mark DB_PASSWORD as skipped if PostgreSQL setup is skipped
    if [ -z "$DB_PASSWORD" ]; then
        DB_PASSWORD="skipped"
    fi
fi

# Step 5: Install Miniconda
if ask_to_run_step "Step 5" "Install Miniconda for Python environments" "$(check_step_5_completed && echo true || echo false)"; then
    log "Installing Miniconda..."
    
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
    log "Skipping Step 5: Miniconda installation"
    if [ -d "$HOME_DIR/miniconda3" ]; then
        eval "$($HOME_DIR/miniconda3/bin/conda shell.bash hook)"
    fi
fi

# Step 6: Create Conda Environments
if ask_to_run_step "Step 6" "Create conda environments (hechmsfloodace, grib2cog)" "$(check_step_6_completed && echo true || echo false)"; then
    log "Creating Conda environments..."
    
    # Create hechmsfloodace environment
    log "Creating hechmsfloodace environment..."
    conda create -n hechmsfloodace python=3.10 -y
    conda run -n hechmsfloodace conda install -c conda-forge requests beautifulsoup4 watchdog pyyaml flask flask-cors -y
    
    # Create grib2cog environment
    log "Creating grib2cog environment..."
    conda create -n grib2cog python=3.10 -y
    
    # Update conda in the environment first
    conda run -n grib2cog conda update -n base conda -y
    
    # Install packages step by step to avoid dependency conflicts
    log "Installing xarray and rioxarray..."
    conda run -n grib2cog conda install -c conda-forge xarray rioxarray -y
    
    log "Installing GRIB processing packages..."
    # Try installing eccodes and cfgrib with specific versions that work together
    conda run -n grib2cog conda install -c conda-forge eccodes=2.31.0 -y || {
        warning "Failed to install eccodes 2.31.0, trying latest version..."
        conda run -n grib2cog conda install -c conda-forge eccodes -y
    }
    
    # Install cfgrib after eccodes is installed
    conda run -n grib2cog pip install cfgrib || {
        warning "Failed to install cfgrib with pip, trying conda..."
        conda run -n grib2cog conda install -c conda-forge cfgrib -y || warning "cfgrib installation failed"
    }
    
    # Install additional dependencies that might be needed
    conda run -n grib2cog conda install -c conda-forge numpy pandas -y
    
    log "Conda environments created successfully"
else
    log "Skipping Step 6: Conda environment creation"
fi

# Step 7: Install Jython
if ask_to_run_step "Step 7" "Install Jython for HEC-DSSVue integration" "$(check_step_7_completed && echo true || echo false)"; then
    log "Installing Jython..."
    
    wget https://repo1.maven.org/maven2/org/python/jython-standalone/2.7.3/jython-standalone-2.7.3.jar -O /tmp/jython.jar
    sudo mkdir -p /opt
    sudo mv /tmp/jython.jar /opt/jython.jar
    sudo chmod 755 /opt/jython.jar
    
    # Test Jython
    java -jar /opt/jython.jar --version || warning "Jython test failed"
    
    log "Jython installed successfully"
else
    log "Skipping Step 7: Jython installation"
fi

# Step 8: Install HEC-HMS
if ask_to_run_step "Step 8" "Install HEC-HMS from Google Drive" "$(check_step_8_completed && echo true || echo false)"; then
    log "Installing HEC-HMS..."
    
    if [ -n "$HMS_GOOGLE_DRIVE_ID" ]; then
        log "Downloading HEC-HMS from Google Drive..."
        download_from_google_drive "$HMS_GOOGLE_DRIVE_ID" "/tmp/hms.zip" || error "Failed to download HEC-HMS"
        
        log "Extracting HEC-HMS..."
        # First extract to temp location to check structure
        mkdir -p /tmp/hms_extract
        unzip -q /tmp/hms.zip -d /tmp/hms_extract/
        
        # Check the structure and move appropriately
        sudo mkdir -p /opt/hms
        
        # Case 1: If there's a single directory in the extract (e.g., HEC-HMS-4.12)
        if [ $(ls -1 /tmp/hms_extract | wc -l) -eq 1 ] && [ -d "/tmp/hms_extract/$(ls -1 /tmp/hms_extract)" ]; then
            # Move contents of that directory to /opt/hms
            sudo mv /tmp/hms_extract/*/* /opt/hms/ 2>/dev/null || true
        else
            # Case 2: Files are directly in the zip
            sudo mv /tmp/hms_extract/* /opt/hms/ 2>/dev/null || true
        fi
        
        # Clean up temp directory
        rm -rf /tmp/hms_extract
        
        # Ensure proper permissions
        sudo chmod -R 755 /opt/hms
        sudo chmod +x /opt/hms/hec-hms.sh
        
        rm /tmp/hms.zip
        
        # Test HEC-HMS
        /opt/hms/hec-hms.sh -help >/dev/null 2>&1 || warning "HEC-HMS test failed (this is normal for headless systems)"
        
        log "HEC-HMS installed successfully"
    else
        warning "HEC-HMS Google Drive ID not provided. Please manually install HEC-HMS to /opt/hms"
    fi
else
    log "Skipping Step 8: HEC-HMS installation"
fi

# Step 9: Setup Project Structure
if ask_to_run_step "Step 9" "Create project directory structure" "$(check_step_9_completed && echo true || echo false)"; then
    log "Setting up project structure..."
    
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
    log "Project structure created successfully"
else
    log "Skipping Step 9: Project structure setup"
fi

# Step 10: Download HMS Models from Google Drive
if ask_to_run_step "Step 10" "Download HMS models and shapefiles from Google Drive" "$(check_step_10_completed && echo true || echo false)"; then
    log "Downloading HMS models from Google Drive..."
    
    # Download RealTime model
    if [ -n "$GOOGLE_DRIVE_REALTIME_ID" ]; then
        log "Downloading RealTime HMS model..."
        download_from_google_drive "$GOOGLE_DRIVE_REALTIME_ID" "RealTimeZip.zip" || warning "Failed to download RealTime model"
        
        if [ -f "RealTimeZip.zip" ]; then
            log "Extracting RealTime HMS model..."
            mkdir -p temp_realtime
            unzip -q RealTimeZip.zip -d temp_realtime/
            cd temp_realtime
            
            # Find and extract any nested ZIP files
            for zipfile in *.zip; do
                if [ -f "$zipfile" ]; then
                    log "Extracting nested $zipfile..."
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
    else
        warning "RealTime model Google Drive ID not provided"
    fi
    
    # Download Historical model
    if [ -n "$GOOGLE_DRIVE_HISTORICAL_ID" ]; then
        log "Downloading Historical HMS model..."
        download_from_google_drive "$GOOGLE_DRIVE_HISTORICAL_ID" "HistoricalZip.zip" || warning "Failed to download Historical model"
        
        if [ -f "HistoricalZip.zip" ]; then
            log "Extracting Historical HMS model..."
            mkdir -p temp_historical
            unzip -q HistoricalZip.zip -d temp_historical/
            cd temp_historical
            
            # Find and extract any nested ZIP files
            for zipfile in *.zip; do
                if [ -f "$zipfile" ]; then
                    log "Extracting nested $zipfile..."
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
    else
        warning "Historical model Google Drive ID not provided"
    fi
    
    # Download shapefile
    if [ -n "$GOOGLE_DRIVE_SHAPEFILE_ID" ]; then
        log "Downloading shapefile..."
        download_from_google_drive "$GOOGLE_DRIVE_SHAPEFILE_ID" "Bexar_County_shapefile.zip" || warning "Failed to download shapefile"
        
        if [ -f "Bexar_County_shapefile.zip" ]; then
            mkdir -p gis_data/shapefiles/
            unzip -q Bexar_County_shapefile.zip -d gis_data/shapefiles/
            rm Bexar_County_shapefile.zip
            log "Shapefile extracted successfully"
        fi
    else
        warning "Shapefile Google Drive ID not provided"
    fi
else
    log "Skipping Step 10: HMS model downloads"
fi

# Step 11: Fix Script Line Endings
if ask_to_run_step "Step 11" "Fix script line endings and permissions" "false"; then
    log "Fixing script line endings..."
    
    find . -name "*.sh" -type f -exec dos2unix {} + 2>/dev/null || true
    find . -name "*.sh" -type f -exec chmod +x {} + || true
    
    log "Script line endings fixed"
else
    log "Skipping Step 11: Line ending fixes"
fi

# Step 12: Create Configuration Files
if ask_to_run_step "Step 12" "Create configuration files (.env and config.yaml)" "$(check_step_12_completed && echo true || echo false)"; then
    log "Creating configuration files..."
    
    # Ask for database password if not set and not skipped
    if [ -z "$DB_PASSWORD" ] || [ "$DB_PASSWORD" = "skipped" ]; then
        DB_PASSWORD=$(prompt_with_default "PostgreSQL password for hms_user" "hms_secure_password_2024")
    fi
    
    # Create .env file for Go
    cat > "$PROJECT_DIR/Go/.env" << EOF
DB_HOST=localhost
DB_PORT=5432
DB_USER=hms_user
DB_PASSWORD=$DB_PASSWORD
DB_NAME=hms_backend
EOF
    
    # Create config.yaml from config.example.yaml
    if [ -f "$PROJECT_DIR/Go/config.example.yaml" ]; then
        log "Creating config.yaml from config.example.yaml..."
        
        # Copy the example config - it already has relative paths!
        cp "$PROJECT_DIR/Go/config.example.yaml" "$PROJECT_DIR/Go/config.yaml"
        
        # Only update the system-specific paths that need to be absolute
        # Update Python paths
        sed -i "s|hms_env_path: \".*\"|hms_env_path: \"$HOME_DIR/miniconda3/envs/hechmsfloodace/bin/python\"|g" "$PROJECT_DIR/Go/config.yaml"
        sed -i "s|grib2cog_env_path: \".*\"|grib2cog_env_path: \"$HOME_DIR/miniconda3/envs/grib2cog/bin/python\"|g" "$PROJECT_DIR/Go/config.yaml"
        
        # Update server settings for AWS deployment
        if $IS_AWS_DEPLOYMENT; then
            sed -i "s|port: \".*\"|port: \"443\"|g" "$PROJECT_DIR/Go/config.yaml"
            sed -i "s|environment: \".*\"|environment: \"production\"|g" "$PROJECT_DIR/Go/config.yaml"
            
            # Add AWS domain to allowed origins if specified
            if [ -n "$AWS_INSTANCE_DOMAIN" ]; then
                # Add the domain to allowed_origins in CORS section
                sed -i "/allowed_origins:/a\    - \"https://$AWS_INSTANCE_DOMAIN\"" "$PROJECT_DIR/Go/config.yaml"
            fi
        fi
        
        log "config.yaml created successfully"
    else
        error "config.example.yaml not found in Go directory"
    fi
else
    log "Skipping Step 12: Configuration file creation"
fi

# Step 13: Setup Database and SSL Certificates
if ask_to_run_step "Step 13" "Setup database schema and SSL certificates" "$(check_step_13_completed && echo true || echo false)"; then
    log "Setting up database schema..."
    
    cd "$PROJECT_DIR/Go"
    
    # Ask for database password if not set and not skipped
    if [ -z "$DB_PASSWORD" ] || [ "$DB_PASSWORD" = "skipped" ]; then
        DB_PASSWORD=$(prompt_with_default "PostgreSQL password for hms_user" "hms_secure_password_2024")
    fi
    
    if [ -f "sql/schema.sql" ]; then
        PGPASSWORD="$DB_PASSWORD" psql -U hms_user -h localhost -d hms_backend -f sql/schema.sql || warning "Database schema may already exist"
    fi
    
    # Generate SSL certificates
    if [ ! -f "server.crt" ]; then
        if $IS_AWS_DEPLOYMENT && [ -n "$AWS_INSTANCE_DOMAIN" ] && [ -n "$LETSENCRYPT_EMAIL" ]; then
            log "Setting up Let's Encrypt certificate for $AWS_INSTANCE_DOMAIN..."
            
            # Configure nginx for certbot
            sudo tee /etc/nginx/sites-available/hms-backend << EOF
server {
    listen 80;
    server_name $AWS_INSTANCE_DOMAIN;
    
    location /.well-known/acme-challenge/ {
        root /var/www/certbot;
    }
    
    location / {
        return 301 https://\$host\$request_uri;
    }
}
EOF
            
            sudo ln -sf /etc/nginx/sites-available/hms-backend /etc/nginx/sites-enabled/
            sudo mkdir -p /var/www/certbot
            sudo nginx -t && sudo systemctl reload nginx
            
            # Obtain certificate
            sudo certbot certonly --webroot -w /var/www/certbot -d $AWS_INSTANCE_DOMAIN --non-interactive --agree-tos -m $LETSENCRYPT_EMAIL
            
            # Copy certificates to project directory
            sudo cp /etc/letsencrypt/live/$AWS_INSTANCE_DOMAIN/fullchain.pem "$PROJECT_DIR/Go/server.crt"
            sudo cp /etc/letsencrypt/live/$AWS_INSTANCE_DOMAIN/privkey.pem "$PROJECT_DIR/Go/server.key"
            sudo chown $USER:$USER "$PROJECT_DIR/Go/server.crt" "$PROJECT_DIR/Go/server.key"
            
            # Update nginx for HTTPS
            sudo tee /etc/nginx/sites-available/hms-backend << EOF
server {
    listen 80;
    server_name $AWS_INSTANCE_DOMAIN;
    return 301 https://\$host\$request_uri;
}

server {
    listen 443 ssl;
    server_name $AWS_INSTANCE_DOMAIN;
    
    ssl_certificate /etc/letsencrypt/live/$AWS_INSTANCE_DOMAIN/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/$AWS_INSTANCE_DOMAIN/privkey.pem;
    
    location / {
        proxy_pass https://localhost:8443;
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;
    }
}
EOF
            
            sudo nginx -t && sudo systemctl reload nginx
        else
            log "Generating self-signed SSL certificates..."
            openssl req -x509 -newkey rsa:4096 -keyout server.key -out server.crt -days 365 -nodes \
                -subj "/C=US/ST=State/L=City/O=Organization/OU=Unit/CN=localhost"
        fi
    fi
    
    log "Database and SSL setup complete"
else
    log "Skipping Step 13: Database and SSL setup"
fi

# Step 14: Build Go Backend
if ask_to_run_step "Step 14" "Build Go backend executable" "$(check_step_14_completed && echo true || echo false)"; then
    log "Building Go backend..."
    
    cd "$PROJECT_DIR/Go"
    go mod download || error "Failed to download Go dependencies"
    go build -o hms-backend . || error "Failed to build Go backend"
    
    log "Go backend built successfully"
else
    log "Skipping Step 14: Go backend build"
fi

# Step 15: Set Permissions
if ask_to_run_step "Step 15" "Set file permissions" "false"; then
    log "Setting file permissions..."
    
    cd "$PROJECT_DIR"
    find . -type d -exec chmod 755 {} +
    find . -type f -exec chmod 644 {} +
    find . -name "*.sh" -type f -exec chmod +x {} +
    if [ -f "Go/hms-backend" ]; then
        chmod +x Go/hms-backend
    fi
    
    log "File permissions set"
else
    log "Skipping Step 15: File permissions"
fi

# Step 16: Environment Variables
if ask_to_run_step "Step 16" "Setup environment variables" "$(check_step_16_completed && echo true || echo false)"; then
    log "Setting up environment variables..."
    
    # Add environment variables to bashrc if not already there
    if ! grep -q "HMS_HOME" ~/.bashrc; then
        cat >> ~/.bashrc << EOF

# HEC-HMS Backend Environment Variables
export HMS_HOME=/opt/hms
export JYTHON_JAR=/opt/jython.jar
export PATH=\$PATH:/usr/local/go/bin
EOF
    fi
    
    log "Environment variables configured"
else
    log "Skipping Step 16: Environment variables"
fi

# Step 17: Create systemd service
if ask_to_run_step "Step 17" "Create systemd service for auto-start" "$(check_step_17_completed && echo true || echo false)"; then
    log "Creating systemd service file..."
    
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
Environment="PATH=/usr/local/go/bin:/usr/bin:/bin"

[Install]
WantedBy=multi-user.target
EOF
    
    if $IS_AWS_DEPLOYMENT; then
        log "Installing systemd service for AWS deployment..."
        sudo cp "$PROJECT_DIR/hms-backend.service" /etc/systemd/system/
        sudo systemctl daemon-reload
        sudo systemctl enable hms-backend
        log "Service installed and enabled. Start with: sudo systemctl start hms-backend"
    else
        log "Systemd service file created at: $PROJECT_DIR/hms-backend.service"
        log "To install as a service, run:"
        log "  sudo cp $PROJECT_DIR/hms-backend.service /etc/systemd/system/"
        log "  sudo systemctl daemon-reload"
        log "  sudo systemctl enable hms-backend"
        log "  sudo systemctl start hms-backend"
    fi
else
    log "Skipping Step 17: Systemd service creation"
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

if $IS_AWS_DEPLOYMENT; then
    echo ""
    info "AWS Deployment Configuration:"
    echo "  - Domain: ${AWS_INSTANCE_DOMAIN:-Self-signed certificates}"
    echo "  - SSL: ${AWS_INSTANCE_DOMAIN:+Let's Encrypt}${AWS_INSTANCE_DOMAIN:-Self-signed}"
    echo "  - Service: hms-backend (systemd)"
fi

echo ""
info "Next Steps:"
echo ""
echo "  1. Review configuration:"
echo "     - $PROJECT_DIR/Go/config.yaml"
echo "     - $PROJECT_DIR/Go/.env"
echo ""

if $IS_AWS_DEPLOYMENT; then
    echo "  2. Start the service:"
    echo "     sudo systemctl start hms-backend"
    echo "     sudo systemctl status hms-backend"
    echo ""
    echo "  3. View logs:"
    echo "     tail -f $PROJECT_DIR/logs/hms-backend.log"
    echo ""
else
    echo "  2. Start the backend:"
    echo "     cd $PROJECT_DIR/Go"
    echo "     ./hms-backend"
    echo ""
fi

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

# Check for missing components
missing_components=()
[ -z "$HMS_GOOGLE_DRIVE_ID" ] && missing_components+=("HEC-HMS")
[ -z "$GOOGLE_DRIVE_REALTIME_ID" ] && missing_components+=("RealTime HMS models")
[ -z "$GOOGLE_DRIVE_HISTORICAL_ID" ] && missing_components+=("Historical HMS models")
[ -z "$GOOGLE_DRIVE_SHAPEFILE_ID" ] && missing_components+=("Bexar County shapefile")

if [ ${#missing_components[@]} -gt 0 ]; then
    warning "The following components were not downloaded (no Google Drive ID provided):"
    for component in "${missing_components[@]}"; do
        echo "  - $component"
    done
    echo ""
    echo "Please download and install these components manually."
fi

echo ""
log "Setup completed successfully!"

# Run verification script if it exists
if [ -f "$PROJECT_DIR/verify_installation.sh" ]; then
    echo ""
    read -p "Do you want to run the installation verification script? (Y/n): " -n 1 -r
    echo ""
    if [[ ! $REPLY =~ ^[Nn]$ ]]; then
        bash "$PROJECT_DIR/verify_installation.sh"
    fi
fi