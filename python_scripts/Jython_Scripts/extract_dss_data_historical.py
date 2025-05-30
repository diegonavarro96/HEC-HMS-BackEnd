# name=extract_dss_data.py
# displayinmenu=true
# displaytouser=true
# displayinselector=true
# extract_dss_data.py
# This script is intended to be run by HEC-DSSVue via command line

import sys
import json
from hec.script import MessageBox # For basic UI feedback if needed, but avoid for headless
from hec.heclib.dss import HecDss      # Core DSS library
from hec.io import TimeSeriesContainer # To hold time series data
from hec.heclib.util import HecTime    # For time conversions

# --- Configuration - These could be passed as arguments from the CPython script ---
# Default values, can be overridden if script is called with arguments
dss_file_path = "D:\\FloodaceDocuments\\HMS\\HMSBackend\\hms_models\\LeonCreek\\RainrealHistorical.dss" # IMPORTANT: Use forward slashes
target_b_part = "CUL-001"     # The specific B part you're interested in
target_c_part = "FLOW"
target_e_part = "1Hour" # Example, adjust to your data's interval
json_file_name = 'D:\\FloodaceDocuments\\HMS\\HMSBackend\\JSON\\outputHistorical.json'

# --- Attempt to get arguments if passed from command line ---
# sys.argv[0] is the script name itself.
# HEC-DSSVue passes script arguments after the '-s scriptname.py' part
if len(sys.argv) > 1:
    target_b_part = sys.argv[1]
    
# Get month and year if provided
month = "May"  # Default month
year = "2025"  # Default year

if len(sys.argv) > 2:
    month = sys.argv[2]
    
if len(sys.argv) > 3:
    year = sys.argv[3]

# --- Output for debugging script arguments ---
print("DSS File Path: " + dss_file_path)
print("Target B Part: " + target_b_part)
print("Target C Part: " + target_c_part)
print("Target E Part: " + target_e_part)
print("Month: " + month)
print("Year: " + year)



try:
    # Open the DSS file
    # Use 'False' for read-only, 'True' for read-write (not needed here)
    # Use 0 for message level (less verbose)
    print("oppening DSS File...")
    print("dss path: ", dss_file_path)
   # name = raw_input()
    dssFile = HecDss.open(dss_file_path,True) # Added more args for robust open
    print("Dss File: ", dssFile)
    #name = raw_input()
    if not dssFile:
        print("ERROR: DSS File not found or could not be opened: " + dss_file_path)
        sys.exit(1)

    # Construct a pathname pattern to find the desired data
    # Using '*' for parts A, D, and F to be more flexible if they are not precisely known
    # or if you want the latest/first available.
    # Pathname pattern: /A/B/C/D/E/F/
    pathname_pattern = "//{}/{}/01{}{}/{}/RUN:RainrealTime/".format(target_b_part, target_c_part, month, year, target_e_part)
    # Using '.*' for D part to match any date, might need refinement
    # print("DEBUG: Using pathname pattern: " + pathname_pattern)

    # Get a list of pathnames matching the pattern
    pathList = dssFile.get(pathname_pattern) # True for sorted

    print("Path List: ", pathList)
    #name = raw_input()

    tsc = dssFile.get(pathname_pattern, True)

    if tsc is None:
        print("WARNING: Could not retrieve TimeSeriesContainer for path: ", pathList)

    if tsc.numberValues == 0:
            print("INFO: No data values in TimeSeriesContainer for path: ",pathList )

    # print("--- DATA FOR PATH: {} ---".format(fullPath))
    # print("Units: {}".format(tsc.units))
    # print("Data Type: {}".format(tsc.type)) # e.g. "PER-AVER"

    # Prepare data for JSON output
    ht = HecTime()
    print ("ht is : ", ht)
    
    # Create the data structure
    data_points = []
    
    for i in range(tsc.numberValues):
        time_val_minutes = tsc.times[i] # Time in minutes from HEC base date
        ht.set(time_val_minutes)
        
        # Format time as "26 May 2025T24:00"
        # Note: HEC uses 24:00 for midnight, we need to handle this
        hour = ht.hour()
        if hour == 0:
            # Convert midnight to 24:00 of previous day
            ht.set(time_val_minutes - 60)  # Go back one hour
            datetime_str = "{}T24:00".format(ht.date())
        else:
            datetime_str = "{}T{:02d}:00".format(ht.date(), hour)
        
        value = tsc.values[i]
        
        # Add to data points
        data_points.append({
            "time": datetime_str,
            "value": value
        })
        
        print("Data is : " + datetime_str + " Value : " + str(value))
    
    # Create the JSON structure
    json_output = {
        "series": [
            {
                "data": data_points,
                "name": target_b_part,
                "timezone": "UTC",
                "unit": "cfs"
            }
        ]
    }
    
    # Write JSON file
    try:
        with open(json_file_name, 'w') as jsonfile:
            # Note: Jython 2.7 json.dump doesn't have indent parameter
            json_str = json.dumps(json_output, indent=4)
            jsonfile.write(json_str)
            
        print ("JSON file '" + json_file_name + "' has been saved successfully.")
    except IOError as e:
        print ("Error writing to JSON file: ", e)



except Exception as e:
    import traceback
    print("ERROR: An exception occurred in the Jython script.")
    print(str(e))
    print(traceback.format_exc())
    sys.exit(1) # Exit with an error code

finally:
    print ("closing file...")
   # name = raw_input()
    if 'dssFile' in locals() and dssFile:
        dssFile.close()
        # print("INFO: DSS File closed.")

# print("INFO: Jython script finished.") # For debugging
