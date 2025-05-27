import os
import sys
import datetime
import logging
from flask import Flask, jsonify, request
from flask_cors import CORS # Import CORS

# --- Add project root to path for imports ---
SCRIPT_DIR = os.path.dirname(__file__) if "__file__" in locals() else os.getcwd()
PROJECT_ROOT_DIR = os.path.abspath(os.path.join(SCRIPT_DIR, ".."))
if PROJECT_ROOT_DIR not in sys.path:
    sys.path.insert(0, PROJECT_ROOT_DIR)

# --- Import functions from our scripts ---
try:
    # Imports are relative to PROJECT_ROOT_DIR (python_scripts/)
    from RealTime.getgrb2FilesRealTime import download_grib_files
    from RealTime.mergeGrbFilesRealTime import merge_grb_once
    from RealTime.mergeGrbFilesRealTimePass2 import merge_grb_once as merge_grb_once_pass2
    from RealTime.runHMSRealTime import run_computations
    from RealTime.setControlFile import update_control_file
    from RealTime.getHRRRForecastGrb import download_hrrr_forecast_gribs
    from RealTime.mergeHRRRForecastGrb import merge_hrrr_forecast_grb_once
    from RealTime.combineDssRecords import run_combine_dss
    from RealTime.combineDssRecordsPass1Pass2 import run_combine_dss_pass1_pass2
    from RealTime.getDataFromDSSJython import get_dss_data
except ImportError as e:
    print(f"ERROR: Failed to import script modules from RealTime package: {e}")
    print("Ensure RealTime directory is a package (contains __init__.py if needed) and PROJECT_ROOT_DIR is in sys.path.")
    sys.exit(1)

# --- Flask App Setup ---
app = Flask(__name__)
CORS(app) # Enable CORS for all origins by default for development

# Setup basic logging for the Flask app
logging.basicConfig(level=logging.INFO, format='%(asctime)s - %(levelname)s - %(message)s')
logger = logging.getLogger(__name__)

# --- Endpoints ---

@app.route('/download_grib', methods=['POST'])
def handle_download_grib():
    """Endpoint to trigger GRIB file download for a specific date or today."""
    try:
        # Get the date from request body if present
        request_data = request.get_json(silent=True) or {}
        date_str = request_data.get('date')  # Expects YYYYMMDD or None for today
        logger.info(f"Received /download_grib request for date: {date_str or 'today'}")
        
        # download_grib_files handles None date_str correctly (uses today)
        downloaded_files = download_grib_files(date_str=date_str)
        logger.info(f"GRIB download completed. Files processed: {len(downloaded_files)}")
        return jsonify({
            "status": "success",
            "message": f"Downloaded/extracted {len(downloaded_files)} files.",
            "files": downloaded_files
        }), 200
    except ValueError as e:
        # Specific handling for value errors (like invalid date format)
        logger.error(f"Value error during GRIB download: {e}", exc_info=True)
        return jsonify({"status": "error", "message": str(e), "error_type": "value_error"}), 400
    except FileNotFoundError as e:
        # Specific handling for file not found errors
        logger.error(f"File not found during GRIB download: {e}", exc_info=True)
        return jsonify({"status": "error", "message": str(e), "error_type": "file_not_found"}), 404
    except Exception as e:
        # General error handling
        logger.error(f"Error during GRIB download: {e}", exc_info=True)
        return jsonify({"status": "error", "message": str(e), "error_type": "server_error"}), 500

@app.route('/merge_grib', methods=['POST'])
def handle_merge_grib():
    """Endpoint to trigger GRIB file merge for specific date(s) or today."""
    try:
        # Get data from request body if present
        request_data = request.get_json(silent=True) or {}
        
        # Expects a JSON body with a 'dates' list ["YYYYMMDD", ...] or defaults to today
        dates_list = request_data.get('dates')
        if not dates_list:
            today_date_str = datetime.date.today().strftime("%Y%m%d")
            dates_list = [today_date_str]
            logger.info(f"No dates provided for /merge_grib, defaulting to today: {today_date_str}")
        else:
            logger.info(f"Received /merge_grib request for dates: {dates_list}")

        result_pass1 = merge_grb_once(target_date_strs=dates_list)
        logger.info(f"GRIB merge (Pass 1) completed for dates: {dates_list}")

        # Call Pass 2 merge
        result_pass2 = merge_grb_once_pass2(target_date_strs=dates_list)
        logger.info(f"GRIB merge (Pass 2) completed for dates: {dates_list}")
        
        return jsonify({
            "status": "success",
            "message": f"Merge process (Pass 1 and Pass 2) completed for dates: {dates_list}.",
            # Optionally include snippets of stdout/stderr if needed, be cautious of size
            # "pass1_stdout_snippet": result_pass1.get("stdout", "")[:200],
            # "pass2_stdout_snippet": result_pass2.get("stdout", "")[:200],
        }), 200
    except ValueError as e:
        # Specific handling for value errors (like invalid date format)
        logger.error(f"Value error during GRIB merge: {e}", exc_info=True)
        return jsonify({"status": "error", "message": str(e), "error_type": "value_error"}), 400
    except FileNotFoundError as e:
        # Specific handling for file not found errors
        logger.error(f"File not found during GRIB merge: {e}", exc_info=True)
        return jsonify({"status": "error", "message": str(e), "error_type": "file_not_found"}), 404
    except Exception as e:
        # General error handling
        logger.error(f"Error during GRIB merge: {e}", exc_info=True)
        return jsonify({"status": "error", "message": str(e), "error_type": "server_error"}), 500

@app.route('/run_hms', methods=['POST'])
def handle_run_hms():
    """Endpoint to trigger the 'RainrealTime' HEC-HMS run."""
    target_run = "RainrealTime"
    logger.info(f"Received /run_hms request, triggering run: '{target_run}'")
    try:
        # Call run_computations, forcing the specific run
        summary = run_computations(run_name_to_execute=target_run)
        logger.info(f"HMS run completed. Attempted: {summary['attempted']}, Succeeded: {summary['succeeded']}, Failed: {summary.get('failed', 0)}")

        if summary.get('failed', 0) > 0:
            # Return success but indicate partial failure
            failure_messages = []
            for failure in summary.get('failures', []):
                failure_messages.append(f"{failure['name']}: {failure['message'][:100]}...")
            
            return jsonify({
                "status": "partial_failure",
                "message": f"HMS run finished, but {summary.get('failed', 0)} of {summary['attempted']} run(s) failed.",
                "failure_details": failure_messages,
                "summary": {k: v for k, v in summary.items() if k != 'failures'}  # Exclude full failure messages to avoid large responses
            }), 207  # Multi-Status
        else:
            return jsonify({
                "status": "success",
                "message": f"Successfully completed HMS run '{target_run}'.",
                "summary": summary
            }), 200
    except ValueError as e:
        # Specific handling for value errors (like missing run name)
        logger.error(f"Value error during HMS run: {e}", exc_info=True)
        return jsonify({"status": "error", "message": str(e), "error_type": "value_error"}), 400
    except FileNotFoundError as e:
        # Specific handling for file not found errors
        logger.error(f"File not found during HMS run: {e}", exc_info=True)
        return jsonify({"status": "error", "message": str(e), "error_type": "file_not_found"}), 404
    except Exception as e:
        # General error handling
        logger.error(f"Error during HMS run: {e}", exc_info=True)
        return jsonify({"status": "error", "message": str(e), "error_type": "server_error"}), 500

@app.route('/update_control', methods=['POST'])
def handle_update_control():
    """Endpoint to update the RainRealTime.control file directly."""
    logger.info("Received /update_control request")
    try:
        # Get the file path from the config or use default
        # Try to find the control file in the expected location
        target_file = r"D:\FloodaceDocuments\HMS\HMSGit\HEC-HMS-Floodace\hms_models\LeonCreek\RainRealTime.control"
        
        # Log the path we're trying to update
        logger.info(f"Attempting to update control file at: {target_file}")
        
        # Call the update_control_file function from setControlFile.py
        success = update_control_file(target_file)
        
        if success:
            logger.info("Successfully updated RainRealTime.control file")
            return jsonify({
                "status": "success",
                "message": "Successfully updated RainRealTime.control file"
            }), 200
        else:
            logger.error("Failed to update RainRealTime.control file")
            return jsonify({
                "status": "error", 
                "message": "Failed to update RainRealTime.control file",
                "error_type": "update_failure"
            }), 500
    except Exception as e:
        logger.error(f"Error updating control file: {e}", exc_info=True)
        return jsonify({
            "status": "error",
            "message": str(e),
            "error_type": "server_error"
        }), 500

@app.route('/download_hrrr_grib', methods=['POST'])
def handle_download_hrrr_grib():
    """Endpoint to trigger HRRR forecast GRIB file download."""
    try:
        request_data = request.get_json(silent=True) or {} # Use silent=True to avoid error if no JSON body

        target_date = request_data.get('target_date')
        run_hour = request_data.get('run_hour')

        # Apply default logic as requested by user
        if not target_date:
             # Default to current LOCAL date if not provided
             target_date = datetime.datetime.now().strftime("%Y%m%d")
             logger.info(f"No target_date provided for /download_hrrr_grib, defaulting to current local date: {target_date}")
        if not run_hour:
             # Default to current UTC hour (rounded down) if not provided
             run_hour = datetime.datetime.utcnow().strftime("%H")
             logger.info(f"No run_hour provided for /download_hrrr_grib, defaulting to current UTC hour (floor): {run_hour}")

        logger.info(f"Received /download_hrrr_grib request for date: {target_date}, run_hour: {run_hour}")

        # Call the download function from getHRRRForecastGrb.py
        files_count = download_hrrr_forecast_gribs(target_date=target_date, run_hour=run_hour)
        logger.info(f"HRRR GRIB download completed. Files processed: {files_count}")
        return jsonify({
            "status": "success",
            "message": f"Processed {files_count} HRRR forecast files for {target_date} t{run_hour}z.",
            "target_date": target_date,
            "run_hour": run_hour
        }), 200
    except ValueError as e:
        logger.error(f"Value error during HRRR GRIB download: {e}", exc_info=True)
        return jsonify({"status": "error", "message": str(e), "error_type": "value_error"}), 400
    except FileNotFoundError as e:
        logger.error(f"File not found during HRRR GRIB download: {e}", exc_info=True)
        return jsonify({"status": "error", "message": str(e), "error_type": "file_not_found"}), 404
    except Exception as e:
        logger.error(f"Error during HRRR GRIB download: {e}", exc_info=True)
        return jsonify({"status": "error", "message": str(e), "error_type": "server_error"}), 500

@app.route('/merge_hrrr_grib', methods=['POST'])
def handle_merge_hrrr_grib():
    """Endpoint to trigger HRRR forecast GRIB file merge."""
    try:
        request_data = request.get_json(silent=True) or {}
        dates_list = request_data.get('dates')

        if not dates_list:
            today_date_str = datetime.date.today().strftime("%Y%m%d") # Use local date for default merge target
            dates_list = [today_date_str]
            logger.info(f"No dates provided for /merge_hrrr_grib, defaulting to today: {today_date_str}")
        else:
            logger.info(f"Received /merge_hrrr_grib request for dates: {dates_list}")

        # Call the merge function from mergeHRRRForecastGrb.py
        result = merge_hrrr_forecast_grb_once(target_date_strs=dates_list)
        logger.info(f"HRRR GRIB merge completed for dates: {dates_list}")
        return jsonify({
            "status": "success",
            "message": f"HRRR merge process completed for dates: {dates_list}.",
            # Optionally include snippets of stdout/stderr
            # "stdout_snippet": result.get("stdout", "")[:200],
            # "stderr_snippet": result.get("stderr", "")[:200]
        }), 200
    except ValueError as e:
        logger.error(f"Value error during HRRR GRIB merge: {e}", exc_info=True)
        return jsonify({"status": "error", "message": str(e), "error_type": "value_error"}), 400
    except FileNotFoundError as e:
        logger.error(f"File not found during HRRR GRIB merge: {e}", exc_info=True)
        return jsonify({"status": "error", "message": str(e), "error_type": "file_not_found"}), 404
    except Exception as e:
        logger.error(f"Error during HRRR GRIB merge: {e}", exc_info=True)
        return jsonify({"status": "error", "message": str(e), "error_type": "server_error"}), 500

@app.route('/combine_dss', methods=['POST'])
def handle_combine_dss():
    """Endpoint to combine DSS records from HRRR forecast and Real-time GRB processing."""
    logger.info("Received /combine_dss request")
    try:
        # Attempt Pass1Pass2 combine
        logger.info("Attempting DSS combine for Pass1Pass2...")
        success_pass1_pass2 = run_combine_dss_pass1_pass2()
        if success_pass1_pass2:
            logger.info("Successfully combined DSS records for Pass1Pass2.")
        else:
            logger.error("Failed to combine DSS records for Pass1Pass2.")

        # Attempt main DSS combine (HRRR and Real-time) regardless of Pass1Pass2 outcome
        logger.info("Attempting main DSS combine (HRRR and Real-time)...")
        success_main = run_combine_dss()
        if success_main:
            logger.info("Successfully completed main DSS combine.")
        else:
            logger.error("Failed to complete main DSS combine.")

        # Determine overall status and message
        if success_main and success_pass1_pass2:
            return jsonify({
                "status": "success",
                "message": "Successfully combined DSS records (both Pass1Pass2 and main combine)."
            }), 200
        elif success_main and not success_pass1_pass2:
            return jsonify({
                "status": "partial_success",
                "message": "Main DSS combine successful, but Pass1Pass2 combine failed. Check logs for Pass1Pass2 details."
            }), 200 # Still 200 as main task succeeded, but with a warning.
        elif not success_main and success_pass1_pass2:
            return jsonify({
                "status": "error",
                "message": "Pass1Pass2 combine successful, but main DSS combine failed. Check logs for main combine details.",
                "error_type": "combine_failure_main"
            }), 500
        else: # Both failed
            return jsonify({
                "status": "error",
                "message": "Both Pass1Pass2 and main DSS combine failed. Check logs for details.",
                "error_type": "combine_failure_all"
            }), 500
            
    except Exception as e:
        logger.error(f"Error during DSS combine process: {e}", exc_info=True)
        return jsonify({
            "status": "error",
            "message": str(e),
            "error_type": "server_error"
        }), 500

@app.route('/get_dss_data', methods=['POST'])
def handle_get_dss_data():
    """Endpoint to extract data from DSS file for a specific junction."""
    logger.info("Received /get_dss_data request")
    try:
        # Get the b_part_junction from the request JSON, default to None if not provided
        request_data = request.get_json(silent=True) or {}
        b_part_junction = request_data.get('b_part_junction')
        
        logger.info(f"Extracting DSS data for junction: {b_part_junction or 'default'}")
        
        # Call the get_dss_data function from getDataFromDSSJython.py
        output_data, error_output = get_dss_data(b_part_junction)
        
        if error_output and not output_data:
            logger.error(f"Error extracting DSS data: {error_output}")
            return jsonify({
                "status": "error",
                "message": "Failed to extract DSS data"
            }), 500
        
        # Return success status without including the data
        return jsonify({
            "status": "success",
            "message": f"Successfully extracted DSS data for junction: {b_part_junction or 'default'}"
        }), 200
        
    except Exception as e:
        logger.error(f"Error extracting DSS data: {e}", exc_info=True)
        return jsonify({
            "status": "error",
            "message": str(e)
        }), 500

# --- Run the App --- 
if __name__ == '__main__':
    # Note: Use a production WSGI server (like Gunicorn or Waitress) for deployment
    logger.info("Starting Flask development server...")
    app.run(debug=True, host='0.0.0.0', port=5000) # Runs on port 5000, accessible on network
