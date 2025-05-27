# scripts/mergeHRRRForecastGrb.py
import os
import sys
import subprocess
import time
import yaml
from typing import Dict, List, Optional
import logging
import datetime

# --- Constants ---
SCRIPT_DIR = os.path.dirname(__file__) if "__file__" in locals() else os.getcwd()
DEFAULT_CONFIG_PATH = os.path.abspath(os.path.join(SCRIPT_DIR, "..", "config.yaml"))
PROJECT_ROOT_DIR = os.path.abspath(os.path.join(SCRIPT_DIR, ".."))
CONFIG_DOWNLOAD_SUBDIR_KEY = "grb_downloads_subdir"
# Specific config key for this script
HRRR_MERGE_CONFIG_KEY = "merge_hrrr_forecast"

# --- Logging Setup ---
logger = logging.getLogger(__name__)
if not logger.hasHandlers():
     logging.basicConfig(level=logging.INFO, format='%(levelname)s - %(message)s')

# --- Date Validation (Fallback) ---
try:
    from getgrb2FilesPerDay import validate_date
except ImportError:
    import re
    logger.warning("Failed to import validate_date from getgrb2FilesPerDay. Using fallback.")
    def validate_date(date_str):
        if not date_str or not re.match(r"^\d{8}$", date_str): return False
        try: import datetime; datetime.datetime.strptime(date_str, "%Y%m%d"); return True
        except ValueError: return False

# --- Config Utilities ---
def load_config(config_path: str) -> dict:
    if not os.path.exists(config_path):
        raise FileNotFoundError(f"Config file not found at {config_path}")
    with open(config_path, "r", encoding="utf-8") as f:
        try:
            config = yaml.safe_load(f)
            return config if config is not None else {}
        except yaml.YAMLError as e:
            raise ValueError(f"Failed to parse config '{config_path}': {e}")

def get_full_data_path(config: dict, key: str) -> str:
    data_paths = config.get("data_paths", {})
    if not isinstance(data_paths, dict):
         raise ValueError(f"'data_paths' in config is not a dictionary/map.")
    sub = data_paths.get(key)
    if not sub:
        raise KeyError(f"'data_paths.{key}' missing in config")
    if not isinstance(sub, str):
        raise ValueError(f"'data_paths.{key}' value must be a string path.")
    return os.path.abspath(os.path.join(PROJECT_ROOT_DIR, sub))

# --- Merge Logic ---
def build_jython_command(
    config: dict, input_folder_list: List[str]
) -> (List[str], Dict[str, str]):
    install = config.get("install_paths", {})
    # Use the specific HRRR merge config key
    merge_cfg = config.get(HRRR_MERGE_CONFIG_KEY, {})
    jython_cfg = config.get("jython", {})

    if not merge_cfg:
        raise KeyError(f"Config section '{HRRR_MERGE_CONFIG_KEY}' not found.")

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

    shp_key = merge_cfg.get("clip_shapefile_name")
    vars_list = merge_cfg.get("variables") # Get variables from merge_hrrr_forecast
    out_dss = merge_cfg.get("output_dss_path")

    if not shp_key or not vars_list or not out_dss:
        raise KeyError(f"{HRRR_MERGE_CONFIG_KEY} config missing: clip_shapefile_name, variables, or output_dss_path")
    if not isinstance(vars_list, list):
        raise ValueError(f"{HRRR_MERGE_CONFIG_KEY}.variables must be a list.")
    
    logger.info(f"Variables for HRRR merge: {vars_list}")

    shp_dir_base = get_full_data_path(config, "shapefiles_subdir")
    shp_path = os.path.join(shp_dir_base, shp_key)
    if not os.path.exists(shp_path): raise FileNotFoundError(f"Shapefile not found: {shp_path}")

    if not os.path.isabs(out_dss):
        out_dss = os.path.abspath(os.path.join(PROJECT_ROOT_DIR, out_dss))
    out_dss_dir = os.path.dirname(out_dss)
    os.makedirs(out_dss_dir, exist_ok=True)

    jython_tpl = r"""
# Jython script for HEC-Vortex Batch Importer (HRRR Forecast)
from mil.army.usace.hec.vortex.io import BatchImporter
from mil.army.usace.hec.vortex.geo import WktFactory
import os, sys, traceback

input_folders = {input_folder_list!r}
shp = {shp_path!r}
dest = {out_dss!r}
vars = {vars_list!r}

print("--- Jython Script Start (HRRR Forecast) ---")
print("Input Folders: {{}}".format(input_folders))
print("Shapefile: {{}}".format(shp))
print("Destination DSS: {{}}".format(dest))
print("Variables: {{}}".format(vars))

if not os.path.exists(shp): sys.exit("ERROR: Shapefile not found: {{}}".format(shp))
if not vars: sys.exit("ERROR: No variables specified for import.")
if not input_folders: sys.exit("ERROR: No input folders provided.")

files = []
try:
    print("Scanning input folders for GRIB files...")
    for folder in input_folders:
        if not os.path.isdir(folder):
            print("WARNING: Input folder not found, skipping: {{}}".format(folder))
            continue
        print("Scanning folder: {{}}".format(folder))
        count_in_folder = 0
        for filename in os.listdir(folder):
            if filename.lower().endswith((".grb2", ".grib2")):
                files.append(os.path.join(folder, filename))
                count_in_folder += 1
        print("Found {{}} GRIB files in folder.".format(count_in_folder))
    
    if not files: sys.exit("ERROR: No GRIB files found in specified folders.")
    print("Total found {{}} GRIB files to process.".format(len(files)))

    imp_builder = BatchImporter.builder()
    imp_builder.inFiles(files)
    imp_builder.variables(vars)
    imp_builder.geoOptions({{
        "pathToShp": shp, "targetCellSize": "1000",
        "targetWkt": WktFactory.shg(), "resamplingMethod": "Nearest Neighbor"
    }})
    imp_builder.destination(dest)
    # Note: dataType:"PER-CUM" might be needed here too, depending on HRRR GRIB structure.
    # For now, let's keep it simple. Add if Vortex struggles with accumulation.
    imp_builder.writeOptions({{"partA":"SHG", "partB":"SARA","partF":"IMPORT"}}) 
    imp = imp_builder.build()
    print("Starting Vortex BatchImporter process (HRRR Forecast)...")
    results = imp.process()
    if not results: print("WARNING: Vortex process() returned no results for HRRR. Check DSS file and logs.")
    else: print("Vortex process() completed for HRRR. Sample results: {{}}".format(results[:5]))
    print("--- Jython Script End (Success - HRRR Forecast) ---")
except Exception as e:
    print("--- Jython Script Error (HRRR Forecast) ---")
    print("ERROR: {{}}".format(e)); traceback.print_exc(); sys.exit(1)
"""
    formatted_folder_list = [p.replace('\\', '/') for p in input_folder_list]
    jython_code = jython_tpl.format(
        input_folder_list=formatted_folder_list,
        shp_path=shp_path.replace('\\', '/'),
        out_dss=out_dss.replace('\\', '/'),
        vars_list=vars_list,
    )
    cmd = [
        jython_exe,
        f"-J-Xms{jython_cfg.get('initial_heap','256m')}",
        f"-J-Xmx{jython_cfg.get('max_heap','16384m')}",
        f"-J-Djava.library.path={native_paths}",
        "-c", jython_code,
    ]
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

def run_vortex_import(cmd: List[str], env: Dict[str, str]) -> subprocess.CompletedProcess:
    logger.info(">>> Attempting to run Jython HRRR merge command...")
    proc = subprocess.run(
        cmd, env=env, capture_output=True, text=True, check=False,
        encoding='utf-8', errors='replace'
    )
    logger.info("--- Jython STDOUT (HRRR) ---")
    logger.info(proc.stdout if proc.stdout else "[No stdout]")
    logger.warning("--- Jython STDERR (HRRR) ---")
    logger.warning(proc.stderr if proc.stderr else "[No stderr]")
    logger.info(f"--- Jython Exit Code (HRRR): {proc.returncode} ---")
    proc.check_returncode()
    return proc

def merge_hrrr_forecast_grb_once(
    target_date_strs: List[str],
    config_path: Optional[str] = None
) -> Dict[str, str]:
    if not target_date_strs:
         raise ValueError("No target dates provided for HRRR merging.")
    for date_str in target_date_strs:
        if not validate_date(date_str):
            raise ValueError(f"Invalid date format: '{date_str}'. Use YYYYMMDD.")

    effective_config_path = config_path or DEFAULT_CONFIG_PATH
    logger.info(f"Loading configuration for HRRR merge from: {effective_config_path}")
    cfg = load_config(effective_config_path)
    
    base_grib_dir = get_full_data_path(cfg, CONFIG_DOWNLOAD_SUBDIR_KEY)
    logger.info(f"Using base GRIB directory for HRRR: {base_grib_dir}")

    input_folder_paths = []
    logger.info(f"Checking HRRR input folders for dates: {target_date_strs}")
    for date_str in target_date_strs:
        input_folder_for_date = os.path.join(base_grib_dir, date_str)
        if os.path.isdir(input_folder_for_date):
            input_folder_paths.append(input_folder_for_date)
            logger.info(f"Confirmed HRRR folder exists: {input_folder_for_date}")
        else:
            # For HRRR, we might not want a fallback to tomorrow, be strict.
            raise FileNotFoundError(f"Input GRIB folder for HRRR date {date_str} not found: {input_folder_for_date}")
            
    if not input_folder_paths:
        raise RuntimeError("Could not identify any valid input folders for HRRR to process.")

    logger.info(f"Attempting HRRR merge using folders: {input_folder_paths}")
    cmd, env = build_jython_command(cfg, input_folder_paths)
    proc = run_vortex_import(cmd, env)
    return {"stdout": proc.stdout, "stderr": proc.stderr}

if __name__ == "__main__":
    # This script will typically run for the current day or a specified day.
    # For simplicity, it defaults to today. Can be expanded with argparse if needed.
    today_date_str = datetime.date.today().strftime("%Y%m%d") # Using local date as per original version
    target_date_args = [today_date_str]

    if len(sys.argv) > 1:
        custom_dates = sys.argv[1].split(',')
        if custom_dates:
            target_date_args = [date.strip() for date in custom_dates if date.strip()]
            logger.info(f"Using user-specified date(s) for HRRR merge: {target_date_args}")
    else:
        logger.info(f"Defaulting to today's date for HRRR merge: {today_date_str}")

    # Ensure logging is configured if the script is run directly
    if not logging.getLogger().hasHandlers(): # Check if root logger has handlers
        logging.basicConfig(level=logging.INFO, format='%(levelname)s: %(message)s')

    try:
        result = merge_hrrr_forecast_grb_once(target_date_strs=target_date_args)
        print("✅ HRRR GRIB merge process completed successfully.")
        if result.get("stderr"):
             stderr_output = result["stderr"].strip()
             if stderr_output:
                 print("--- Stderr Output (HRRR Merge) ---")
                 print(stderr_output)
    except FileNotFoundError as fnf_error:
        print(f"❌ ERROR: Input GRIB folder not found. Details: {fnf_error}")
        logger.error(f"HRRR Merge failed: {fnf_error}", exc_info=True)
        sys.exit(1)
    except (KeyError, ValueError) as config_error:
        print(f"❌ ERROR: Configuration problem for HRRR merge. Details: {config_error}")
        logger.error(f"HRRR Merge failed due to config error: {config_error}", exc_info=True)
        sys.exit(1)
    except Exception as e:
        print(f"❌ ERROR during HRRR GRIB merge: {e}")
        logger.error("HRRR Merge failed:", exc_info=True)
        sys.exit(1)