# mergeGrbFiles.py
import os
import sys
import subprocess
import time
import yaml
from typing import Dict, List, Optional # Ensure this import is present
import logging # Use logging module
import datetime # Import datetime to get the current date

# Import the date validation function if needed
try:
    from getgrb2FilesPerDay import validate_date
except ImportError:
    import re
    print("Warning: Failed to import validate_date from getgrb2FilesPerDay. Using fallback.")
    def validate_date(date_str):
        if not date_str or not re.match(r"^\d{8}$", date_str): return False
        try: import datetime; datetime.datetime.strptime(date_str, "%Y%m%d"); return True
        except ValueError: return False

# --- Constants ---
SCRIPT_DIR = os.path.dirname(__file__) if "__file__" in locals() else os.getcwd()
# Load config from the same directory as the script
DEFAULT_CONFIG_PATH = os.path.abspath(os.path.join(SCRIPT_DIR, "config.yaml"))
PROJECT_ROOT_DIR = os.path.abspath(os.path.join(SCRIPT_DIR, ".."))
CONFIG_DOWNLOAD_SUBDIR_KEY = "grb_downloads_subdir"

# --- Logging Setup ---
logger = logging.getLogger(__name__)
if not logger.hasHandlers():
     logging.basicConfig(level=logging.INFO, format='%(levelname)s - %(message)s')


# --- Config Utilities (Keep as before) ---
def load_config(config_path: str) -> dict:
    """Load and return YAML config, or raise."""
    if not os.path.exists(config_path):
        raise FileNotFoundError(f"Config file not found at {config_path}")
    with open(config_path, "r", encoding="utf-8") as f:
        try:
            config = yaml.safe_load(f)
            if config is None: return {}
            return config
        except yaml.YAMLError as e:
            raise ValueError(f"Failed to parse config '{config_path}': {e}")

def get_full_data_path(config: dict, key: str) -> str:
    """Return absolute path for data_paths[key], relative to PROJECT_ROOT_DIR, or raise."""
    data_paths = config.get("data_paths", {})
    if not isinstance(data_paths, dict):
         raise ValueError(f"'data_paths' in config is not a dictionary/map.")
    sub = data_paths.get(key)
    if not sub:
        raise KeyError(f"'data_paths.{key}' missing in config")
    if not isinstance(sub, str):
        raise ValueError(f"'data_paths.{key}' value must be a string path.")
    full = os.path.abspath(os.path.join(PROJECT_ROOT_DIR, sub))
    return full

# --- Merge Logic ---

# MODIFIED build_jython_command: Takes list of input FOLDERS
def build_jython_command(
    config: dict, input_folder_list: List[str] # Changed back to list of folders
) -> (List[str], Dict[str, str]):
    """
    Prepare the Jython command list and environment for a single merge run
    using a provided list of input folders.
    Returns (command_args, env_dict).
    """
    # --- Get config sections ---
    install = config.get("install_paths", {})
    merge_cfg = config.get("merge_grb", {}) # Focus on merge_grb for this script
    jython_cfg = config.get("jython", {})

    if not merge_cfg:
        raise KeyError("Config section 'merge_grb' not found.")

    # --- Validate essential paths ---
    jython_root = install.get("jython")
    vortex_home = install.get("vortex")
    if not jython_root or not vortex_home: raise KeyError("Missing install_paths.jython or install_paths.vortex")
    if not os.path.isdir(jython_root): raise FileNotFoundError(f"Jython root not found: {jython_root}")
    if not os.path.isdir(vortex_home): raise FileNotFoundError(f"Vortex home not found: {vortex_home}")
    jython_bin = "jython.exe" if sys.platform.startswith("win32") else "jython"
    jython_exe = os.path.join(jython_root, "bin", jython_bin)
    if not os.path.exists(jython_exe): raise FileNotFoundError(f"Jython executable not found: {jython_exe}")
    bin_folder = os.path.join(vortex_home, "bin")
    gdal_folder = os.path.join(bin_folder, "gdal")
    if not os.path.isdir(bin_folder): raise FileNotFoundError(f"Vortex bin not found: {bin_folder}")
    if not os.path.isdir(gdal_folder): raise FileNotFoundError(f"Vortex gdal not found: {gdal_folder}")
    native_paths = os.pathsep.join([bin_folder, gdal_folder])

    # --- Validate merge parameters ---
    shp_key = merge_cfg.get("clip_shapefile_name")
    vars_list = merge_cfg.get("variablesv4") # Use only variablesv4 for this script
    out_dss = merge_cfg.get("output_dss_path_pass2")

    if not shp_key or not vars_list or not out_dss:
        raise KeyError("merge_grb config missing: clip_shapefile_name, variablesv4, or output_dss_path_pass2")
    if not isinstance(vars_list, list):
        raise ValueError("merge_grb.variablesv4 must be a list.")
    
    logger.info(f"Variables for merge (real-time): {vars_list}")

    # --- Validate Shapefile Path ---
    shp_dir_base = get_full_data_path(config, "shapefiles_subdir")
    shp_path = os.path.join(shp_dir_base, shp_key)
    if not os.path.exists(shp_path): raise FileNotFoundError(f"Shapefile not found: {shp_path}")

    # --- Resolve and prepare output DSS path ---
    if not os.path.isabs(out_dss):
        out_dss = os.path.abspath(os.path.join(PROJECT_ROOT_DIR, out_dss))
    out_dss_dir = os.path.dirname(out_dss)
    try:
        os.makedirs(out_dss_dir, exist_ok=True)
    except OSError as e:
         raise IOError(f"Failed to create output DSS directory {out_dss_dir}: {e}") from e

    # --- MODIFIED jython_tpl: Scans list of folders internally ---
    jython_tpl = r"""
# Jython script for HEC-Vortex Batch Importer
# Expects variables: input_folders (list), shp_path, out_dss, vars_list

from mil.army.usace.hec.vortex.io import BatchImporter
from mil.army.usace.hec.vortex.geo import WktFactory
import os, sys
import traceback
from java.util import ArrayList # Modified import

# --- Input Parameters (Set by Python wrapper) ---
input_folders = {input_folder_list!r} # Use the passed list of folders
shp = {shp_path!r}
dest = {out_dss!r}
vars = {vars_list!r}
# --- End Input Parameters ---

print("--- Jython Script Start ---")
print("Input Folders: {{}}".format(input_folders))
print("Shapefile: {{}}".format(shp))
print("Destination DSS: {{}}".format(dest))
print("Variables: {{}}".format(vars))

# Basic validation inside Jython
if not os.path.exists(shp):
    print("ERROR: Shapefile not found: {{}}".format(shp))
    sys.exit(1)
if not vars:
     print("ERROR: No variables specified for import.")
     sys.exit(1)
if not input_folders:
     print("ERROR: No input folders provided to scan.")
     sys.exit(1)

files = [] # Initialize empty list for GRIB files
try:
    print("Scanning input folders for GRIB files...")
    # Loop through the provided folders and collect files
    for folder in input_folders:
        if not os.path.isdir(folder):
            print("WARNING: Input folder not found or not a directory, skipping: {{}}".format(folder))
            continue # Skip this folder
        print("Scanning folder: {{}}".format(folder))
        count_in_folder = 0
        for filename in os.listdir(folder):
            if "01H_Pass2" in filename and filename.lower().endswith((".grb2", ".grib2")):
                files.append(os.path.join(folder, filename))
                count_in_folder += 1
        print("Found {{}} GRIB files in folder.".format(count_in_folder))

    # Check if any files were found across all folders
    if not files:
        print("ERROR: No GRIB files (.grb2, .grib2) found in any of the specified input folders.")
        sys.exit(1)

    print("Total found {{}} GRIB files to process.".format(len(files)))

    # Build the importer
    imp_builder = BatchImporter.builder()

    # Convert Python list to a new Java ArrayList (copy)
    # files is the Python list of file paths collected above
    java_files_copy = ArrayList()
    for f_path in files:
        java_files_copy.add(f_path)

    imp_builder.inFiles(java_files_copy) # Pass the copied Java ArrayList
    imp_builder.variables(vars)
    imp_builder.geoOptions({{
        "pathToShp": shp,
        "targetCellSize": "1000",
        "targetWkt": WktFactory.shg(),
        "resamplingMethod": "Nearest Neighbor"
    }})
    
    imp_builder.destination(dest)
    imp_builder.writeOptions({{"partA":"SHG", "partB":"SARA","partF":"IMPORT", "dataType":"PER-CUM"}})

    imp = imp_builder.build()

    # Run the import process
    print("Starting Vortex BatchImporter process...")
    results = imp.process()

    if not results:
         print("WARNING: Vortex process() returned no results. Check DSS file and logs.")
    else:
         print("Vortex process() completed. Sample results: {{}}".format(results[:5]))
    
    print("--- Jython Script End (Success) ---")

except Exception as e:
    print("--- Jython Script Error ---")
    print(traceback.format_exc()) # Print full traceback for Jython errors
    sys.exit(1) # Ensure script exits with error code on exception

"""
    # --- END MODIFIED jython_tpl ---

    # Format the template with current values
    # Pass the input_folder_list to the template
    # Ensure paths in the list use forward slashes for Jython
    formatted_folder_list = [p.replace('\\', '/') for p in input_folder_list]
    jython_code = jython_tpl.format(
        input_folder_list=formatted_folder_list, # Pass the folder list
        shp_path=shp_path.replace('\\', '/'),
        out_dss=out_dss.replace('\\', '/'),
        vars_list=vars_list, # Use the specific vars_list for this script
    )

    # Build the command list for subprocess
    cmd = [
        jython_exe,
        f"-J-Xms{jython_cfg.get('initial_heap','256m')}",
        f"-J-Xmx{jython_cfg.get('max_heap','8192m')}",
        f"-J-Djava.library.path={native_paths}",
        "-J-Djava.util.concurrent.ForkJoinPool.common.parallelism=1",  # Limit parallelism to avoid concurrent issues
        "-c",
        jython_code,
    ]

    # Set up environment variables (Keep as before)
    env = os.environ.copy()
    vortex_lib_dir = os.path.join(vortex_home, "lib")
    vortex_jar = os.path.join(vortex_home, "vortex.jar")
    classpath_entries = [os.path.join(vortex_lib_dir, "*"), vortex_jar]
    existing_classpath = env.get("CLASSPATH")
    if existing_classpath: classpath_entries.append(existing_classpath)
    env.update({
        "VORTEX_HOME": vortex_home,
        "CLASSPATH": os.pathsep.join(classpath_entries),
        "PATH": native_paths + os.pathsep + env.get("PATH",""),
        "LD_LIBRARY_PATH": native_paths + os.pathsep + env.get("LD_LIBRARY_PATH",""),
        "JAVA_LIBRARY_PATH": native_paths,
        "GDAL_DRIVER_PATH": os.path.join(gdal_folder, "gdalplugins"),
        "GDAL_DATA": os.path.join(gdal_folder, "gdal-data"),
        "PROJ_LIB": os.path.join(gdal_folder, "projlib"),
    })

    return cmd, env


# Keep run_vortex_import as before
def run_vortex_import(cmd: List[str], env: Dict[str, str]) -> subprocess.CompletedProcess:
    """Executes the Jython command and returns the CompletedProcess, printing output."""
    try:
        logger.info(">>> Attempting to run Jython merge command...")
        proc = subprocess.run(
            cmd, env=env, capture_output=True, text=True, check=False,
            encoding='utf-8', errors='replace'
        )
        logger.info("--- Jython STDOUT ---")
        logger.info(proc.stdout if proc.stdout else "[No stdout]")
        logger.warning("--- Jython STDERR ---")
        logger.warning(proc.stderr if proc.stderr else "[No stderr]")
        logger.info(f"--- Jython Exit Code: {proc.returncode} ---")
        proc.check_returncode()
        return proc
    except subprocess.CalledProcessError as e:
        logger.error(f"ERROR: Jython process failed with exit code {e.returncode}.")
        # Log stdout/stderr from the failed process for better diagnostics
        if e.stdout: logger.error(f"Failed process STDOUT:\n{e.stdout}")
        if e.stderr: logger.error(f"Failed process STDERR:\n{e.stderr}")
        raise e
    except FileNotFoundError as e:
         logger.error(f"ERROR: Could not find Jython executable or required file: {e}")
         raise e
    except Exception as e:
         logger.error(f"ERROR: Unexpected error running subprocess: {e}", exc_info=True)
         raise e


# --- MODIFIED Function: Takes list of dates, collects FOLDER paths ---
def merge_grb_once(
    target_date_strs: List[str], # Takes list of date strings
    config_path: Optional[str] = None
) -> Dict[str, str]:
    """
    Performs a single GRIB merge run for a list of specific dates.

    Args:
        target_date_strs (List[str]): The list of dates to process, in 'YYYYMMDD' format.
        config_path (Optional[str]): Path to the configuration file.

    Returns:
        Dict[str, str]: Dictionary containing 'stdout' and 'stderr' from the process.
    """
    if not target_date_strs:
         raise ValueError("No target dates provided for merging.")

    # Validate input date formats
    for date_str in target_date_strs:
        if not validate_date(date_str):
            raise ValueError(f"Invalid date format found in list: '{date_str}'. Use YYYYMMDD.")

    # Load configuration
    effective_config_path = config_path or DEFAULT_CONFIG_PATH
    logger.info(f"Loading configuration for merge from: {effective_config_path}")
    try:
        cfg = load_config(effective_config_path)
    except (FileNotFoundError, ValueError) as e:
         logger.error(f"Failed to load or parse config: {e}")
         raise e

    # Determine the base directory
    try:
        base_grib_dir = get_full_data_path(cfg, CONFIG_DOWNLOAD_SUBDIR_KEY)
        logger.info(f"Using base GRIB directory: {base_grib_dir}")
    except KeyError:
         raise KeyError(f"Config file '{effective_config_path}' missing 'data_paths.{CONFIG_DOWNLOAD_SUBDIR_KEY}'")
    except (ValueError, IOError) as e:
         raise ValueError(f"Config error related to '{CONFIG_DOWNLOAD_SUBDIR_KEY}': {e}") from e

    # --- Collect FOLDER paths for specified dates ---
    input_folder_paths = []
    logger.info(f"Checking existence of input folders for dates: {target_date_strs}")
    missing_dates = []
    
    for date_str in target_date_strs:
        input_folder_for_date = os.path.join(base_grib_dir, date_str)
        if os.path.isdir(input_folder_for_date):
            input_folder_paths.append(input_folder_for_date)
            logger.info(f"Confirmed folder exists: {input_folder_for_date}")
        else:
            logger.warning(f"Input GRIB folder for date {date_str} not found at: {input_folder_for_date}")
            missing_dates.append(date_str)
    
    # --- Handle missing dates - try tomorrow's date for each missing date ---
    for date_str in missing_dates:
        try:
            # Parse the date and add one day to get tomorrow
            date_obj = datetime.datetime.strptime(date_str, "%Y%m%d")
            tomorrow = date_obj + datetime.timedelta(days=1)
            tomorrow_str = tomorrow.strftime("%Y%m%d")
            
            # Check if tomorrow's folder exists
            tomorrow_folder = os.path.join(base_grib_dir, tomorrow_str)
            if os.path.isdir(tomorrow_folder):
                logger.info(f"Using tomorrow's date folder instead: {tomorrow_folder}")
                input_folder_paths.append(tomorrow_folder)
            else:
                logger.warning(f"Tomorrow's folder also not found: {tomorrow_folder}")
        except Exception as e:
            logger.error(f"Error calculating tomorrow's date for {date_str}: {e}")
    
    # --- End folder collection ---

    if not input_folder_paths:
        # No valid folders found for any date or its tomorrow
        raise RuntimeError("Could not identify any valid input folders to process for the specified dates or their tomorrow dates.")

    logger.info(f"Attempting merge using folders: {input_folder_paths}")

    # Build and run the command using the list of folder paths
    try:
        # Pass the list of folders to build_jython_command
        cmd, env = build_jython_command(cfg, input_folder_paths)
        proc = run_vortex_import(cmd, env)

        # Post-processing was removed in previous iteration, ensuring it stays removed.
        # logger.info("DSS post-processing step is not included in this script version.")

        # Return stdout/stderr on success
        return {"stdout": proc.stdout, "stderr": proc.stderr}
    except (KeyError, ValueError, FileNotFoundError, IOError, RuntimeError, subprocess.CalledProcessError) as e:
        logger.error(f"Error during merge process for dates {target_date_strs}: {e}", exc_info=True)
        raise # Re-raise the caught exception
    except Exception as e:
         logger.error(f"Unexpected error during merge for dates {target_date_strs}: {e}", exc_info=True)
         raise RuntimeError(f"Unexpected error during merge: {e}") from e


# ——— Standalone entrypoint (Updated for today/tomorrow fallback) ———
if __name__ == "__main__":
    # Get current date and tomorrow's date
    today_date = datetime.date.today()
    tomorrow_date = today_date + datetime.timedelta(days=1)
    
    today_date_str = today_date.strftime("%Y%m%d")
    tomorrow_date_str = tomorrow_date.strftime("%Y%m%d")
    
    print(f"Standalone merge run for date(s): {today_date_str} (with fallback to {tomorrow_date_str})")
    
    # Set up the target dates, starting with today
    target_date_args = [today_date_str]
    
    # Check command-line arguments for custom dates
    if len(sys.argv) > 1:
        # If user provided dates, use those instead
        custom_dates = sys.argv[1].split(',')
        if custom_dates:
            target_date_args = [date.strip() for date in custom_dates if date.strip()]
            print(f"Using user-specified date(s): {target_date_args}")

    logging.basicConfig(level=logging.INFO, format='%(levelname)s: %(message)s')
    try:
        result = merge_grb_once(target_date_strs=target_date_args)
        print("Merge process completed successfully.")
        if result.get("stderr"):
             print("--- Stderr Output ---")
             print(result["stderr"])
    except Exception as e:
        print(f"ERROR during merge: {e}")
        sys.exit(1)
