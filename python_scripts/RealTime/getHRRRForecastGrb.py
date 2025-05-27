# scripts/getHRRRForecastGrb.py
import os
import sys
import argparse
import datetime
import time
import requests # Using requests for simplicity
import yaml
import logging

# --- Constants ---
SCRIPT_DIR = os.path.dirname(__file__) if "__file__" in locals() else os.getcwd()
# Load config from the same directory as the script
DEFAULT_CONFIG_PATH = os.path.abspath(os.path.join(SCRIPT_DIR, "config.yaml"))
PROJECT_ROOT_DIR = os.path.abspath(os.path.join(SCRIPT_DIR, ".."))
CONFIG_DOWNLOAD_SUBDIR_KEY = "grb_downloads_subdir" # Key from existing scripts

# --- Logging Setup ---
logger = logging.getLogger(__name__)
if not logger.hasHandlers():
    logging.basicConfig(level=logging.INFO, format='%(asctime)s - %(levelname)s - %(message)s')

# --- Config Utilities (adapted from mergeGrbFiles.py) ---
def load_config(config_path: str) -> dict:
    if not os.path.exists(config_path):
        logger.error(f"Config file not found at {config_path}")
        raise FileNotFoundError(f"Config file not found at {config_path}")
    with open(config_path, "r", encoding="utf-8") as f:
        try:
            config = yaml.safe_load(f)
            if config is None: return {}
            return config
        except yaml.YAMLError as e:
            logger.error(f"Failed to parse config '{config_path}': {e}")
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
    full = os.path.abspath(os.path.join(PROJECT_ROOT_DIR, sub))
    return full

def validate_date_format(date_str: str) -> bool:
    """Validates if the date string is in YYYYMMDD format."""
    if not date_str: return False
    try:
        datetime.datetime.strptime(date_str, "%Y%m%d")
        return True
    except ValueError:
        return False

def validate_run_hour_format(hour_str: str) -> bool:
    """Validates if the run hour string is HH format (00-23)."""
    if not hour_str or not len(hour_str) == 2: return False
    try:
        hour = int(hour_str)
        return 0 <= hour <= 23
    except ValueError:
        return False

def download_file(url: str, local_filepath: str):
    """Downloads a file from a URL to a local path. No retries."""
    try:
        logger.info(f"Attempting to download: {url}")
        response = requests.get(url, stream=True, timeout=60) # Added timeout
        response.raise_for_status()  # Raises an HTTPError for bad responses (4XX or 5XX)
        
        # Check if the response indicates a file not found (though raise_for_status should handle 404)
        # This might be redundant but adds clarity if raise_for_status behavior changes.
        if response.status_code == 404:
             logger.error(f"Download failed: File not found (404) at {url}")
             return False
             
        with open(local_filepath, 'wb') as f:
            for chunk in response.iter_content(chunk_size=8192):
                f.write(chunk)
        logger.info(f"Successfully downloaded to: {local_filepath}")
        return True
    except requests.exceptions.RequestException as e:
        # Log specific error (e.g., connection error, timeout, 404)
        logger.error(f"Download failed for {url}: {e}")
        return False


def download_hrrr_forecast_gribs(target_date: str, run_hour: str, config_path: str = DEFAULT_CONFIG_PATH):
    """
    Downloads HRRR forecast GRIB files for a specific date and run hour.
    Forecast hours 02 through 12 are downloaded.
    """
    if not validate_date_format(target_date):
        logger.error(f"Invalid target_date format: {target_date}. Expected YYYYMMDD.")
        raise ValueError(f"Invalid target_date format: {target_date}. Expected YYYYMMDD.")
    if not validate_run_hour_format(run_hour):
        logger.error(f"Invalid run_hour format: {run_hour}. Expected HH (e.g., 00, 06, 12, 18).")
        raise ValueError(f"Invalid run_hour format: {run_hour}. Expected HH.")

    try:
        cfg = load_config(config_path)
        base_download_dir = get_full_data_path(cfg, CONFIG_DOWNLOAD_SUBDIR_KEY)
    except (FileNotFoundError, KeyError, ValueError) as e:
        logger.error(f"Configuration error: {e}")
        raise

    date_specific_download_dir = os.path.join(base_download_dir, target_date)
    os.makedirs(date_specific_download_dir, exist_ok=True)
    logger.info(f"Ensured download directory exists: {date_specific_download_dir}")

    base_url = f"https://nomads.ncep.noaa.gov/pub/data/nccf/com/hrrr/prod/hrrr.{target_date}/conus/"
    # User requested forecast hours 02 through 24 (inclusive)
    forecast_hours_to_download = range(2, 13) # Changed end from 13 to 25 to include hour 24

    files_downloaded_count = 0
    total_files_to_download = len(list(forecast_hours_to_download))

    for fh_int in forecast_hours_to_download:
        fh_str = f"{fh_int:02d}" # Format as 02, 03, ..., 12
        filename = f"hrrr.t{run_hour}z.wrfsfcf{fh_str}.grib2"
        file_url = f"{base_url}{filename}"
        local_filepath = os.path.join(date_specific_download_dir, filename)

        if os.path.exists(local_filepath):
            logger.info(f"File already exists, skipping: {local_filepath}")
            files_downloaded_count +=1 
            continue

        # Attempt download (no retries in download_file now)
        if download_file(file_url, local_filepath):
            files_downloaded_count += 1
        else:
            # Stop processing further hours if one fails
            logger.error(f"Failed to download {filename}. Stopping download sequence for this run.")
            break # Exit the loop over forecast hours
            
    if files_downloaded_count == total_files_to_download:
        logger.info(f"All {files_downloaded_count} requested HRRR forecast files downloaded successfully for {target_date} t{run_hour}z.")
    else:
        logger.warning(f"Successfully processed {files_downloaded_count} out of {total_files_to_download} HRRR forecast files for {target_date} t{run_hour}z. Check logs for errors.")
    return files_downloaded_count

if __name__ == "__main__":
    parser = argparse.ArgumentParser(description="Download HRRR forecast GRIB files. Defaults to current UTC date and hour if not specified.")
    parser.add_argument("target_date", type=str, nargs='?', default=None,
                        help="Target date in YYYYMMDD format (e.g., 20250508). Defaults to current UTC date.")
    parser.add_argument("run_hour", type=str, nargs='?', default=None, # Changed default to None
                        help="Model run hour in HH format (e.g., 00, 06, 12, 18). Defaults to current UTC hour.")
    parser.add_argument("--config", type=str, default=DEFAULT_CONFIG_PATH,
                        help=f"Path to the config.yaml file (default: {DEFAULT_CONFIG_PATH}).")

    args = parser.parse_args()

    # Determine target_date
    current_target_date = args.target_date
    if current_target_date is None:
        # Use timezone-aware UTC time
        current_target_date = datetime.datetime.now(datetime.timezone.utc).strftime("%Y%m%d")
        logger.info(f"No target_date provided, defaulting to current UTC date: {current_target_date}")

    # Determine run_hour
    current_run_hour = args.run_hour
    if current_run_hour is None:
        # Calculate previous UTC hour
        now_utc = datetime.datetime.now(datetime.timezone.utc)
        previous_hour_utc = now_utc - datetime.timedelta(hours=1)
        current_run_hour = previous_hour_utc.strftime("%H")
        logger.info(f"No run_hour provided, defaulting to previous UTC hour: {current_run_hour}")
    else:
        logger.info(f"Using provided run_hour: {current_run_hour}")


    try:
        download_hrrr_forecast_gribs(current_target_date, current_run_hour, args.config)
        print(f"✅ HRRR GRIB file download process completed for {current_target_date} t{current_run_hour}z.")
    except (ValueError, FileNotFoundError, KeyError) as e:
        print(f"❌ ERROR: {e}")
        sys.exit(1)
    except Exception as e:
        print(f"❌ An unexpected error occurred: {e}")
        logger.error("Unexpected error in main execution:", exc_info=True)
        sys.exit(1)
