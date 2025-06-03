# -*- coding: utf-8 -*-
# MergeGRIBFilesRealTimeJython.py - Jython version
# This script merges GRIB files using HEC-Vortex BatchImporter
# Takes a folder path as input and processes all GRIB files within it
import os
import sys

# Delete every *.class under the folder that holds this .py file
BASE_DIR = os.path.dirname(os.path.abspath(__file__))
for root, _, files in os.walk(BASE_DIR):
    for f in files:
        if f.endswith(".class"):
            try:
                os.remove(os.path.join(root, f))
            except OSError:
                pass  # ignore files that are locked

from mil.army.usace.hec.vortex.io import BatchImporter
from mil.army.usace.hec.vortex.geo import WktFactory
from java.util import ArrayList, HashMap
from java.io import File

import traceback


# Configuration - read from environment variables or use defaults
SHAPEFILE_PATH = os.environ.get('VORTEX_SHAPEFILE_PATH', '/mnt/d/Floodace Projects/HEC-HMS-BackEnd/gis_data/shapefiles/Bexar_County.shp')
OUTPUT_DSS_PATH = os.environ.get('VORTEX_OUTPUT_DSS_PATH_HRR', '/mnt/d/Floodace Projects/HEC-HMS-BackEnd/hms_models/LeonCreek/Rainfall/HRR.dss')
VARIABLES = ["Total_precipitation_surface_Mixed_intervals_Accumulation"]  # Update with actual variables from variablesv3

def collect_grib_files(folder_path):
    """Collect GRIB files from the given folder"""
    files = []
    
    if not os.path.isdir(folder_path):
        raise ValueError("Input path is not a directory: %s" % folder_path)
    
    print("Scanning folder: %s" % folder_path)
    
    for filename in os.listdir(folder_path):
        if "wrfsfcf" in filename and filename.lower().endswith((".grb2", ".grib2")):
            file_path = os.path.join(folder_path, filename)
            files.append(file_path)
    
    print("Found %d GRIB files in folder." % len(files))
    #print ("Files Found :  ",files)
    return files

def merge_grib_folder(input_folder, shapefile_path=None, output_dss=None, variables=None):
    """
    Merges GRIB files from a single folder using HEC-Vortex BatchImporter.
    """
    shp_path = shapefile_path if shapefile_path else SHAPEFILE_PATH
    dss_path = output_dss if output_dss else OUTPUT_DSS_PATH
    vars_list = variables if variables else VARIABLES

    if not os.path.exists(input_folder):
        raise ValueError("Input folder not found: %s" % input_folder)
    if not os.path.exists(shp_path):
        raise ValueError("Shapefile not found: %s" % shp_path)

    output_dir = os.path.dirname(dss_path)
    if not os.path.exists(output_dir):
        try:
            os.makedirs(output_dir)
        except: # Simplified bare except, consider specific exceptions
            raise IOError("Failed to create output directory: %s" % output_dir)

    files = collect_grib_files(input_folder)
    if not files:
        # Changed from RuntimeError to returning messages for consistency,
        # or you can keep raising an error if preferred.
        print("No GRIB files found in folder: %s" % input_folder)
        return {"stdout": "No GRIB files found in folder: %s" % input_folder, "stderr": ""}


    print("Processing %d GRIB files..." % len(files))
    
    # Initialize message lists ONCE before the try block
    stdout_msgs = []
    stderr_msgs = []
    
    try:
        # The redundant re-initialization of stdout_msgs and stderr_msgs
        # that was here (lines 94-95 in your script) has been removed.

        for f in files:
            imp_builder = BatchImporter.builder()
            single = ArrayList()
            single.add(f)
            imp_builder.inFiles(single)
            imp_builder.variables(vars_list)

            geo_options = HashMap()
            geo_options.put("pathToShp", shp_path)
            geo_options.put("targetCellSize", "1000")
            geo_options.put("targetWkt", WktFactory.shg())
            geo_options.put("resamplingMethod", "Nearest Neighbor")
            imp_builder.geoOptions(geo_options)

            imp_builder.destination(dss_path)
            write_options = HashMap()
            write_options.put("partA", "SHG")
            write_options.put("partB", "SARA")
            write_options.put("partF", "IMPORT")
            imp_builder.writeOptions(write_options)

            imp = imp_builder.build()

            msg = "Starting Vortex BatchImporter process for {}".format(os.path.basename(f))
            print(msg); stdout_msgs.append(msg) # Semicolon for multiple statements is fine

            imp.process()

        msg = "All GRIB files processed sequentially without concurrency."
        print(msg) 
        stdout_msgs.append(msg)

    except Exception as e_process:
        # **** THIS IS THE ADDED except BLOCK ****
        err_msg = "ERROR during GRIB processing in merge_grib_folder: %s\nTraceback:\n%s" % (str(e_process), traceback.format_exc())
        print(err_msg)
        stderr_msgs.append(err_msg)
        # The function will continue and return the messages collected so far, including the error.

    # This return statement is now outside the try-except specific to processing,
    # or it could be inside both try and except if you want to ensure it's always hit last
    # For this structure, having it after the try-except is cleaner.
    return {"stdout": "\n".join(stdout_msgs),
            "stderr": "\n".join(stderr_msgs)}

# Main execution
if __name__ == "__main__":
    # ... (keep your main execution block as is) ...
    if len(sys.argv) < 2:
        print("Usage: jython %s <input_folder> [shapefile_path] [output_dss] [variables]" % sys.argv[0])
        print("Example: jython %s /path/to/grib/folder" % sys.argv[0])
        sys.exit(1)
    
    input_folder = sys.argv[1]
    
    shapefile_path = sys.argv[2] if len(sys.argv) > 2 else None
    output_dss = sys.argv[3] if len(sys.argv) > 3 else None
    variables_str = sys.argv[4] if len(sys.argv) > 4 else None
    variables = variables_str.split(',') if variables_str else None # Ensure variables_str is not None before split
    
    print("Processing GRIB files from folder: %s" % input_folder)
    
    try:
        result = merge_grib_folder(input_folder, shapefile_path, output_dss, variables)
        # Check stderr from the result to determine if there was a processing error
        if result.get("stderr"):
            print("--- Stderr Output ---")
            print(result["stderr"])
            # Optionally, exit with an error code if stderr is not empty
            # sys.exit(1) # Uncomment if any stderr should indicate script failure
        print("Merge process completed.") # Changed from "successfully" to allow for errors in stderr
        if not result.get("stderr"):
             print("Merge process completed successfully.")

    except ValueError as ve: # Catch specific known errors like path not found
        print("Configuration ERROR: %s" % str(ve))
        sys.exit(1)
    except Exception as e: # Catch any other unexpected errors during setup or call
        print("Unhandled ERROR during merge_grib_folder call: %s\nTraceback:\n%s" % (str(e), traceback.format_exc()))
        sys.exit(1)