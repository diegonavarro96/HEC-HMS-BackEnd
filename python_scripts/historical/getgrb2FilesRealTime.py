# getgrb2Files.py

import os
import sys
from datetime import datetime, timezone, timedelta
from typing import List, Optional
from urllib.parse import urljoin
import gzip
import shutil
import logging # Import logging

import requests
import yaml
from bs4 import BeautifulSoup

# --- Logging Setup ---
logger = logging.getLogger(__name__)
if not logger.hasHandlers():
     logging.basicConfig(level=logging.INFO, format='%(levelname)s - %(message)s')

# where config.yaml lives by default
SCRIPT_DIR = os.path.dirname(__file__) if "__file__" in locals() else os.getcwd()
DEFAULT_CONFIG_PATH = os.path.abspath(os.path.join(SCRIPT_DIR, "..", "config.yaml"))
PROJECT_ROOT_DIR = os.path.abspath(os.path.join(SCRIPT_DIR, ".."))


def load_config(config_path: Optional[str] = None) -> dict:
    """Loads the YAML configuration file or raises FileNotFoundError/ValueError."""
    path = config_path or DEFAULT_CONFIG_PATH
    if not os.path.exists(path):
        raise FileNotFoundError(f"Configuration file not found at {path}")
    with open(path, "r", encoding="utf-8") as f:
        try:
            config = yaml.safe_load(f)
            if config is None: return {}
            return config
        except yaml.YAMLError as e:
            raise ValueError(f"Failed to parse configuration file: {e}")


def get_full_data_path(config: dict, subdir_key: str) -> str:
    """Constructs a full data directory under PROJECT_ROOT_DIR or raises KeyError."""
    data_paths = config.get("data_paths", {})
    if not isinstance(data_paths, dict):
        raise ValueError("'data_paths' in config must be a dictionary.")
    subdir = data_paths.get(subdir_key)
    if not subdir:
        raise KeyError(f"'data_paths.{subdir_key}' not defined in config")
    if not isinstance(subdir, str):
        raise ValueError(f"'data_paths.{subdir_key}' must be a string path.")
    full = os.path.abspath(os.path.join(PROJECT_ROOT_DIR, subdir))
    try:
        os.makedirs(full, exist_ok=True)
    except OSError as e:
        raise IOError(f"Failed to create directory {full}: {e}") from e
    return full


def download_files_for_date(
    target_url: str, 
    output_dir: str, 
    file_pattern: str, 
    session: requests.Session
) -> List[str]:
    """
    Downloads GRIB2 files for a specific date URL.
    Returns list of local paths to the downloaded .grib2 files.
    """
    extracted_files = []
    
    logger.info(f"Fetching directory listing from {target_url}...")
    try:
        resp = session.get(target_url, timeout=30)
        resp.raise_for_status()
        logger.info(f"Successfully fetched directory listing (Status: {resp.status_code}).")

        soup = BeautifulSoup(resp.text, "html.parser")
        links_found = soup.find_all("a", href=True)
        logger.info(f"Found {len(links_found)} links on the page. Filtering...")

        files_to_process = []
        for a in links_found:
            href = a["href"]
            # Check if filename ends with .grb2 AND contains the specified pattern
            if href.lower().endswith(".grb2") and file_pattern in href:
                files_to_process.append(href)
                logger.info(f"Found matching file: {href}")

        if not files_to_process:
            logger.warning(f"No .grb2 files found matching pattern '{file_pattern}' at {target_url}")
            return [] # Return empty list, not an error if simply no files for that day yet

        logger.info(f"Found {len(files_to_process)} .grb2 files matching pattern. Processing...")

        for filename in files_to_process:
            local_path = os.path.join(output_dir, filename)

            # Skip if file already exists
            if os.path.exists(local_path):
                logger.info(f"Skipping download, file already exists: {local_path}")
                extracted_files.append(local_path)
                continue

            file_url = urljoin(target_url, filename)
            logger.info(f"Downloading: {file_url} -> {local_path}")

            try:
                with session.get(file_url, stream=True, timeout=60) as fr:
                    fr.raise_for_status()
                    # Double check it's not an HTML error page
                    if "text/html" in fr.headers.get("Content-Type", "").lower():
                        logger.warning(f"Skipping {filename}, received HTML instead of GRIB data.")
                        continue

                    # Download the file
                    with open(local_path, "wb") as f_out:
                        shutil.copyfileobj(fr.raw, f_out)
                    logger.info(f"Successfully downloaded: {local_path}")
                    extracted_files.append(local_path)

            except requests.exceptions.RequestException as e_req:
                logger.error(f"ERROR: Failed to download {file_url}: {e_req}")
                # Clean up potentially partially downloaded file
                if os.path.exists(local_path): os.remove(local_path)
            except Exception as e_dl:
                 logger.error(f"ERROR: Unexpected error during download/processing for {filename}: {e_dl}")
                 if os.path.exists(local_path): os.remove(local_path)
    
    except requests.exceptions.RequestException as e:
        logger.error(f"Failed to fetch URL {target_url}: {e}")
    except Exception as e:
        logger.error(f"An unexpected error occurred processing {target_url}: {e}", exc_info=True)
    
    return extracted_files


def download_grib_files(
    date_str: Optional[str] = None, config_path: Optional[str] = None, 
    include_yesterday: bool = True
) -> List[str]:
    """
    Downloads GRIB2 files for a given date (YYYYMMDD) and optionally yesterday's date
    from the NOAA URL. All files are stored in a folder with today's date.
    Returns list of local paths to the downloaded .grib2 files.
    Raises RuntimeError if no files found or on failure.
    """
    logger.info(f"Starting GRIB file download process...")
    try:
        cfg = load_config(config_path)
    except (FileNotFoundError, ValueError) as e:
        logger.error(f"Failed to load config: {e}")
        raise

    # Figure out date (default to today UTC)
    if date_str is None:
        now = datetime.now(timezone.utc)
        date_str = now.strftime("%Y%m%d")
        logger.info(f"No date provided, using current UTC date: {date_str}")
    else:
        # Basic validation
        try:
            datetime.strptime(date_str, "%Y%m%d")
            logger.info(f"Using provided date: {date_str}")
        except ValueError:
            raise ValueError(f"Invalid date format '{date_str}'. Use YYYYMMDD.")

    # Calculate yesterday's date if needed
    yesterday_date_str = None
    if include_yesterday:
        try:
            date_obj = datetime.strptime(date_str, "%Y%m%d")
            yesterday = date_obj - timedelta(days=1)
            yesterday_date_str = yesterday.strftime("%Y%m%d")
            logger.info(f"Will also download yesterday's data: {yesterday_date_str}")
        except Exception as e:
            logger.error(f"Failed to calculate yesterday's date: {e}")
            yesterday_date_str = None

    # Get URL config from the main grib_download section
    grib_cfg = cfg.get("grib_download", {})
    base_url = grib_cfg.get("base_url")
    
    # Get the file pattern to filter GRIB files
    file_pattern = grib_cfg.get("file_pattern", "")
    logger.info(f"Using file pattern filter: '{file_pattern}'")

    if not base_url:
        raise ValueError("Missing 'grib_download.base_url' in config")

    # Prepare output directory using today's date (regardless of what days we download)
    try:
        grb_base = get_full_data_path(cfg, "grb_downloads_subdir")
        output_dir = os.path.join(grb_base, date_str)
        os.makedirs(output_dir, exist_ok=True)
        logger.info(f"Output directory for all files: {output_dir}")
    except (KeyError, ValueError, IOError) as e:
        logger.error(f"Failed to setup output directory: {e}")
        raise

    extracted_files = []
    
    try:
        # Create a session for better performance with multiple requests
        session = requests.Session()
        session.headers.update({
            "User-Agent": "Mozilla/5.0 (compatible; FloodACE-Script/1.0)",
            "Accept": "*/*"
        })

        # Download today's data
        today_url = urljoin(base_url, f"pcpanl.{date_str}/")
        logger.info(f"Processing today's data from URL: {today_url}")
        today_files = download_files_for_date(today_url, output_dir, file_pattern, session)
        extracted_files.extend(today_files)
        
        # Download yesterday's data if requested
        if yesterday_date_str:
            yesterday_url = urljoin(base_url, f"pcpanl.{yesterday_date_str}/")
            logger.info(f"Processing yesterday's data from URL: {yesterday_url}")
            yesterday_files = download_files_for_date(yesterday_url, output_dir, file_pattern, session)
            extracted_files.extend(yesterday_files)

    except Exception as e:
        logger.error(f"An unexpected error occurred during GRIB download: {e}", exc_info=True)
        raise RuntimeError(f"Unexpected error: {e}") from e

    if not extracted_files:
        logger.warning(f"Completed process, but no files were successfully downloaded.")
    else:
         logger.info(f"Successfully processed {len(extracted_files)} files in total.")

    return extracted_files


# Allow standalone run (Updated logging)
if __name__ == "__main__":
    # Check for a flag to disable yesterday's data
    skip_yesterday = "--today-only" in sys.argv
    if skip_yesterday:
        logger.info("Flag --today-only detected, skipping yesterday's data")
        sys.argv.remove("--today-only")
    
    target_date = sys.argv[1] if len(sys.argv) > 1 else None
    # Setup basic logging for standalone run
    logging.basicConfig(level=logging.INFO, format='%(asctime)s - %(levelname)s - %(message)s')

    try:
        files = download_grib_files(date_str=target_date, include_yesterday=not skip_yesterday)
        if files:
            # Get the directory from the first file path
            output_directory = os.path.dirname(files[0])
            logger.info(f"✅ Downloaded/extracted {len(files)} files into {output_directory}")
        else:
            logger.info(f"✅ Process completed, but no new files were downloaded/extracted.")
    except Exception as e:
        logger.error(f"❌ ERROR during GRIB file download: {e}")
        sys.exit(1)
