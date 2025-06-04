# Setup Script Auto-Detection Changes

## Summary of Modifications

The `setup_hms_backend.sh` script has been enhanced with auto-detection capabilities that check for completed installation steps and automatically skip them.

## Key Changes

### 1. Auto-Detection Functions
Added 17 check functions (`check_step_X_completed()`) that detect if each step has already been completed:

- **Step 1**: Checks if key system packages are installed (curl, wget, git, postgresql, gdal-bin, dos2unix)
- **Step 2**: Checks if Go 1.21 is installed and in PATH
- **Step 3**: Checks if Java 8 is set as the default Java version
- **Step 4**: Checks if PostgreSQL database "hms_backend" and user "hms_user" exist
- **Step 5**: Checks if Miniconda is installed in ~/miniconda3
- **Step 6**: Checks if conda environments "hechmsfloodace" and "grib2cog" exist
- **Step 7**: Checks if Jython jar exists at /opt/jython.jar
- **Step 8**: Checks if HEC-HMS is installed at /opt/hms
- **Step 9**: Checks if project structure exists with key folders
- **Step 10**: Checks if HMS models are downloaded for both RealTime and Historical
- **Step 11**: Always runs (line endings fix is quick)
- **Step 12**: Checks if config files (.env and config.yaml) exist
- **Step 13**: Checks if database tables exist (specifically checks for "users" table)
- **Step 14**: Checks if Go binary (hms-backend) exists
- **Step 15**: Always runs (permissions fix is quick)
- **Step 16**: Checks if environment variables are in .bashrc
- **Step 17**: Checks if systemd service file exists

### 2. Auto-Detection Process
- The script now runs `auto_detect_completed_steps()` at startup
- Shows a clear list of which steps are already completed
- Automatically merges detected skips with any command-line specified skips
- Works in both normal and interactive modes

### 3. User Experience Improvements
- Users see which steps will be automatically skipped before confirmation
- Clear messaging about what's already installed
- Prevents redundant installations
- Saves time on repeated runs

### 4. HEC-HMS Download URL Update
Updated the HEC-HMS download URL to the new version:
- Old: `https://github.com/HydrologicEngineeringCenter/hec-downloads/releases/download/1.0.27/hec-hms-4.12-linux-x64.tar.gz`
- New: `https://github.com/HydrologicEngineeringCenter/hec-downloads/releases/download/1.0.32/HEC-HMS-4.12-linux64.tar.gz`

## Usage Examples

### Normal Mode with Auto-Detection
```bash
./setup_hms_backend.sh
# Script will auto-detect completed steps and show them before proceeding
```

### Skip Additional Steps
```bash
./setup_hms_backend.sh --skip 10,12
# Will skip steps 10 and 12 plus any auto-detected completed steps
```

### Interactive Mode
```bash
./setup_hms_backend.sh --interactive
# Shows auto-detected steps and lets you choose additional steps to skip
```

## Benefits

1. **Idempotent**: Script can be run multiple times safely
2. **Time-Saving**: Skips already completed installations
3. **Transparent**: Shows what's being skipped and why
4. **Flexible**: Can still manually skip steps if needed
5. **Smart**: Detects partial installations and existing configurations