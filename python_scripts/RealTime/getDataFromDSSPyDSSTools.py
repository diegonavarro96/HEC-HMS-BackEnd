#!/usr/bin/env python3
# getDataFromDSSPyDSSTools.py

import pandas as pd
from typing import Dict
from pydsstools.heclib.dss import HecDss
import os
import pytz

def extract_by_junction(
    dss_file: str,
    junction: str,
    item: str = None,
    start: str = None,
    end: str = None
) -> Dict[str, pd.DataFrame]:
    if not os.path.exists(dss_file):
        print(f"Error: DSS file not found at '{dss_file}'")
        return {}

    print(f"Attempting to open DSS file: {dss_file}")
    try:
        dss = HecDss.Open(dss_file)
    except Exception as e:
        print(f"Error opening DSS file: {e}")
        return {}

    print("DSS file opened.")
    all_paths = dss.getPathnameList("/*/*/*/*/*/*/")
    print(f"Found {len(all_paths)} total paths in the DSS file.")

    target_b = junction.strip().upper()
    target_c = item.strip().upper() if item else None

    selected = []
    for path in all_paths:
        parts = path.split('/')
        if len(parts) != 8:
            continue
        b = parts[2].strip().upper()
        c = parts[3].strip().upper()
        if b == target_b and (not target_c or c == target_c):
            selected.append(path)

    if not selected:
        print(f"Warning: No matching paths found for Part B='{junction}' and Part C='{item}'")
        dss.close()
        return {}

    print(f"Found {len(selected)} matching paths.")

    data: Dict[str, pd.DataFrame] = {}
    utc_timezone = pytz.utc

    for path in selected:
        print(f"Processing path: {path}")
        try:
            tsc = dss.read_ts(path, window=(start, end) if start and end else None)
            print(f"Raw TSC Object: {tsc}")
            print(f"Type of tsc.times: {type(tsc.times)}")
            print(f"Type of tsc.values: {type(tsc.values)}")
            print(f"Length of tsc.times immediately after read: {len(tsc.times) if hasattr(tsc.times, '__len__') else len(tsc.times.keys())}")
            print(f"Length of tsc.values immediately after read: {len(tsc.values)}")

            # Detect if tsc.times is a dict (irregular series) or list-like (regular)
            if isinstance(tsc.times, dict):
                timestamps_raw = list(tsc.times.keys())
            else:
                timestamps_raw = list(tsc.times)

            print(f" • Parsed {len(timestamps_raw)} raw timestamps, {len(tsc.values)} values")

            if len(timestamps_raw) != len(tsc.values):
                print(f" • Warning: Mismatch in lengths: raw timestamps={len(timestamps_raw)}, values={len(tsc.values)}. Skipping.")
                continue

            # Localize timestamps to UTC
            timestamps_utc = [pd.Timestamp(ts, tz='UTC') for ts in timestamps_raw]

            df = pd.DataFrame({'datetime': timestamps_utc, 'value': tsc.values})
            df['units'] = tsc.units
            df['type'] = tsc.type

            data[path] = df
            print(f" • Successfully read {len(df)} records (Units: {tsc.units}, Type: {tsc.type})")

        except Exception as e:
            print(f" • Error reading path '{path}': {e}")
            continue

    dss.close()
    print("DSS file closed.")
    return data

if __name__ == "__main__":
    dss_file_path = r"D:\FloodaceDocuments\HMS\HMSGit\HEC-HMS-Floodace\hms_models\LeonCreek\RainrealTime.dss"
    junction_filter = "COM-001"
    item_filter = "FLOW"
    start_date_filter = None
    end_date_filter = None

    print("\n--- Starting Data Extraction ---")
    print(f"DSS File: {dss_file_path}")
    print(f"Junction (Part B): {junction_filter}")
    print(f"Item (Part C): {item_filter}")
    print(f"Start Date Filter: {start_date_filter or 'None'}")
    print(f"End Date Filter: {end_date_filter or 'None'}")

    results = extract_by_junction(dss_file_path, junction_filter, item_filter, start_date_filter, end_date_filter)

    if results:
        print("\n--- Extraction Results ---")
        for path, df in results.items():
            print(f"\n=== Data for Path: {path} ===")
            print(f" • Number of records: {len(df)}")
            print(f" • Time range: {df['datetime'].min()} to {df['datetime'].max()}")
            print(f" • Units: {df['units'].iloc[0]}, Type: {df['type'].iloc[0]}")
            print(" • Sample:")
            print(df.head().to_string(index=False))
    else:
        print("No data extracted.")
