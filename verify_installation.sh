#!/bin/bash

# HEC-HMS Backend Installation Verification Script
# This script verifies the installation and helps with troubleshooting

set -e

# Color codes
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Functions
check_ok() {
    echo -e "${GREEN}✓${NC} $1"
}

check_fail() {
    echo -e "${RED}✗${NC} $1"
    FAILED_CHECKS=$((FAILED_CHECKS + 1))
}

check_warn() {
    echo -e "${YELLOW}⚠${NC} $1"
    WARNING_CHECKS=$((WARNING_CHECKS + 1))
}

info() {
    echo -e "${BLUE}ℹ${NC} $1"
}

# Initialize counters
FAILED_CHECKS=0
WARNING_CHECKS=0

echo "================================================================="
echo "   HEC-HMS Backend Installation Verification"
echo "================================================================="
echo ""

# Check system dependencies
echo "Checking System Dependencies..."
echo "------------------------------"

# Check Java versions
if command -v java >/dev/null 2>&1; then
    JAVA_VERSION=$(java -version 2>&1 | head -n 1)
    check_ok "Java installed: $JAVA_VERSION"
else
    check_fail "Java not found"
fi

# Check Java 17
if [ -f "/usr/lib/jvm/java-17-openjdk-amd64/bin/java" ]; then
    check_ok "Java 17 installed"
else
    check_fail "Java 17 not found"
fi

# Check Go
if command -v go >/dev/null 2>&1; then
    GO_VERSION=$(go version)
    check_ok "Go installed: $GO_VERSION"
else
    check_fail "Go not found"
fi

# Check PostgreSQL
if systemctl is-active --quiet postgresql || service postgresql status >/dev/null 2>&1; then
    check_ok "PostgreSQL is running"
else
    check_fail "PostgreSQL is not running"
fi

# Check GDAL
if command -v gdalinfo >/dev/null 2>&1; then
    check_ok "GDAL installed"
else
    check_fail "GDAL not found"
fi

# Check Conda
if [ -d "$HOME/miniconda3" ]; then
    check_ok "Miniconda installed"
    
    # Check environments
    if $HOME/miniconda3/bin/conda env list | grep -q "hechmsfloodace"; then
        check_ok "hechmsfloodace environment exists"
    else
        check_fail "hechmsfloodace environment not found"
    fi
    
    if $HOME/miniconda3/bin/conda env list | grep -q "grib2cog"; then
        check_ok "grib2cog environment exists"
    else
        check_fail "grib2cog environment not found"
    fi
else
    check_fail "Miniconda not found"
fi

echo ""
echo "Checking HEC-HMS Installation..."
echo "--------------------------------"

# Check Jython
if [ -f "/opt/jython.jar" ]; then
    check_ok "Jython installed at /opt/jython.jar"
    if java -jar /opt/jython.jar --version >/dev/null 2>&1; then
        check_ok "Jython is functional"
    else
        check_fail "Jython test failed"
    fi
else
    check_fail "Jython not found at /opt/jython.jar"
fi

# Check HEC-HMS
if [ -d "/opt/hms" ]; then
    check_ok "HEC-HMS directory exists"
    if [ -f "/opt/hms/hec-hms.sh" ]; then
        check_ok "HEC-HMS executable found"
        if /opt/hms/hec-hms.sh -help >/dev/null 2>&1; then
            check_ok "HEC-HMS is functional"
        else
            check_warn "HEC-HMS functionality test failed (this might be normal)"
        fi
    else
        check_fail "HEC-HMS executable not found"
    fi
else
    check_fail "HEC-HMS not installed at /opt/hms"
fi

echo ""
echo "Checking Project Structure..."
echo "-----------------------------"

PROJECT_DIR="$HOME/hms-backend"

if [ -d "$PROJECT_DIR" ]; then
    check_ok "Project directory exists: $PROJECT_DIR"
    
    # Check key directories
    DIRS=(
        "Go"
        "python_scripts"
        "HMSScripts"
        "hms_models/RealTime/LeonCreek"
        "hms_models/Historical/LeonCreek"
        "gis_data/shapefiles"
        "logs"
        "JSON"
        "gribFiles"
    )
    
    for dir in "${DIRS[@]}"; do
        if [ -d "$PROJECT_DIR/$dir" ]; then
            check_ok "Directory exists: $dir"
        else
            check_warn "Directory missing: $dir"
        fi
    done
    
    # Check for HMS models
    if [ -f "$PROJECT_DIR/hms_models/RealTime/LeonCreek/LeonCreek.hms" ]; then
        check_ok "RealTime HMS model found"
    else
        check_warn "RealTime HMS model not found - manual download required"
    fi
    
    if [ -f "$PROJECT_DIR/hms_models/Historical/LeonCreek/LeonCreek.hms" ]; then
        check_ok "Historical HMS model found"
    else
        check_warn "Historical HMS model not found - manual download required"
    fi
    
    # Check shapefile
    if [ -f "$PROJECT_DIR/gis_data/shapefiles/Bexar_County.shp" ]; then
        check_ok "Shapefile found"
    else
        check_warn "Shapefile not found - manual download required"
    fi
    
    # Check Go backend
    if [ -f "$PROJECT_DIR/Go/hms-backend" ]; then
        check_ok "Go backend binary exists"
        if [ -x "$PROJECT_DIR/Go/hms-backend" ]; then
            check_ok "Go backend is executable"
        else
            check_fail "Go backend is not executable"
        fi
    else
        check_warn "Go backend not built yet"
    fi
    
    # Check configuration files
    if [ -f "$PROJECT_DIR/Go/.env" ]; then
        check_ok ".env file exists"
    else
        check_fail ".env file missing"
    fi
    
    if [ -f "$PROJECT_DIR/Go/config.yaml" ]; then
        check_ok "config.yaml exists"
    else
        check_fail "config.yaml missing"
    fi
    
    # Check SSL certificates
    if [ -f "$PROJECT_DIR/Go/server.crt" ] && [ -f "$PROJECT_DIR/Go/server.key" ]; then
        check_ok "SSL certificates exist"
    else
        check_warn "SSL certificates missing"
    fi
    
else
    check_fail "Project directory not found at $PROJECT_DIR"
fi

echo ""
echo "Checking Database..."
echo "--------------------"

# Try to connect to database
if [ -f "$PROJECT_DIR/Go/.env" ]; then
    source "$PROJECT_DIR/Go/.env"
    if PGPASSWORD="$DB_PASSWORD" psql -U hms_user -h localhost -d hms_backend -c '\dt' >/dev/null 2>&1; then
        check_ok "Database connection successful"
    else
        check_fail "Cannot connect to database"
    fi
else
    check_warn "Cannot test database - .env file missing"
fi

echo ""
echo "Checking Script Permissions..."
echo "------------------------------"

if [ -d "$PROJECT_DIR" ]; then
    # Check shell scripts
    SCRIPT_COUNT=$(find "$PROJECT_DIR" -name "*.sh" -type f | wc -l)
    EXEC_COUNT=$(find "$PROJECT_DIR" -name "*.sh" -type f -executable | wc -l)
    
    if [ "$SCRIPT_COUNT" -eq "$EXEC_COUNT" ] && [ "$SCRIPT_COUNT" -gt 0 ]; then
        check_ok "All shell scripts are executable ($EXEC_COUNT scripts)"
    else
        check_warn "Some shell scripts are not executable ($EXEC_COUNT/$SCRIPT_COUNT)"
    fi
fi

echo ""
echo "================================================================="
echo "                    Verification Summary"
echo "================================================================="
echo ""

TOTAL_CHECKS=$((FAILED_CHECKS + WARNING_CHECKS))

if [ $FAILED_CHECKS -eq 0 ] && [ $WARNING_CHECKS -eq 0 ]; then
    echo -e "${GREEN}All checks passed!${NC} The installation appears to be complete."
elif [ $FAILED_CHECKS -eq 0 ]; then
    echo -e "${YELLOW}Installation complete with $WARNING_CHECKS warnings.${NC}"
    echo "The system should be functional, but some optional components may need attention."
else
    echo -e "${RED}Installation has $FAILED_CHECKS critical issues and $WARNING_CHECKS warnings.${NC}"
    echo "Please address the critical issues before proceeding."
fi

echo ""
echo "Quick Test Commands:"
echo "-------------------"
echo ""
echo "1. Test Go backend:"
echo "   cd $PROJECT_DIR/Go && ./hms-backend"
echo ""
echo "2. Test database connection:"
echo "   PGPASSWORD=<your_password> psql -U hms_user -h localhost -d hms_backend"
echo ""
echo "3. Test Jython:"
echo "   java -jar /opt/jython.jar -c \"print('Jython works!')\""
echo ""
echo "4. Test HEC-HMS:"
echo "   /opt/hms/hec-hms.sh -help"
echo ""

if [ $WARNING_CHECKS -gt 0 ] || [ $FAILED_CHECKS -gt 0 ]; then
    echo "Troubleshooting Tips:"
    echo "--------------------"
    
    if ! systemctl is-active --quiet postgresql && ! service postgresql status >/dev/null 2>&1; then
        echo "- Start PostgreSQL: sudo service postgresql start"
    fi
    
    if [ ! -f "$PROJECT_DIR/hms_models/RealTime/LeonCreek/LeonCreek.hms" ]; then
        echo "- Download HMS models from Google Drive and extract to hms_models/"
    fi
    
    if [ ! -f "$PROJECT_DIR/gis_data/shapefiles/Bexar_County.shp" ]; then
        echo "- Download Bexar County shapefile and place in gis_data/shapefiles/"
    fi
    
    if [ ! -f "$PROJECT_DIR/Go/hms-backend" ]; then
        echo "- Build Go backend: cd $PROJECT_DIR/Go && go build -o hms-backend ."
    fi
    
    echo ""
fi