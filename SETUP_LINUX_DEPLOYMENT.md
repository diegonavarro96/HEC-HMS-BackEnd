# HEC-HMS Backend - Linux/Ubuntu Deployment Setup Guide

## Overview
This guide provides detailed steps to deploy the HEC-HMS Backend on a fresh Ubuntu Linux system. It covers all dependencies, configurations, and fixes implemented during the WSL migration.

## Prerequisites
- Ubuntu 20.04+ (or compatible distribution)
- Root/sudo access
- At least 16GB RAM recommended
- 50GB+ free disk space

## Step 1: System Dependencies

### 1.1 Update System
```bash
sudo apt update && sudo apt upgrade -y
```

### 1.2 Install Basic Dependencies
```bash
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
    unzip
```

### 1.3 Install Go 1.21+
```bash
# Download and install Go
wget https://go.dev/dl/go1.21.5.linux-amd64.tar.gz
sudo rm -rf /usr/local/go && sudo tar -C /usr/local -xzf go1.21.5.linux-amd64.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
source ~/.bashrc
```

## Step 2: Java Configuration

### 2.1 Set Java 17 for specific applications
```bash
# Install if not already installed
sudo apt install openjdk-17-jdk

# Keep Java 8 as default for compatibility
sudo update-alternatives --config java
# Select Java 8 when prompted
```

## Step 3: PostgreSQL Setup

### 3.1 Configure PostgreSQL
```bash
# Start PostgreSQL
sudo service postgresql start

# Create database and user
sudo -u postgres psql << EOF
CREATE USER hms_user WITH PASSWORD 'your_secure_password_here';
CREATE DATABASE hms_backend OWNER hms_user;
GRANT ALL PRIVILEGES ON DATABASE hms_backend TO hms_user;
EOF
```

### 3.2 Create .env file in Go directory
```bash
cd /path/to/HEC-HMS-BackEnd/Go
cat > .env << EOF
DB_HOST=localhost
DB_PORT=5432
DB_USER=hms_user
DB_PASSWORD=your_secure_password_here
DB_NAME=hms_backend
EOF
```

## Step 4: Python Environment Setup

### 4.1 Install Miniconda
```bash
wget https://repo.anaconda.com/miniconda/Miniconda3-latest-Linux-x86_64.sh
bash Miniconda3-latest-Linux-x86_64.sh -b -p $HOME/miniconda3
eval "$($HOME/miniconda3/bin/conda shell.bash hook)"
echo 'eval "$($HOME/miniconda3/bin/conda shell.bash hook)"' >> ~/.bashrc
```

### 4.2 Create Conda Environments
```bash
# Main HMS environment
conda create -n hechmsfloodace python=3.10 -y
conda activate hechmsfloodace
conda install -c conda-forge requests beautifulsoup4 watchdog pyyaml flask flask-cors -y

# GRIB processing environment
conda create -n grib2cog python=3.10 -y
conda activate grib2cog
conda install -c conda-forge xarray rioxarray cfgrib eccodes -y
```

## Step 5: Jython Installation

```bash
# Download Jython standalone
wget https://repo1.maven.org/maven2/org/python/jython-standalone/2.7.3/jython-standalone-2.7.3.jar

# Move to /opt
sudo mkdir -p /opt
sudo mv jython-standalone-2.7.3.jar /opt/jython.jar
sudo chmod 755 /opt/jython.jar

# Test installation
java -jar /opt/jython.jar --version
```

## Step 6: HEC-HMS Installation

### 6.1 Extract HEC-HMS (assuming you have the Linux version)
```bash
# Create directory
sudo mkdir -p /opt/hms

# Extract HMS (adjust path to your HMS tar/zip file)
sudo tar -xf hec-hms-4.12-linux-x64.tar -C /opt/hms --strip-components=1

# Set permissions
sudo chmod -R 755 /opt/hms
```

### 6.2 Verify Installation
```bash
/opt/hms/hec-hms.sh -help
```

## Step 7: Project Setup

### 7.1 Clone/Copy Project
```bash
# Clone or copy your project to a path WITHOUT spaces
# IMPORTANT: Avoid paths with spaces due to HEC-HMS limitations
mkdir -p ~/hms-backend
cd ~/hms-backend
# Copy your project files here
```

### 7.2 Create Required Directories
```bash
cd ~/hms-backend

# Create all required directories
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
```

### 7.3 Download HMS Models from Google Drive

#### 7.3.1 Install gdown for Google Drive downloads
```bash
pip install gdown
```

#### 7.3.2 Download HMS Model Files
```bash
cd ~/hms-backend

# Download RealTimeZip.zip (replace FILE_ID with actual Google Drive file ID)
# To get FILE_ID: Right-click file in Google Drive > Get link > Extract ID from URL
# Example URL: https://drive.google.com/file/d/FILE_ID/view?usp=sharing

# Download RealTimeZip
gdown --id REALTIME_FILE_ID -O RealTimeZip.zip

# Download HistoricalZip
gdown --id HISTORICAL_FILE_ID -O HistoricalZip.zip

# Alternative method using wget if files are publicly shared:
# wget --no-check-certificate 'https://drive.google.com/uc?export=download&id=REALTIME_FILE_ID' -O RealTimeZip.zip
# wget --no-check-certificate 'https://drive.google.com/uc?export=download&id=HISTORICAL_FILE_ID' -O HistoricalZip.zip
```

#### 7.3.3 Extract HMS Models
```bash
# Extract RealTime models
unzip RealTimeZip.zip -d temp_realtime/
cd temp_realtime
unzip LeonCreek.zip
cp -r LeonCreek ~/hms-backend/hms_models/RealTime/
cd ..

# Extract Historical models
unzip HistoricalZip.zip -d temp_historical/
cd temp_historical
unzip LeonCreek.zip
cp -r LeonCreek ~/hms-backend/hms_models/Historical/
cd ..

# Clean up
rm -rf temp_realtime temp_historical
rm RealTimeZip.zip HistoricalZip.zip
```

#### 7.3.4 Verify HMS Model Structure
```bash
# The structure should be:
# hms_models/
# ├── RealTime/
# │   └── LeonCreek/
# │       ├── LeonCreek.hms
# │       ├── RainRealTime.control
# │       ├── Rainfall/
# │       └── dssArchive/
# └── Historical/
#     └── LeonCreek/
#         ├── LeonCreek.hms
#         ├── RainHistorical.control
#         └── ...

# Verify structure
tree -L 3 ~/hms-backend/hms_models/
```

### 7.4 Copy GIS Data

#### 7.4.1 Download Shapefile from Google Drive
```bash
# Download shapefile and associated files
# You need: Bexar_County.shp, .shx, .dbf, .prj files
# These might be in a zip file on Google Drive

# Download shapefile zip (replace SHAPEFILE_FILE_ID with actual ID)
gdown --id SHAPEFILE_FILE_ID -O Bexar_County_shapefile.zip

# Extract to gis_data/shapefiles
unzip Bexar_County_shapefile.zip -d ~/hms-backend/gis_data/shapefiles/

# Verify files
ls -la ~/hms-backend/gis_data/shapefiles/Bexar_County.*
```

#### 7.4.2 Alternative: Manual Copy
```bash
# If you have the files locally
cp /path/to/Bexar_County.* ~/hms-backend/gis_data/shapefiles/
```

### 7.5 Update HMS Model Paths

**IMPORTANT**: After extracting HMS models, you need to update internal paths in the model files:

```bash
# The HMS model files may contain Windows paths that need to be updated
# Check and update paths in:
# - LeonCreek.hms
# - RainRealTime.control
# - RainHistorical.control

# Example: Update DSS file paths in control files
cd ~/hms-backend/hms_models/Historical/LeonCreek
# Edit RainHistorical.control to update paths

cd ~/hms-backend/hms_models/RealTime/LeonCreek
# Edit RainRealTime.control to update paths
```

## Step 8: Fix Script Line Endings

### 8.1 Convert all shell scripts to Unix format
```bash
cd ~/hms-backend
find . -name "*.sh" -type f -exec dos2unix {} +
```

### 8.2 Make all scripts executable
```bash
find . -name "*.sh" -type f -exec chmod +x {} +
```

## Step 9: Update Configuration Files

### 9.1 Update Go config.yaml
```yaml
# ~/hms-backend/Go/config.yaml
# Update all paths to use absolute paths without spaces
# Key paths to update:

paths:
  hms_models_dir: "/home/your_user/hms-backend/hms_models"
  python_scripts_dir: "../python_scripts"
  grib_files_dir: "../gribFiles"
  shapefile_path: "/home/your_user/hms-backend/gis_data/shapefiles/Bexar_County.shp"
  hms_historical_models_dir: "/home/your_user/hms-backend/hms_models/Historical"
  # ... update all other paths

python:
  hms_env_path: "/home/your_user/miniconda3/envs/hechmsfloodace/bin/python"
  grib2cog_env_path: "/home/your_user/miniconda3/envs/grib2cog/bin/python"

jython:
  executable_path: "java -jar /opt/jython.jar"
  batch_scripts_dir: "/home/your_user/hms-backend/python_scripts/Jython_Scripts/batchScripts"

hms:
  executable_path: "/opt/hms/hec-hms.sh"
  # ... update control file paths
```

### 9.2 Update Jython Script Paths
All Jython scripts have been updated to use environment variables:
- `VORTEX_SHAPEFILE_PATH` - Path to shapefile
- `VORTEX_OUTPUT_DSS_PATH` - Path to output DSS file

### 9.3 Update HMS Project Paths
Update paths in these files to use absolute paths without spaces:
- `HMSScripts/computeHistorical.script`
- `HMSScripts/computeRealTime.script`

Example:
```python
OpenProject("Leon_46", "/home/your_user/hms-backend/hms_models/Historical/LeonCreek")
```

## Step 10: Script Updates Applied

### 10.1 Shell Script Updates
The following scripts have been updated for Linux compatibility:

1. **MergeGRIBFilesRealTimePass2Batch.sh**
   - Uses HMS's Java 17: `/opt/hms/jre/bin/java`
   - Uses HMS's Vortex libraries
   - Accepts arguments in order: input_folder, shapefile_path, output_dss
   - Exports environment variables for Jython

2. **Other Merge Scripts** (similar updates):
   - MergeGRIBFilesRealTimeBatch.sh
   - MergeGRIBFilesRealTimeHRRBatch.sh
   - CombineTwoDssFilesPass1Pass2Batch.sh
   - CombineTwoDssFilesRealTimeAndHRRBatch.sh

3. **HMSHistoricalBatch.sh & HMSRealTimeBatch.sh**
   - Uses symlink workaround for spaces in paths
   - HMS executable path: `/opt/hms/hec-hms.sh`

### 10.2 Jython Script Updates
All Jython scripts now use environment variables instead of hardcoded paths:
- MergeGRIBFilesRealTimePass2Jython.py
- MergeGRIBFilesRealTimeJython.py
- MergeGRIBFilesRealTimeHRRJython.py

## Step 11: Database Setup

```bash
cd ~/hms-backend/Go

# Run database migrations
psql -U hms_user -d hms_backend -f sql/schema.sql

# Generate SSL certificates for development
openssl req -x509 -newkey rsa:4096 -keyout server.key -out server.crt -days 365 -nodes \
    -subj "/C=US/ST=State/L=City/O=Organization/OU=Unit/CN=localhost"
```

## Step 12: Build and Run

### 12.1 Build Go Backend
```bash
cd ~/hms-backend/Go
go mod download
go build -o hms-backend .
```

### 12.2 Run the Backend
```bash
./hms-backend
```

## Step 13: Verify Pipelines

### 13.1 Test Historical Pipeline
```bash
curl -k -X POST https://localhost:8443/api/run-hms-pipeline-historical \
  -H "Content-Type: application/json" \
  -d '{
    "startDate": "2025-05-08",
    "endDate": "2025-05-08",
    "startTime": "00:00",
    "endTime": "23:00"
  }'
```

### 13.2 Test Real-Time Pipeline
```bash
curl -k -X POST https://localhost:8443/api/run-hms-pipeline \
  -H "Content-Type: application/json"
```

## Critical Path Considerations

### Paths WITHOUT Spaces
Due to HEC-HMS limitations with spaces in paths, ensure:
1. Project root path has no spaces
2. All script paths have no spaces
3. Use symlinks if necessary

### File Permissions
```bash
# Ensure proper ownership
sudo chown -R $USER:$USER ~/hms-backend

# Set directory permissions
find ~/hms-backend -type d -exec chmod 755 {} +

# Set file permissions
find ~/hms-backend -type f -exec chmod 644 {} +

# Make scripts executable
find ~/hms-backend -name "*.sh" -type f -exec chmod +x {} +
```

### Environment Variables
Add to ~/.bashrc:
```bash
export HMS_HOME=/opt/hms
export JYTHON_JAR=/opt/jython.jar
export PATH=$PATH:/usr/local/go/bin
```

## Troubleshooting

### Common Issues:

1. **GLIBC Version Conflicts**
   - Solution: Use HMS's bundled Java and minimal LD_LIBRARY_PATH

2. **"Cannot find script file" Error**
   - Solution: Scripts use symlinks to avoid spaces in paths

3. **GDAL Library Issues**
   - Solution: Scripts use HMS's bundled GDAL libraries

4. **Line Ending Issues**
   - Solution: Run dos2unix on all shell scripts

5. **Permission Denied**
   - Solution: Check file permissions and ownership

### Log Locations
- Go backend logs: `~/hms-backend/logs/`
- HMS logs: Check HMS output directory
- Script debug: Add `set -x` to shell scripts

## Performance Optimization

### For Better I/O Performance:
1. Use native Linux filesystem instead of /mnt/c or /mnt/d
2. Increase Java heap size in scripts if needed
3. Use SSD for working directories

### WSL Specific (if applicable):
Create .wslconfig in Windows:
```ini
[wsl2]
memory=16GB
processors=8
swap=8GB
```

## Final Checklist

- [ ] All system dependencies installed
- [ ] PostgreSQL configured and running
- [ ] Python environments created
- [ ] Jython installed and tested
- [ ] HEC-HMS installed and verified
- [ ] Project files copied to path without spaces
- [ ] All scripts converted to Unix line endings
- [ ] All scripts made executable
- [ ] Configuration files updated with correct paths
- [ ] Database migrations run
- [ ] SSL certificates generated
- [ ] Go backend built successfully
- [ ] Both pipelines tested and working

## Notes

- The real-time pipeline uses similar scripts and should work with these configurations
- Always use absolute paths in configuration files
- Keep Java 8 as system default but use Java 17 for HMS/Vortex operations
- Monitor disk space as GRIB files and DSS files can be large