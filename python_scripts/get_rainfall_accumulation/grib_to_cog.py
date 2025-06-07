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

    # --- LONGITUDE COORDINATE CONVERSION: 0-360° to -180 to +180° ---
    # Convert longitude coordinates from 0-360° range to -180 to +180° range
    lon_coords = band.coords['longitude']
    lon_values = lon_coords.values
    
    # Convert longitudes > 180 to negative values
    lon_values = ((lon_values + 180) % 360) - 180
    
    # Update the longitude coordinates
    band = band.assign_coords(longitude=lon_values)
    
    # Sort by longitude to maintain proper ordering
    band = band.sortby('longitude')
    print(f"Longitude range after conversion: {lon_values.min():.2f}° to {lon_values.max():.2f}°", file=sys.stderr)
    # --- END OF LONGITUDE CONVERSION ---

    # --- PROJECTION HANDLING ---
    # First set the original CRS (geographic lat/lon)
    band = band.rio.write_crs(4326)
    
    # Option 1: Keep in EPSG:4326 but ensure proper metadata
    # The data will be displayed correctly if the frontend handles the projection
    # This is the current approach - minimal changes
    
    # Option 2: Reproject to Web Mercator (EPSG:3857) for web display
    # Uncomment the following lines if your frontend expects Web Mercator
    print("Reprojecting to Web Mercator (EPSG:3857)...", file=sys.stderr)
    band = band.rio.reproject("EPSG:3857")
    
    # Option 3: Reproject to Albers Equal Area for minimal distortion over CONUS
    # This projection preserves area and reduces distortion for the continental US
    # Uncomment the following lines to use Albers Equal Area
    # albers_crs = "+proj=aea +lat_1=29.5 +lat_2=45.5 +lat_0=37.5 +lon_0=-96 +x_0=0 +y_0=0 +datum=WGS84 +units=m +no_defs"
    # print("Reprojecting to Albers Equal Area for CONUS...", file=sys.stderr)
    # band = band.rio.reproject(albers_crs)
    
    # Ensure the data has the correct spatial dimensions
    # This helps with the "squished" appearance issue
    if band.rio.crs.to_epsg() == 4326:
        # For EPSG:4326, ensure pixels are interpreted correctly
        # The frontend should handle the latitude-dependent aspect ratio
        print(f"Output CRS: EPSG:4326 (Geographic)", file=sys.stderr)
        print(f"Note: Web maps will need to handle latitude-dependent scaling", file=sys.stderr)
    # --- END OF PROJECTION HANDLING ---

    dst  = os.path.join(outdir, f"{tag}.tif")

    # Ensure the output directory exists (Go code also does this, but good for standalone script use)
    os.makedirs(outdir, exist_ok=True)
    
    print(f"Writing COG to: {dst} with units: {band.attrs.get('units', 'not specified')}", file=sys.stderr)
    band.rio.to_raster(dst, tiled=True, compress="deflate", driver="COG") # Specify COG driver

    # build minimal metadata the Go side needs
    meta = {
        "timestamp": tag,
        "cog_path":  dst,
        "bounds":    list(map(float, band.rio.bounds())), # These will be in the output CRS units
        "width":     int(band.rio.width),                 
        "height":    int(band.rio.height),                
        "units_of_values": "inches",
        "crs": str(band.rio.crs),  # Include CRS information
        "epsg": band.rio.crs.to_epsg() if band.rio.crs else None
    }
    print(json.dumps(meta)) # This is the main JSON output for Go

if __name__ == "__main__":
    if len(sys.argv) != 4:
        sys.exit("Args: <in.grib> <tag> <output_dir>")
    main(*sys.argv[1:])