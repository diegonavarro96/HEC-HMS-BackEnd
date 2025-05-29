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
b_part_junction = "CUL-048" # !!! UPDATE THIS to your specific B Part !!!
c_part_parameter = "FLOW"       # This should be "FLOW" as per your request
e_part_interval = "1HOUR"       # !!! UPDATE THIS to match your data's interval (e.g., "1DAY", "15MIN") !!!


def run_hec_dssvue_script(dss_file, b_part, c_part, e_part):
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

        return process.stdout, process.stderr

    except FileNotFoundError:
        print(f"ERROR: Could not find or execute HEC-DSSVue: {hec_jython_runner}")
        return None, "HEC-DSSVue command execution failed (FileNotFoundError)"
    except Exception as e:
        print(f"ERROR: An unexpected error occurred while running HEC-DSSVue: {e}")
        return None, str(e)

if __name__ == "__main__":
    print("Starting DSS data extraction...")
    output_data, error_output = run_hec_dssvue_script(
        dss_file_to_process,
        b_part_junction,
        c_part_parameter,
        e_part_interval
    )
    print("output data :", output_data)
    if output_data:
        print("\n--- Extracted Data (CSV Format) ---")
        #print(output_data.strip()) # .strip() to remove leading/trailing whitespace

        # Optional: Process the data using pandas
        try:
            # Use StringIO to treat the string output as a file
            data_io = StringIO(output_data)
            df = pd.read_csv(data_io, header=None, names=['DateTime', 'FlowValue'])

            if not df.empty:
                print("\n--- Data in Pandas DataFrame ---")
                print(df.head())
                # print("\nDataFrame Info:")
                # df.info()

                # Example: Convert DateTime column to actual datetime objects
                df['DateTime'] = pd.to_datetime(df['DateTime'])
                # print("\nDataFrame with parsed DateTime:")
                # print(df.head())
                # print("\nFlow statistics:")
                # print(df['FlowValue'].describe())

                # You can now work with the DataFrame (e.g., save to CSV, plot, etc.)
                # df.to_csv("extracted_flow_data.csv", index=False)
                # print("\nINFO: Data saved to extracted_flow_data.csv")
            else:
                print("\nWARNING: No data was parsed into the DataFrame. The output might be empty or not in the expected format.")

        except pd.errors.EmptyDataError:
            print("\nWARNING: No data was returned from the Jython script to parse with pandas.")
        except Exception as e:
            print(f"\nERROR: Could not process data with pandas: {e}")
            print("Raw output was:\n", output_data)

    elif error_output:
        print(f"\n--- Errors Occurred During Extraction ---")
        # Error messages were already printed by run_hec_dssvue_script
    else:
        print("\n--- No data or significant errors reported, but check logs. ---")

    print("\nExtraction process finished.")