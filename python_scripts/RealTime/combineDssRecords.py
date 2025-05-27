# -*- coding: utf-8 -*-
# scripts/combineDssRecords.py
# Merges the output DSS from HRRR forecast and Real-time GRB processing into a single DSS file.

import os
import subprocess
import sys
import yaml
import datetime
import logging
import traceback

# --- Logging Setup ---
logger = logging.getLogger(__name__)
if not logger.hasHandlers():
    logging.basicConfig(level=logging.INFO, format='%(levelname)s - %(message)s')

# --- Configuration Loading Block ---
SCRIPT_DIR = os.path.dirname(__file__) if "__file__" in locals() else os.getcwd()
PROJECT_ROOT_DIR = os.path.abspath(os.path.join(SCRIPT_DIR, '..'))
# Load config from the same directory as the script
CONFIG_FILE = os.path.join(SCRIPT_DIR, 'config.yaml')

def load_config(config_path=CONFIG_FILE):
    """Loads the YAML configuration file."""
    if not os.path.exists(config_path):
        logger.error(f"Configuration file not found at {config_path}")
        raise FileNotFoundError(f"Configuration file not found: {config_path}")
    try:
        with open(config_path, 'r', encoding='utf-8') as f:
            logger.info(f"Loading configuration from: {config_path}")
            config = yaml.safe_load(f)
            return config if config is not None else {}
    except Exception as e:
        logger.error(f"Failed to load or parse configuration file {config_path}: {e}")
        raise ValueError(f"Failed to load or parse configuration file {config_path}: {e}")

def get_full_data_path(config, subdir_key, base_dir=PROJECT_ROOT_DIR):
    """Constructs full data path relative to the base_dir."""
    subdir = config.get('data_paths', {}).get(subdir_key)
    if not subdir:
        raise KeyError(f"'data_paths.{subdir_key}' not found in config.")
    full_path = os.path.join(base_dir, subdir)
    return os.path.abspath(full_path)

def resolve_path(config, section_key, path_key, base_dir=PROJECT_ROOT_DIR):
    """Resolves a path defined in a specific config section."""
    section = config.get(section_key, {})
    rel_path = section.get(path_key)
    if not rel_path:
        raise KeyError(f"'{section_key}.{path_key}' not found in config.")
    # Assume path is relative to project root if not absolute
    if not os.path.isabs(rel_path):
        return os.path.abspath(os.path.join(base_dir, rel_path))
    return os.path.abspath(rel_path) # Return as is if already absolute

# --- Global Configuration and Setup ---
try:
    config = load_config()

    # Get paths and settings from config
    install_cfg = config.get('install_paths', {})
    jython_cfg = config.get('jython', {})
    data_cfg = config.get('data_paths', {})
    merge_hrrr_cfg = config.get('merge_hrrr_forecast', {})
    merge_realtime_cfg = config.get('merge_grb', {}) # Assuming this is the realtime merge config key

    # --- Ensure required config sections exist ---
    if not all([install_cfg, jython_cfg, data_cfg, merge_hrrr_cfg, merge_realtime_cfg]):
         raise KeyError("One or more required configuration sections are missing ('install_paths', 'jython', 'data_paths', 'merge_hrrr_forecast', 'merge_grb').")

    # --- Installation paths ---
    hms_executable_path = install_cfg.get('hec_hms')
    jython_path = install_cfg.get('jython')
    if not hms_executable_path or not jython_path:
        raise KeyError("'install_paths.hec_hms' or 'install_paths.jython' missing in config.yaml.")
    hms_executable_path = os.path.abspath(hms_executable_path)
    jython_path = os.path.abspath(jython_path)

    # --- Jython settings ---
    jython_initial_heap_size = jython_cfg.get('initial_heap', "256m")
    jython_maximum_heap_size = jython_cfg.get('max_heap', "8192m")
    jython_exe_name = "jython.exe" if sys.platform == "win32" else "jython"
    jython_exe_path = os.path.join(jython_path, "bin", jython_exe_name)
    if not os.path.exists(jython_exe_path):
         logger.warning(f"Jython executable not found at expected path: {jython_exe_path}. Check 'install_paths.jython' in config.")
         # Consider raising FileNotFoundError if strict checking is needed

    # --- Data paths ---
    # Input DSS files
    hrrr_dss_path = resolve_path(config, 'merge_hrrr_forecast', 'output_dss_path')
    realtime_dss_path = resolve_path(config, 'merge_grb', 'output_dss_path_pass1_pass2')

    # Output DSS file directory (assuming both inputs land in the same merged dir)
    # Using the realtime merge config's output path to determine the directory
    output_dss_dir = os.path.dirname(realtime_dss_path)
    # Ensure the output directory exists
    os.makedirs(output_dss_dir, exist_ok=True)

    # Define the final combined DSS filename
    combined_dss_filename = "RainfallRealTimeAndForcast.dss"
    combined_dss_full_path = os.path.join(output_dss_dir, combined_dss_filename)

    # Temp dir for generated scripts
    temp_files_subdir = data_cfg.get('temp_files_dir')
    if not temp_files_subdir:
         raise KeyError("'data_paths.temp_files_dir' not defined in config.yaml")
    temp_dir = os.path.abspath(os.path.join(PROJECT_ROOT_DIR, temp_files_subdir))
    os.makedirs(temp_dir, exist_ok=True)

    # --- Paths for generated scripts ---
    combine_jython_script_path = os.path.join(temp_dir, "combine_specific_dss.py")
    combine_batch_script_path = os.path.join(temp_dir, "combine_specific_dss.bat")

    # --- Log paths for verification ---
    logger.info(f"[Config] HEC-HMS Path: {hms_executable_path}")
    logger.info(f"[Config] Jython Executable: {jython_exe_path}")
    logger.info(f"[Config] Input HRRR DSS: {hrrr_dss_path}")
    logger.info(f"[Config] Input Realtime DSS: {realtime_dss_path}")
    logger.info(f"[Config] Output Combined DSS Dir: {output_dss_dir}")
    logger.info(f"[Config] Output Combined DSS File: {combined_dss_full_path}")
    logger.info(f"[Config] Temporary Script Dir: {temp_dir}")
    logger.info(f"[Generated] Jython combine script path: {combine_jython_script_path}")
    logger.info(f"[Generated] Batch file path: {combine_batch_script_path}")

    # --- Create the Jython script content ---
    # Use repr() for paths to handle backslashes correctly in the generated script
    combine_jython_content = f'''# -*- coding: utf-8 -*-
# Generated by combineDssRecords.py - DO NOT EDIT MANUALLY
import os
import sys
import traceback
try:
    from hec.heclib.dss import HecDss
    # from hec.script import MessageBox # Avoid MessageBox in automated scripts
except ImportError as e:
    print("ERROR: Cannot import HEC libraries in Jython.")
    print("Ensure HEC-HMS/DSSVue JARs are in CLASSPATH or Jython's site-packages.")
    print("Import Error: {{0}}".format(e))
    sys.exit(1)

# --- Input & Output Paths (from Python wrapper) ---
# Use a list for input files for potential future expansion
input_dss_files = [
    {repr(hrrr_dss_path)},
    {repr(realtime_dss_path)}
]
combined_dss_output = {repr(combined_dss_full_path)}
# --- End Paths ---

print("--- Jython Combine Script Start ---")
print("Input DSS Files:")
for f in input_dss_files: print("- {{0}}".format(f))
print("Output Combined DSS: {{0}}".format(combined_dss_output))

# Ensure output directory exists (Python wrapper should handle this, but double-check)
output_dir = os.path.dirname(combined_dss_output)
if not os.path.exists(output_dir):
    try:
        os.makedirs(output_dir)
        print("Jython: Created output directory: {{0}}".format(output_dir))
    except OSError as e:
        print("ERROR: Failed to create output directory {{0}}: {{1}}".format(output_dir, e))
        sys.exit(1)

# Optional: Delete existing combined file before merging to start fresh
if os.path.exists(combined_dss_output):
    try:
        os.remove(combined_dss_output)
        print("Jython: Removed existing combined file: {{0}}".format(combined_dss_output))
    except OSError as e:
        print("WARNING: Could not remove existing combined file {{0}}: {{1}}".format(combined_dss_output, e))
        # Decide if this is a fatal error or just a warning
        # sys.exit(1)

copied_record_count = 0
processed_files_count = 0
error_occurred = False

try:
    # Open the target DSS file once for writing (implicitly creates it)
    # This is more efficient than opening/closing within the loop
    # Note: HecDss.open() might not return a standard file handle,
    # rely on copyRecordsFrom to manage the target file.

    for input_file in input_dss_files:
        processed_files_count += 1
        print("\\nJython: Processing input file: {{0}}".format(input_file))
        if not os.path.exists(input_file):
            print("ERROR: Input DSS file not found: {{0}}".format(input_file))
            error_occurred = True
            continue # Skip to the next file

        source_dss = None # Ensure defined before try
        try:
            # Open source DSS in read-only mode
            source_dss = HecDss.open(input_file, True)
            if not source_dss:
                 print("ERROR: Failed to open source DSS file: {{0}}".format(input_file))
                 error_occurred = True
                 continue

            pathnames = source_dss.getCatalogedPathnames(True)
            print("Jython: Found {{0}} records in {{1}}".format(len(pathnames), os.path.basename(input_file)))

            if not pathnames:
                 print("Jython: No records found in {{0}}, skipping copy.".format(os.path.basename(input_file)))
                 source_dss.done()
                 continue

            # Copy records from the current source file into the combined output file
            # HecDss.open(combined_dss_output) is implicitly handled by copyRecordsFrom
            # It will create or open the target file as needed.
            result = source_dss.copyRecordsFrom(combined_dss_output, pathnames)

            if result < 0:
                error_occurred = True
                print("ERROR: Failed to copy records from {{0}} (Result: {{1}})".format(input_file, result))
            else:
                # result might not be the count, just status. Increment by pathnames length.
                copied_record_count += len(pathnames)
                print("Jython: Copied records from {{0}} (Result: {{1}})".format(os.path.basename(input_file), result))

        except Exception as file_e:
            error_occurred = True
            error_msg = "Failed processing file {{0}}: {{1}}".format(input_file, ' '.join(file_e.args) if hasattr(file_e, 'args') else str(file_e))
            print("ERROR: {{0}}".format(error_msg))
            traceback.print_exc() # Print stack trace for debugging
        finally:
             if source_dss:
                 source_dss.done() # Close the source DSS file

except Exception as e:
    error_occurred = True
    error_msg = ' '.join(e.args) if hasattr(e, 'args') and e.args else str(e)
    tb_str = traceback.format_exc()
    full_error = "Error during DSS combine process: {{0}}\\nTraceback:\\n{{1}}".format(error_msg, tb_str)
    print("ERROR: {{0}}".format(full_error))
    sys.exit(1) # Exit Jython script with error

print("\\n--- Jython Combine Script End ---")
print("Jython: Finished processing {{0}} input DSS files.".format(processed_files_count))
print("Jython: Approx {{0}} records copied/merged into {{1}}.".format(copied_record_count, combined_dss_output))

if error_occurred:
    print("WARNING: Errors occurred during the combine process. Check logs above.")
    sys.exit(1) # Indicate failure
else:
    # Verify the output file exists and has size > 0 as a basic check
    if os.path.exists(combined_dss_output) and os.path.getsize(combined_dss_output) > 0:
         print("Jython: Combine completed successfully. Output file created: {{0}}".format(combined_dss_output))
         sys.exit(0) # Indicate success
    elif os.path.exists(combined_dss_output):
         print("WARNING: Output file exists but is empty: {{0}}".format(combined_dss_output))
         sys.exit(1) # Indicate potential failure (no records copied?)
    else:
         print("ERROR: Output file was not created: {{0}}".format(combined_dss_output))
         sys.exit(1) # Indicate failure
'''

    # Write the Jython script to the temp directory
    try:
        with open(combine_jython_script_path, 'w', encoding='utf-8') as py_file:
            py_file.write(combine_jython_content)
        logger.info(f"Jython combine script written to {combine_jython_script_path}")
    except IOError as e:
        raise IOError(f"ERROR writing Jython script {combine_jython_script_path}: {e}")

    # --- Create the Batch file content ---
    combine_batch_content = f'''@echo off
REM Generated by combineDssRecords.py - DO NOT EDIT MANUALLY
echo Setting up environment for HEC-HMS/Jython DSS Combine...

set "HMS_HOME={hms_executable_path}"
set "JYTHON_EXE={jython_exe_path}"
set "JYTHON_SCRIPT={combine_jython_script_path}"

REM Validate paths
if not exist "%HMS_HOME%" ( echo ERROR: HEC-HMS home directory not found: "%HMS_HOME%" & exit /b 1 )
if not exist "%JYTHON_EXE%" ( echo ERROR: Jython executable not found: "%JYTHON_EXE%" & exit /b 1 )
if not exist "%JYTHON_SCRIPT%" ( echo ERROR: Jython script not found: "%JYTHON_SCRIPT%" & exit /b 1 )

REM Add HEC-HMS GDAL and Bin to PATH
set "GDAL_BIN_PATH=%HMS_HOME%\\bin\\gdal"
set "HMS_BIN_PATH=%HMS_HOME%\\bin"
set "PATH=%GDAL_BIN_PATH%;%HMS_BIN_PATH%;%PATH%"
echo PATH includes: %GDAL_BIN_PATH%

REM Set GDAL/PROJ environment variables
set "GDAL_DRIVER_PATH=%GDAL_BIN_PATH%\\gdalplugins"
set "GDAL_DATA=%GDAL_BIN_PATH%\\gdal-data"
set "PROJ_LIB=%GDAL_BIN_PATH%\\projlib"

REM Set CLASSPATH to include HEC-HMS JARs
set "HEC_LIBS=%HMS_HOME%\\lib\\hec\\heclib\\*.jar;%HMS_HOME%\\lib\\hec\\*;%HMS_HOME%\\lib\\*"
set "HMS_JAR=%HMS_HOME%\\hms.jar"
set "CLASSPATH=%HMS_JAR%;%HEC_LIBS%"
echo CLASSPATH set (check contents if errors occur)

REM Define Java library path value
set "JAVA_LIB_PATH_VALUE=%HMS_BIN_PATH%;%GDAL_BIN_PATH%"
echo Java Library Path Value: %JAVA_LIB_PATH_VALUE%

echo Running Jython combine script: %JYTHON_SCRIPT%
REM Execute Jython with JVM options
"%JYTHON_EXE%" ^
  -J-Xms{jython_initial_heap_size} ^
  -J-Xmx{jython_maximum_heap_size} ^
  "-J-Djava.library.path=%JAVA_LIB_PATH_VALUE%" ^
  "%JYTHON_SCRIPT%"

REM Capture the exit code from Jython script
set JYTHON_EXIT_CODE=%ERRORLEVEL%

echo Jython script finished with exit code: %JYTHON_EXIT_CODE%
if %JYTHON_EXIT_CODE% neq 0 (
    echo ERROR: Jython script reported an error. Check output above.
    exit /b %JYTHON_EXIT_CODE%
) else (
    echo SUCCESS: Jython script completed successfully.
    exit /b 0
)
'''

    # Write the Batch file to the temp directory
    try:
        with open(combine_batch_script_path, 'w', encoding='utf-8') as bat_file:
            bat_file.write(combine_batch_content)
        logger.info(f"Batch file written to {combine_batch_script_path}")
    except IOError as e:
        raise IOError(f"ERROR writing Batch script {combine_batch_script_path}: {e}")

# --- Catch configuration/setup errors ---
except (KeyError, FileNotFoundError, ValueError, IOError) as e:
    logger.critical(f"FATAL ERROR during initial setup: {e}", exc_info=True)
    # Re-raise so calling scripts know setup failed
    raise
except Exception as e:
    logger.critical(f"An unexpected FATAL error occurred during initial setup: {e}", exc_info=True)
    # Re-raise
    raise

# --- Function to Execute the Batch File ---
def run_combine_dss():
    """
    Executes the DSS combine process by running the generated batch script.
    Assumes config loaded and scripts generated by module-level code.

    Returns:
        bool: True if the batch script ran and reported success (exit code 0), False otherwise.
    """
    logger.info(f"\n--- Running DSS Combine via Batch ({datetime.datetime.now()}) ---")

    # Pre-run checks
    if not os.path.exists(hrrr_dss_path):
         logger.error(f"Input HRRR DSS file not found: {hrrr_dss_path}. Cannot run combine.")
         return False
    if not os.path.exists(realtime_dss_path):
         logger.error(f"Input Realtime DSS file not found: {realtime_dss_path}. Cannot run combine.")
         return False
    if not os.path.exists(combine_batch_script_path):
         logger.error(f"Batch script not found at {combine_batch_script_path}. Setup might have failed.")
         return False

    # Platform specific command execution
    if sys.platform == "win32":
         cmd = ['cmd', '/c', combine_batch_script_path]
    else:
         # Basic non-Windows execution (needs .sh equivalent)
         logger.error("Batch execution currently only supported on Windows.")
         return False

    logger.info(f"Running command: {' '.join(cmd)}")
    success = False
    try:
        process = subprocess.run(
            cmd,
            capture_output=True,
            text=True,
            encoding='utf-8',
            errors='replace',
            check=False # Check manually
        )

        logger.info("--- Batch Script Output ---")
        logger.info(process.stdout if process.stdout else "<No stdout>")
        logger.warning("--- Batch Script Stderr ---")
        logger.warning(process.stderr if process.stderr else "<No stderr>")

        if process.returncode == 0:
            logger.info(f"Batch combine process finished successfully (Exit Code: {process.returncode}).")
            # Check Jython output for explicit success message
            if "Jython: Combine completed successfully." in process.stdout:
                success = True
                logger.info(f"Combined DSS file created/updated: {combined_dss_full_path}")
            elif "WARNING:" in process.stdout or "ERROR:" in process.stdout or "ERROR:" in process.stderr:
                 logger.warning("Batch script exited 0, but potential errors/warnings detected in output.")
                 success = False # Treat warnings/errors in output as failure for safety
            else:
                 logger.info("Batch script exited 0, assuming success based on exit code.")
                 success = True # Assume success if exit code 0 and no explicit errors
        else:
            logger.error(f"Batch combine process failed (Exit Code: {process.returncode}).")
            success = False

    except FileNotFoundError:
        logger.error(f"Cannot execute command. '{cmd[0]}' not found in PATH?")
        success = False
    except Exception as e:
        logger.error(f"An unexpected error occurred during subprocess execution: {e}", exc_info=True)
        success = False

    return success

# --- Main execution block (for running the script directly) ---
if __name__ == "__main__":
    logger.info("\n--- Running combineDssRecords.py directly ---")
    logger.info("This will perform setup and run the combine process once.")

    try:
        # Setup code (config loading, script generation) has already run at module level.
        combine_successful = run_combine_dss()

        if combine_successful:
            logger.info("\nDirect execution: Combine completed successfully.")
            sys.exit(0)
        else:
            logger.error("\nDirect execution: Combine failed.")
            sys.exit(1)

    except Exception as e:
        # Catch any exceptions raised during setup or run_combine_dss
        logger.critical(f"\nFATAL ERROR during direct execution: {e}", exc_info=True)
        sys.exit(1)
