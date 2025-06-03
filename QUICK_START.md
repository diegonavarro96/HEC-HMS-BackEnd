# HEC-HMS Backend Quick Start Guide

## Prerequisites
- Ubuntu 20.04+ server with SSH access
- At least 16GB RAM
- 50GB+ free disk space
- Google Drive file IDs for HMS models and shapefile (optional)

## Installation

### 1. Download and prepare the setup script
```bash
# Clone or download the repository
git clone [your-repo-url] HEC-HMS-BackEnd
cd HEC-HMS-BackEnd

# Make the setup script executable
chmod +x setup_hms_backend.sh
```

### 2. Run the automated setup
```bash
# Run the setup script (DO NOT run as root)
./setup_hms_backend.sh
```

The script will prompt you for:
- PostgreSQL password for the hms_user
- Google Drive file IDs (optional - can be added later)
- Path to HEC-HMS tar file (or 'download' to auto-download)

### 3. Verify installation
```bash
# Make the verification script executable
chmod +x verify_installation.sh

# Run verification
./verify_installation.sh
```

## Manual Steps (if needed)

### Download HMS Models from Google Drive

If you didn't provide Google Drive IDs during setup:

1. Download the following files from Google Drive:
   - RealTimeZip.zip
   - HistoricalZip.zip
   - Bexar_County_shapefile.zip

2. Extract HMS models:
```bash
cd ~/hms-backend

# Extract RealTime model
unzip RealTimeZip.zip -d temp_realtime/
cd temp_realtime
unzip LeonCreek.zip
cp -r LeonCreek ../hms_models/RealTime/
cd ..
rm -rf temp_realtime

# Extract Historical model
unzip HistoricalZip.zip -d temp_historical/
cd temp_historical
unzip LeonCreek.zip
cp -r LeonCreek ../hms_models/Historical/
cd ..
rm -rf temp_historical

# Extract shapefile
unzip Bexar_County_shapefile.zip -d gis_data/shapefiles/
```

### Update Configuration

1. Edit `~/hms-backend/Go/config.yaml` to ensure all paths are correct
2. Update HMS model paths in control files if needed

## Starting the Backend

### Manual Start
```bash
cd ~/hms-backend/Go
./hms-backend
```

### As a System Service
```bash
# Copy the service file
sudo cp ~/hms-backend/hms-backend.service /etc/systemd/system/

# Enable and start the service
sudo systemctl daemon-reload
sudo systemctl enable hms-backend
sudo systemctl start hms-backend

# Check status
sudo systemctl status hms-backend
```

## Testing the Pipelines

### Test Historical Pipeline
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

### Test Real-Time Pipeline
```bash
curl -k -X POST https://localhost:8443/api/run-hms-pipeline \
  -H "Content-Type: application/json"
```

## Troubleshooting

### Common Issues

1. **PostgreSQL not running**
   ```bash
   sudo service postgresql start
   ```

2. **Permission denied errors**
   ```bash
   cd ~/hms-backend
   find . -name "*.sh" -type f -exec chmod +x {} +
   ```

3. **Database connection failed**
   - Check `.env` file in `~/hms-backend/Go/`
   - Verify PostgreSQL user and password

4. **HMS models not found**
   - Ensure models are extracted to correct directories
   - Check paths in `config.yaml`

### View Logs
```bash
# Application logs
tail -f ~/hms-backend/logs/*.log

# If using systemd service
sudo journalctl -u hms-backend -f
```

## Environment Variables

Add to your `~/.bashrc` if not already added:
```bash
export HMS_HOME=/opt/hms
export JYTHON_JAR=/opt/jython.jar
export PATH=$PATH:/usr/local/go/bin
eval "$($HOME/miniconda3/bin/conda shell.bash hook)"
```

Then reload:
```bash
source ~/.bashrc
```

## Support

For issues or questions:
1. Check the installation log: `setup_log_[timestamp].log`
2. Run the verification script: `./verify_installation.sh`
3. Review the detailed setup guide: `SETUP_LINUX_DEPLOYMENT.md`