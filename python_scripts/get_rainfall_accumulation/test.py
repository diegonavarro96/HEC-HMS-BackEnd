from pathlib import Path
import xarray as xr, pprint

# --- build an absolute path ---
repo_root = Path(__file__).resolve().parents[1]        # HMSBackend/
grib_path = repo_root / ".." / "gribFiles" / "test.grib2"     # HMSBackend/gribFiles/test.grib2

if not grib_path.exists():
    raise FileNotFoundError(f"{grib_path} does not exist")

# --- optional: tell cfgrib where to store its index ---
index_dir = repo_root / "gribFiles" / ".idx"           # keep idx files out of the way
index_dir.mkdir(parents=True, exist_ok=True)

ds = xr.open_dataset(
    grib_path,
    engine="cfgrib",
    backend_kwargs={"indexpath": str(index_dir / (grib_path.name + ".idx"))},
)

print("\nVARIABLES FOUND:", list(ds.data_vars))
if "unknown" in ds:
    print("\nATTRS FOR 'unknown':")
    pprint.pprint(ds["unknown"].attrs)