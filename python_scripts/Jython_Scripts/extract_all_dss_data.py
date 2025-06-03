# -*- coding: utf-8 -*-
# extract_dss_data.py
# Displayed in DSSVue “Scripts” menu: name=extract_dss_data.py
# Intended to be launched by HEC-DSSVue’s Jython interpreter

import sys, json
from hec.heclib.dss import HecDss
from hec.heclib.util import HecTime

# ---------------------------------------------------------------------------
# ---- user-configurable constants ------------------------------------------
# ---------------------------------------------------------------------------
DSS_FILE_PATH  = r"D:\FloodaceDocuments\HMS\HMSGit\HEC-HMS-Floodace\hms_models\LeonCreek\RainrealTime.dss"
TARGET_C_PART  = "FLOW"          # variable name
TARGET_E_PART  = "1HOUR"         # time-step
RUN_ID_TOKEN   = "RUN:RAINREALTIME"  # what the F-part must contain
OUTPUT_JSON    = r"D:\FloodaceDocuments\HMS\HMSBackend\JSON\output.json"
TIMEZONE_LABEL = "UTC"           # whatever label your consumer expects
UNITS_LABEL    = "cfs"           # adjust if your DSS units differ
# ---------------------------------------------------------------------------

def format_ht(ht_obj):
    """
    Convert a HecTime that is already set to a Python-style
    'DD Mon YYYYTHH:MM' string, e.g. '27 May 2025T03:00'
    """
    return "{:02d} {} {}T{:02d}:{:02d}".format(
        ht_obj.day(),         # 1-31
        ["Jan","Feb","Mar","Apr","May","Jun",
         "Jul","Aug","Sep","Oct","Nov","Dec"][ht_obj.month()-1],
        ht_obj.year(),
        ht_obj.hour(),
        ht_obj.minute()
    )

def main():
    print("Opening DSS file:", DSS_FILE_PATH)
    dss = HecDss.open(DSS_FILE_PATH, True)  # read-only = True
    try:
        catalog = dss.getCatalogedPathnames()
        if not catalog:
            print("No pathnames found - check the DSS file.")
            sys.exit(1)

        # -------------------------------------------------------------------
        # Build a map:  B-part  → list[full pathname]
        # -------------------------------------------------------------------
        bpart_to_paths = {}
        for p in catalog:
            # pathname string is like  "/A/B/C/D/E/F/"
          #  print ("pathname: " , p)
            parts = p.split("/")
            if len(parts) < 7:
                continue  # malformed
            _, a, b, c, d, e, f, _ = parts  # leading/trailing blanks
            if c.upper() != TARGET_C_PART.upper():
                continue
            if e != TARGET_E_PART:          # exact match – change to .upper() if needed
                continue
            if RUN_ID_TOKEN.upper() not in f.upper():
                continue
            bpart_to_paths.setdefault(b, []).append(p)

        if not bpart_to_paths:
            print("No matching FLOW series found.")
            sys.exit(1)

        print("Discovered {} junction(s): {}".format(
            len(bpart_to_paths), ", ".join(sorted(bpart_to_paths))))

        # -------------------------------------------------------------------
        # Extract data for every junction and build JSON structure
        # -------------------------------------------------------------------
        series_array = []
        ht = HecTime()

        for bpart, path_list in sorted(bpart_to_paths.items()):
            path = path_list[0]                       # take first pathname
            tsc  = dss.get(path, True)
            if tsc is None or tsc.numberValues == 0:
                print("WARNING - no data for", path)
                continue

            # ---- collect time series -------------------------------------------------
            data_points = []
            for i in range(tsc.numberValues):
                ht.set(tsc.times[i])
                data_points.append({
                    "time":  format_ht(ht),
                    "value": float(tsc.values[i])
                })

            # ---- compute peak flow ---------------------------------------------------
            max_flow = max(tsc.values) if tsc.numberValues > 0 else None

            # ---- append series entry --------------------------------------------------
            series_array.append({
                "name":     bpart,
                "timezone": TIMEZONE_LABEL,
                "unit":     UNITS_LABEL,
                "max":      float(max_flow) if max_flow is not None else None,
                "data":     data_points
            })

        # -------------------------------------------------------------------
        # Write JSON
        # -------------------------------------------------------------------
        payload = { "series": series_array }
        with open(OUTPUT_JSON, "w") as fp:
            json.dump(payload, fp, indent=4)
        print("JSON written to", OUTPUT_JSON)

    finally:
        dss.close()
        print("Closed DSS file.")

if __name__ == "__main__":
    main()



#//CHI-001/FLOW/01May2025/1Hour/RUN:RainrealTime/

