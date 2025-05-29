#!/usr/bin/env python3
"""
Usage:
    python grib_to_cog.py input.grib 20250519_18Z /data/cogs
Prints a JSON blob describing the COG it created.
"""

import sys, json, tempfile, os # tempfile is not used, can be removed
import xarray as xr
# import rasterio # Not directly used if using rioxarray for I/O
import rioxarray as rio

def main(infile, tag, outdir):
    ds  = xr.open_dataset(infile, engine="cfgrib")
    print(ds, file=sys.stderr)
    print(ds.data_vars, file=sys.stderr)
    
    band = ds['unknown'] # Or whatever your actual variable name is

    # --- UNIT CONVERSION: mm to inches ---
    # IMPORTANT: This assumes the original units of 'band' are millimeters!
    # If they are not, this conversion will be incorrect.
    mm_to_inches_conversion_factor = 25.4
    band = band / mm_to_inches_conversion_factor
    
    # Optionally, update the 'units' attribute in the DataArray's metadata
    # This is good practice for the self-descriptiveness of the COG.
    if 'units' in band.attrs:
        original_units = band.attrs['units']
        # You might want to check if original_units was 'mm' or similar before overwriting
        print(f"Original units attribute: {original_units}. Updating to 'inches'.", file=sys.stderr)
    band.attrs['units'] = 'inches' 
    # --- END OF UNIT CONVERSION ---

    band = band.rio.write_crs(4326)       # EPSG:4326 lat/lon
    dst  = os.path.join(outdir, f"{tag}.tif")

    # Ensure the output directory exists (Go code also does this, but good for standalone script use)
    os.makedirs(outdir, exist_ok=True)
    
    print(f"Writing COG to: {dst} with units: {band.attrs.get('units', 'not specified')}", file=sys.stderr)
    band.rio.to_raster(dst, tiled=True, compress="deflate", driver="COG") # Specify COG driver

    # build minimal metadata the Go side needs
    meta = {
        "timestamp": tag,
        "cog_path":  dst,
        "bounds":    list(map(float, band.rio.bounds())), # These won't change due to unit conversion
        "width":     int(band.rio.width),                 # These won't change
        "height":    int(band.rio.height),                 # These won't change
        # You might want to add a "units": "inches" field to your JSON meta as well
        "units_of_values": "inches" 
    }
    print(json.dumps(meta)) # This is the main JSON output for Go

if __name__ == "__main__":
    if len(sys.argv) != 4:
        sys.exit("Args: <in.grib> <tag> <output_dir>")
    main(*sys.argv[1:])