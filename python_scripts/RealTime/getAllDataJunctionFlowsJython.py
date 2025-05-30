# run_dss_extraction.py
import subprocess
import os
import pandas as pd # Optional: for easily handling the data afterward
from io import StringIO # Optional: to read string data into pandas

# --- Configuration ---
# !!! IMPORTANT: UPDATE THIS PATH to your HEC-DSSVue installation !!!
#hec_dssvue_executable = r"C:/Program Files/HEC/HEC-DSSVue/HEC-DSSVue.cmd" # Windows example
hec_dssvue_installation_dir = "C:/Program Files/HEC/HEC-DSSVue" # As per your screenshot
hec_jython_runner = os.path.join(hec_dssvue_installation_dir, "Jython.bat")
# hec_dssvue_executable = "/opt/HEC/HEC-DSSVue/hec-dssvue.sh" # Linux/Mac example

# Path to the Jython script we created
jython_script_path = r"D:\FloodaceDocuments\HMS\HMSGit\HEC-HMS-Floodace\scripts\jythonScripts\extract_dss_data.py"# Assumes it's in the same directory

# --- DSS File and Path Details (to be passed to the Jython script) ---
dss_file_to_process = r"D:\FloodaceDocuments\HMS\HMSGit\HEC-HMS-Floodace\hms_models\LeonCreek\RainrealTime.dss" # !!! UPDATE THIS to your DSS file !!!
# dss_file_to_process = "/path/to/your/data.dss" # Linux/Mac example
default_b_part_junction = "CUL-048" # Default value if none provided


def run_hec_dssvue_script(dss_file, b_part, c_part=None, e_part=None):
    """
    Runs the HEC-DSSVue Jython script headlessly.
    Returns the captured standard output and standard error.
    """
    if not os.path.exists(hec_jython_runner):
        print(f"ERROR: HEC-DSSVue executable not found at: {hec_jython_runner}")
        return None, "HEC-DSSVue executable not found"
    if not os.path.exists(jython_script_path):
        print(f"ERROR: Jython script not found at: {jython_script_path}")
        return None, "Jython script not found"
    if not os.path.exists(dss_file):
        print(f"ERROR: DSS file not found at: {dss_file}")
        return None, f"DSS file not found: {dss_file}"

    # Command to execute: HEC-DSSVue -s script_name.py arg1 arg2 arg3 ...
    command = [
        hec_jython_runner,
        "-s", # Flag to run a script
        jython_script_path,
        b_part   # Argument 1
    ]

    print(f"INFO: Executing command: {' '.join(command)}")

    try:
        process = subprocess.Popen(
            command,
            stdout=subprocess.PIPE,   # <-- capture stdout
            stderr=subprocess.PIPE,
            text=True                 # <-- decode bytes to str
        )

        stdout, stderr = process.communicate(timeout=60)  # Add timeout to avoid indefinite hangs


        if process.returncode != 0:
            print(f"ERROR: HEC-DSSVue script execution failed with return code {process.returncode}.")
            print("Standard Error Output from HEC-DSSVue/Jython script:")
            print(process.stderr if process.stderr else "No stderr.")
            print("Standard Output from HEC-DSSVue/Jython script (if any):")
            print(process.stdout if process.stdout else "No stdout.")
            return None, process.stderr

        # print("INFO: HEC-DSSVue script executed successfully.")
        # print("Standard Output (Data from Jython script):")
        # print(process.stdout)

        return stdout, stderr

    except FileNotFoundError:
        print(f"ERROR: Could not find or execute HEC-DSSVue: {hec_jython_runner}")
        return None, "HEC-DSSVue command execution failed (FileNotFoundError)"
    except Exception as e:
        print(f"ERROR: An unexpected error occurred while running HEC-DSSVue: {e}")
        return None, str(e)

def get_dss_data(b_part_junction=None):
    """
    Extract data from DSS file for a specific B part junction.
    If b_part_junction is not provided, uses the default value.
    Returns the output data and any error output.
    """
    b_part = b_part_junction if b_part_junction else default_b_part_junction
    print(f"Starting DSS data extraction for junction: {b_part}...")
    output_data, error_output = run_hec_dssvue_script(
        dss_file_to_process,
        b_part,
        None,  # c_part not used
        None   # e_part not used
    )
    
    return output_data, error_output

if __name__ == "__main__":
    import sys
    
    # Check if b_part_junction is provided as command line argument
    b_part = sys.argv[1] if len(sys.argv) > 1 else default_b_part_junction
    
    print(f"Starting DSS data extraction for junction: {b_part}...")
    output_data, error_output = run_hec_dssvue_script(
        dss_file_to_process,
        b_part,
        None,  # c_part parameter variable not defined in original code
        None   # e_part parameter variable not defined in original code
    )
    print("output data:", output_data)

    print("\nExtraction process finished.")