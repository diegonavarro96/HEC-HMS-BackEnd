#!/usr/bin/env python3
# download_grib_historical.py   (formerly getgrb2Files.py)

"""
Download & gun-zip MRMS GRIB2 archives for an arbitrary
inclusive date-range supplied on the command line.

Example (called from Go, PowerShell, Bash, etc.):

    python download_grib_historical.py --start 20210701 --end 20210703 \
      --config configHistorical.yaml

The script

1.  Reads download settings from *configHistorical.yaml*.
2.  Walks every day from --start to --end (UTC) inclusive.
3.  Builds the daily URL by injecting YYYY/MM/DD tokens into the
    `base_url` you supply in YAML (use “{YYYY}/{MM}/{DD}” placeholders).
4.  Downloads only files that match `file_pattern`.
5.  If a file ends in “.gz” it is gun-zipped in-place; the original
    “.gz” file is then deleted, leaving the raw GRIB2 file.
6.  Files for each date go in
       <project_root>/<data_paths.grb_downloads_subdir>/<YYYYMMDD>/
"""

import argparse
import gzip
import logging
import os
import shutil
import sys
from datetime import datetime, timedelta, timezone
from typing import List, Optional
from urllib.parse import urljoin

import requests
import yaml
from bs4 import BeautifulSoup

# ─── Logging ──────────────────────────────────────────────────────────────────
logger = logging.getLogger("grib_dl")
if not logger.hasHandlers():
    logging.basicConfig(
        level=logging.INFO, format="%(asctime)s - %(levelname)s - %(message)s"
    )

# ─── Paths & Helpers ──────────────────────────────────────────────────────────
SCRIPT_DIR = os.path.dirname(__file__) if "__file__" in locals() else os.getcwd()
PROJECT_ROOT_DIR = os.path.abspath(os.path.join(SCRIPT_DIR, ".."))
DEFAULT_CFG = os.path.join(PROJECT_ROOT_DIR, "configHistorical.yaml")


def load_config(path: str) -> dict:
    if not os.path.exists(path):
        raise FileNotFoundError(f"Configuration file not found: {path}")
    with open(path, encoding="utf-8") as f:
        data = yaml.safe_load(f) or {}
    if "grib_download" not in data:
        raise ValueError("Missing 'grib_download' section in YAML")
    return data


def ensure_dir(p: str) -> str:
    os.makedirs(p, exist_ok=True)
    return p


def daterange(start: datetime, end: datetime):
    cur = start
    while cur <= end:
        yield cur
        cur += timedelta(days=1)


def gunzip_file(path_gz: str) -> str:
    path_out = path_gz[:-3]  # strip '.gz'
    with gzip.open(path_gz, "rb") as f_in, open(path_out, "wb") as f_out:
        shutil.copyfileobj(f_in, f_out)
    os.remove(path_gz)
    logger.info(f"Decompressed → {path_out}")
    return path_out


# ─── Download Logic ───────────────────────────────────────────────────────────
def download_one_day(
    daily_url: str, out_dir: str, pattern: str, session: requests.Session
) -> List[str]:
    logger.info(f"Scanning {daily_url}")
    r = session.get(daily_url, timeout=30)
    r.raise_for_status()

    soup = BeautifulSoup(r.text, "html.parser")
    candidates = [
        a["href"]
        for a in soup.find_all("a", href=True)
        if pattern in a["href"] and (a["href"].endswith(".grb2") or a["href"].endswith(".gz"))
    ]

    if not candidates:
        logger.warning(f"No files matching '{pattern}' at {daily_url}")
        return []

    downloaded: List[str] = []
    for fname in candidates:
        local_path = os.path.join(out_dir, fname)
        if os.path.exists(local_path.rstrip(".gz")):  # already decompressed?
            logger.info(f"Exists, skipping: {local_path}")
            downloaded.append(local_path.rstrip(".gz"))
            continue

        file_url = urljoin(daily_url, fname)
        logger.info(f"→ GET {file_url}")
        with session.get(file_url, stream=True, timeout=90) as resp:
            resp.raise_for_status()
            ensure_dir(out_dir)
            with open(local_path, "wb") as f_out:
                shutil.copyfileobj(resp.raw, f_out)
        logger.info(f"Saved {local_path}")

        # unzip if needed
        final = gunzip_file(local_path) if local_path.endswith(".gz") else local_path
        downloaded.append(final)

    return downloaded


def main(argv: Optional[List[str]] = None):
    ap = argparse.ArgumentParser(description="Historical GRIB2 downloader")
    ap.add_argument("--start", required=True, help="YYYYMMDD (inclusive)")
    ap.add_argument("--end", required=True, help="YYYYMMDD (inclusive)")
    ap.add_argument("--config", default=DEFAULT_CFG, help="config YAML path")
    args = ap.parse_args(argv)

    try:
        dt_start = datetime.strptime(args.start, "%Y%m%d").replace(tzinfo=timezone.utc)
        dt_end = datetime.strptime(args.end, "%Y%m%d").replace(tzinfo=timezone.utc)
    except ValueError:
        sys.exit("Dates must be in YYYYMMDD format")

    if dt_start > dt_end:
        sys.exit("--start must be on or before --end")

    cfg = load_config(args.config)
    gd = cfg["grib_download"]
    base_url_tmpl: str = gd["base_url"].rstrip("/")  # allow placeholders
    pattern: str = gd.get("file_pattern", "")

    data_root = ensure_dir(
        os.path.join(PROJECT_ROOT_DIR, cfg["data_paths"]["grb_downloads_subdir"])
    )

    session = requests.Session()
    session.headers.update(
        {"User-Agent": "Mozilla/5.0 (FloodACE-Historical/1.0)", "Accept": "*/*"}
    )

    total_files = 0
    for day in daterange(dt_start, dt_end):
        yyyy, mm, dd = day.strftime("%Y"), day.strftime("%m"), day.strftime("%d")
        daily_url = base_url_tmpl.format(YYYY=yyyy, MM=mm, DD=dd)
        out_dir = ensure_dir(os.path.join(data_root, day.strftime("%Y%m%d")))
        files = download_one_day(daily_url, out_dir, pattern, session)
        total_files += len(files)

    logger.info(f"✅ Completed. {total_files} files downloaded for "
                f"{(dt_end - dt_start).days + 1} day(s).")


if __name__ == "__main__":
    main()
