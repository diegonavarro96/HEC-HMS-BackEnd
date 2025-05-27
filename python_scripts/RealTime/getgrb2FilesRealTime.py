# getgrb2Files.py – store everything under **local‑date folder**
"""
Downloads MRMS 1‑hour QPE data.

**What’s new (local‑date folder)**
----------------------------------
All files saved during a single run now land in **one directory named after
your computer’s local calendar date** (e.g., `20250526`), rather than separate
folders by UTC date.  This makes it easier to scoop every product created
while you’re on the same “day” locally.

Nothing else changed for callers: the public function
`download_grib_files()` keeps its original signature.
"""
from __future__ import annotations

import argparse
import gzip
import logging
import os
import re
import shutil
import sys
from datetime import datetime, timedelta, timezone
from typing import List, Optional
from urllib.parse import urljoin

import requests
import yaml
from bs4 import BeautifulSoup

# ── Logging ───────────────────────────────────────────────────────────────────
logger = logging.getLogger(__name__)
if not logger.hasHandlers():
    logging.basicConfig(
        level=logging.INFO,
        format="%(asctime)s [%(levelname)s] %(name)s – %(message)s",
        datefmt="%Y-%m-%d %H:%M:%S",
    )

# ── Paths & Config helpers ────────────────────────────────────────────────────
SCRIPT_DIR = os.path.dirname(__file__) if "__file__" in globals() else os.getcwd()
# Load config from the same directory as the script
DEFAULT_CONFIG_PATH = os.path.abspath(os.path.join(SCRIPT_DIR, "config.yaml"))
PROJECT_ROOT_DIR = os.path.abspath(os.path.join(SCRIPT_DIR, ".."))


def load_config(config_path: Optional[str] = None) -> dict:
    path = config_path or DEFAULT_CONFIG_PATH
    if not os.path.exists(path):
        raise FileNotFoundError(f"Configuration file not found at {path}")
    with open(path, "r", encoding="utf-8") as f:
        return yaml.safe_load(f) or {}


def get_full_data_path(cfg: dict, subkey: str) -> str:
    rel = cfg.get("data_paths", {}).get(subkey)
    if not isinstance(rel, str):
        raise KeyError(f"data_paths.{subkey} not defined or not a string")
    full = os.path.abspath(os.path.join(PROJECT_ROOT_DIR, rel))
    os.makedirs(full, exist_ok=True)
    return full

# ── Utility helpers ───────────────────────────────────────────────────────────

def _format_base_url(template: str, dt: datetime) -> str:
    """Insert YYYY/MM/DD into `template` via ####/##/## placeholders or strftime."""
    if "%" in template:
        res = dt.strftime(template)
    else:
        y, m, d = dt.strftime("%Y %m %d").split()
        res = template.replace("####", y, 1).replace("##", m, 1).replace("##", d, 1)
    return res.rstrip("/") + "/"

_fname_time_re = re.compile(r"_(\d{8}-\d{6})\.grib2", re.I)


def _parse_timestamp(fname: str) -> Optional[datetime]:
    m = _fname_time_re.search(fname)
    if not m:
        return None
    try:
        return datetime.strptime(m.group(1), "%Y%m%d-%H%M%S").replace(tzinfo=timezone.utc)
    except ValueError:
        return None

# ── Scraping & download helpers ──────────────────────────────────────────────

def _scrape_links(html: str, ext: str, pattern: str = "") -> List[str]:
    soup = BeautifulSoup(html, "html.parser")
    return [a["href"] for a in soup.find_all("a", href=True)
            if a["href"].lower().endswith(ext) and pattern in a["href"]]


def _download_and_extract(url: str, dst: str, sess: requests.Session) -> str | None:
    try:
        with sess.get(url, stream=True, timeout=90) as r:
            r.raise_for_status()
            if "text/html" in r.headers.get("Content-Type", "").lower():
                logger.debug("HTML received – skipped %s", url)
                return None
            with open(dst, "wb") as f:
                shutil.copyfileobj(r.raw, f)
        # auto‑decompress .gz
        if dst.endswith(".gz"):
            out_path = dst[:-3]
            try:
                with gzip.open(dst, "rb") as gz_in, open(out_path, "wb") as f_out:
                    shutil.copyfileobj(gz_in, f_out)
                os.remove(dst)
                dst = out_path
            except Exception as ex:
                logger.error("Decompression failed for %s: %s", dst, ex)
        return dst
    except requests.RequestException as ex:
        logger.error("Download error %s: %s", url, ex)
    except Exception as ex:
        logger.error("Unexpected error %s: %s", url, ex)
    if os.path.exists(dst):
        os.remove(dst)
    return None

# ── Real‑time Pass‑1 download ────────────────────────────────────────────────

def _download_realtime(cfg: dict, hours_back: int, sess: requests.Session, dest_dir: str) -> List[str]:
    live_url = cfg["grib_download"].get("base_url_realtime")
    if not live_url:
        return []
    live_url = live_url.rstrip("/") + "/"
    pattern = str(cfg["grib_download"].get("file_pattern", ""))
    ext = ".gz" if pattern == "" or pattern.endswith("gz") else ".grb2"

    try:
        index = sess.get(live_url, timeout=30)
        index.raise_for_status()
    except requests.RequestException as ex:
        logger.error("Cannot list live directory: %s", ex)
        return []

    links = _scrape_links(index.text, ext, pattern)
    if not links:
        logger.info("Live directory: no matching files")
        return []

    cutoff = datetime.now(timezone.utc) - timedelta(hours=hours_back)
    os.makedirs(dest_dir, exist_ok=True)
    new_files: List[str] = []
    for href in links:
        ts = _parse_timestamp(href)
        if ts and ts < cutoff:
            continue
        dst = os.path.join(dest_dir, os.path.basename(href))
        final_check = dst[:-3] if dst.endswith(".gz") else dst
        if os.path.exists(final_check):
            continue
        saved = _download_and_extract(urljoin(live_url, href), dst, sess)
        if saved:
            new_files.append(saved)
    logger.info("Real‑time: %d new file(s)", len(new_files))
    return new_files

# ── Archive Pass‑2 download ─────────────────────────────────────────────────

def _download_archive(base_dt: datetime, cfg: dict, days_back: int, sess: requests.Session, dest_dir: str) -> List[str]:
    tpl = cfg["grib_download"]["base_url"]
    pattern = str(cfg["grib_download"].get("file_pattern", ""))
    ext = ".gz" if pattern == "" or pattern.endswith("gz") else ".grb2"

    collected: List[str] = []
    os.makedirs(dest_dir, exist_ok=True)
    for d in range(days_back + 1):
        day_dt = base_dt - timedelta(days=d)
        day_url = _format_base_url(tpl, day_dt)
        try:
            idx = sess.get(day_url, timeout=30)
            idx.raise_for_status()
        except requests.RequestException as ex:
            logger.error("Cannot list %s: %s", day_url, ex)
            continue
        links = _scrape_links(idx.text, ext, pattern)
        if not links:
            logger.info("No archive files for %s", day_dt.strftime("%Y‑%m‑%d"))
            continue
        for href in links:
            dst = os.path.join(dest_dir, os.path.basename(href))
            final_check = dst[:-3] if dst.endswith(".gz") else dst
            if os.path.exists(final_check):
                continue
            saved = _download_and_extract(urljoin(day_url, href), dst, sess)
            if saved:
                collected.append(saved)
    logger.info("Archive: %d new file(s)", len(collected))
    return collected

# ── Public API ───────────────────────────────────────────────────────────────

def download_grib_files(date_str: str | None = None, config_path: str | None = None, include_yesterday: bool = True) -> List[str]:
    """Backward‑compatible entry point used by Flask."""
    cfg = load_config(config_path)
    hours_live = int(cfg["grib_download"].get("realtime_hours", 25))
    days_cfg = int(cfg["grib_download"].get("days_back", 2))
    days_back = 0 if not include_yesterday else max(1, days_cfg)

    base_dt = datetime.now(timezone.utc) if date_str is None else datetime.strptime(date_str, "%Y%m%d").replace(tzinfo=timezone.utc)

    # Folder based on *local* date (system tz)
    out_root = get_full_data_path(cfg, "grb_downloads_subdir")
    local_folder = datetime.now().astimezone().strftime("%Y%m%d")
    dest_dir = os.path.join(out_root, local_folder)

    sess = requests.Session()
    sess.headers.update({"User-Agent": "Mozilla/5.0 (FloodACE/5.0)"})

    downloaded: List[str] = []
    downloaded += _download_realtime(cfg, hours_live, sess, dest_dir)
    downloaded += _download_archive(base_dt, cfg, days_back, sess, dest_dir)

    if downloaded:
        logger.info("Saved %d file(s) to %s", len(downloaded), dest_dir)
    else:
        logger.warning("No new files downloaded.")
    return downloaded

# ── CLI helper ──────────────────────────────────────────────────────────────

def _parse_args(argv: List[str]) -> argparse.Namespace:
    p = argparse.ArgumentParser("MRMS downloader – single local‑date folder")
    p.add_argument("date", nargs="?", help="UTC base date YYYYMMDD for archive")
    p.add_argument("--config", "-c", help="Path to YAML config")
    p.add_argument("--today-only", action="store_true", help="Skip archive back days")
    p.add_argument("--hours", "-H", type=int, help="Override realtime_hours window")
    return p.parse_args(argv)


def main(argv: List[str] | None = None) -> None:
    args = _parse_args(argv or sys.argv[1:])
    cfg_path = args.config or None
    try:
        # Override realtime window on the fly if provided
        if args.hours is not None:
            cfg = load_config(cfg_path)
            cfg.setdefault("grib_download", {})["realtime_hours"] = args.hours
            # write back?
            # We won't persist, just inject at runtime by passing cfg dict
            # but download_grib_files expects path – quick workaround: write temp yaml
            from tempfile import NamedTemporaryFile
            with NamedTemporaryFile("w", delete=False, suffix=".yaml") as tmp:
                yaml.safe_dump(cfg, tmp)
                cfg_path = tmp.name
        download_grib_files(args.date, cfg_path, not args.today_only)
    except Exception as ex:
        logger.exception("Fatal error: %s", ex)
        sys.exit(1)


if __name__ == "__main__":
    main()
