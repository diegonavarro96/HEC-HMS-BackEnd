#\!/bin/bash

echo "Fixing grib2cog environment..."

# 1. Install system dependencies
echo "Installing system eccodes libraries..."
sudo apt-get update
sudo apt-get install -y libeccodes-dev libeccodes-tools

# 2. Remove the problematic environment if it exists
echo "Removing existing grib2cog environment..."
conda env remove -n grib2cog -y 2>/dev/null || true

# 3. Create new environment
echo "Creating new grib2cog environment..."
conda create -n grib2cog python=3.10 -y

# 4. Activate and install packages
echo "Installing packages..."
source ~/miniconda3/bin/activate grib2cog

# Install core packages
conda install -c conda-forge numpy pandas xarray rioxarray -y

# Install cfgrib using pip (will use system eccodes)
pip install cfgrib

# 5. Test the installation
echo "Testing installation..."
python -c "
import xarray
import rioxarray  
import cfgrib
print('✓ All packages imported successfully\!')
print('  - xarray version:', xarray.__version__)
print('  - cfgrib version:', cfgrib.__version__)
"

echo "Done\! The grib2cog environment should now be working."
