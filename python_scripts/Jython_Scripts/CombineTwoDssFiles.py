# -*- coding: utf-8 -*-
# combine_two_dss.py  – run with jython
# Usage:
#   jython combine_two_dss.py <source1.dss> <source2.dss> <dest.dss>
#
# Merges every record from the two source files into <dest.dss>.
# Exit code: 0 = success, 1 = failure.

import sys, os, traceback
from hec.heclib.dss import HecDss     # heclib.jar must be on the classpath

# ----------------------------------------------------------------------
# 1.  Parse & validate command-line arguments
# ----------------------------------------------------------------------
if len(sys.argv) != 4:
    print(
        "USAGE: jython combine_two_dss.py <source1.dss> <source2.dss> <dest.dss>"
    )
    sys.exit(1)

src1, src2, dest = sys.argv[1], sys.argv[2], os.path.abspath(sys.argv[3])

for f in (src1, src2):
    if not os.path.exists(f):
        print("ERROR: File not found ->", f)
        sys.exit(1)

# Start fresh if the destination already exists
if os.path.exists(dest):
    try:
        os.remove(dest)
    except OSError as e:
        print("ERROR: Cannot overwrite", dest, ":", e)
        sys.exit(1)

# ----------------------------------------------------------------------
# 2.  Helper to copy every pathname from one DSS into another
# ----------------------------------------------------------------------
def copy_all_records(source_path, target_path):
    """Copy every pathname in source_path into target_path."""
    src = None
    try:
        src = HecDss.open(source_path, True)          # read-only
        paths = src.getCatalogedPathnames(True)
        if not paths:
            print("WARNING: no records found in", source_path)
            return 0
        status = src.copyRecordsFrom(target_path, paths)
        return len(paths) if status >= 0 else -1
    finally:
        if src:
            src.done()

# ----------------------------------------------------------------------
# 3.  Merge loop
# ----------------------------------------------------------------------
try:
    total = 0
    for s in (src1, src2):
        print("Copying from", os.path.basename(s))
        n = copy_all_records(s, dest)
        if n < 0:
            raise RuntimeError("copyRecordsFrom() failed for " + s)
        total += n

    # Basic sanity check
    if os.path.exists(dest) and os.path.getsize(dest) > 0:
        print("\nSUCCESS: merged", total, "records into", dest)
        sys.exit(0)
    else:
        raise RuntimeError("Output file missing or empty")

except Exception as exc:
    print("\nERROR during merge:", exc)
    traceback.print_exc()
    sys.exit(1)
