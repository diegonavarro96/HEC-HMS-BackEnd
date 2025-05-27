import os
import sys
import yaml
import shutil
import subprocess
import time
import datetime
import re
from typing import List, Tuple, Optional, Dict

# Add the current directory to the path to ensure imports work properly
SCRIPT_DIR = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, SCRIPT_DIR)
import setControlFile  # Import the setControlFile module

class HmsRunnerError(Exception):
    """Custom exception for Hms compute errors"""
    pass

SCRIPT_DIR = os.path.dirname(__file__) if '__file__' in locals() else os.getcwd()
# Load config from the same directory as the script
DEFAULT_CONFIG_PATH = os.path.abspath(os.path.join(SCRIPT_DIR, 'config.yaml'))
PROJECT_ROOT_DIR = os.path.abspath(os.path.join(SCRIPT_DIR, '..'))

def load_config(config_path: Optional[str] = None) -> dict:
    path = config_path or DEFAULT_CONFIG_PATH
    if not os.path.exists(path):
        raise FileNotFoundError(f"Config file not found: {path}")
    with open(path, 'r', encoding='utf-8') as f:
        try:
            return yaml.safe_load(f)
        except yaml.YAMLError as e:
            raise ValueError(f"Failed to parse config: {e}")

def get_full_data_path(config: dict, key: str) -> str:
    sub = config.get('data_paths', {}).get(key)
    if not sub:
        raise KeyError(f"Missing data_paths.{key} in config")
    full = sub if os.path.isabs(sub) else os.path.join(PROJECT_ROOT_DIR, sub)
    os.makedirs(full, exist_ok=True)
    return full

def discover_project_files(project_dir: str) -> Tuple[str, str]:
    try:
        entries = os.listdir(project_dir)
    except FileNotFoundError:
        raise HmsRunnerError(f"Project directory not found: {project_dir}")
    hms = [f for f in entries if f.lower().endswith('.hms')]
    run = [f for f in entries if f.lower().endswith('.run')]
    if len(hms) != 1:
        raise HmsRunnerError(f"Expected 1 .hms file, found {len(hms)}: {hms}")
    if len(run) != 1:
        raise HmsRunnerError(f"Expected 1 .run file, found {len(run)}: {run}")
    return hms[0], run[0]

def parse_run_file(run_file_path: str) -> List[str]:
    if not os.path.isfile(run_file_path):
        raise FileNotFoundError(f"Run file not found: {run_file_path}")
    runs = []
    with open(run_file_path, 'r', encoding='utf-8', errors='replace') as f:
        for line in f:
            line = line.strip()
            if line.startswith('Run:'):
                parts = line.split(':', 1)
                if len(parts) > 1 and parts[1].strip():
                    runs.append(parts[1].strip())
    if not runs:
        raise HmsRunnerError(f"No runs found in: {run_file_path}")
    return runs

def prompt_user_for_runs(available_runs: List[str]) -> List[str]:
    print("Available runs:")
    for i, name in enumerate(available_runs, start=1):
        print(f"  {i}: {name}")
    print("Enter numbers separated by commas, or 'all'.")
    while True:
        choice = input("Select runs> ").strip().lower()
        if choice == 'all':
            return available_runs[:]
        nums = re.split(r'\s*,\s*', choice)
        selected = []
        try:
            for n in nums:
                idx = int(n) - 1
                if 0 <= idx < len(available_runs):
                    selected.append(available_runs[idx])
                else:
                    raise ValueError
            if selected:
                return selected
        except ValueError:
            print("Invalid selection. Try again.")

def generate_script_and_paths(
    config: dict,
    project_dir: str,
    project_file: str,
    run_name: str,
    dss_suffix: str
) -> Tuple[str, str]:
    install = config.get('install_paths', {})
    hms_home = install.get('hec_hms')
    if not hms_home:
        raise KeyError("install_paths.hec_hms missing")

    temp_dir = get_full_data_path(config, 'temp_files_dir')
    os.makedirs(temp_dir, exist_ok=True)

    # Derive project base (filename without extension) and create .script file
    project_base = os.path.splitext(os.path.basename(project_file))[0]
    script_path = os.path.join(temp_dir, f"compute_{run_name}.script")
    
    # Create the Jython script content
    script_content = f'''# -*- coding: utf-8 -*-
from hms.model.JythonHms import *

# Open the HMS project
OpenProject(
    "{project_base}",
    r"{project_dir}"
)

# Run the specified simulation
ComputeRun("{run_name}")

# Optional: save any changes
SaveAllProjectComponents()

# Cleanly shut down without printing a stack trace
try:
    Exit(0)
except SystemExit:
    pass
'''

    with open(script_path, 'w', encoding='utf-8') as f:
        f.write(script_content)
    return os.path.join(hms_home, 'hec-hms.cmd'), script_path

def execute_run(
    project_dir: str,
    run_name: str,
    hms_cmd: str,
    script_path: str,
    dss_suffix: str
) -> Tuple[bool, str]:
    # Call the official HMS CLI with the Jython script
    cmd = [hms_cmd, '-script', script_path]
    hms_cmd_dir = os.path.dirname(hms_cmd)
    print(f"--- Starting HEC-HMS compute for run: '{run_name}' ---")
    proc = subprocess.run(cmd, cwd=hms_cmd_dir, capture_output=True, text=True)
    ok = (proc.returncode == 0)
    if ok:
        print(f"--- Finished HEC-HMS compute for run: '{run_name}' (Success) ---")
    log_old = os.path.join(project_dir, f"{run_name}.log")
    log_new = os.path.join(project_dir, f"{run_name}{dss_suffix}.log")
    if ok and os.path.exists(log_old):
        if os.path.exists(log_new): os.remove(log_new)
        os.rename(log_old, log_new)
    msg = proc.stdout + proc.stderr
    return ok, msg

def update_rain_realtime_control(project_dir: str) -> bool:
    """
    Updates the RainRealTime.control file by calling the setControlFile module.
    
    Args:
        project_dir: The directory where the HMS project is located
        
    Returns:
        bool: True if successful, False otherwise
    """
    control_file_path = os.path.join(project_dir, "RainRealTime.control")
    if not os.path.exists(control_file_path):
        print(f"Error: RainRealTime.control file not found at {control_file_path}")
        return False
    
    try:
        # Create a backup of the original file
        backup_path = control_file_path + ".bak"
        shutil.copy2(control_file_path, backup_path)
        
        # Use the setControlFile module to update the control file
        print(f"Updating RainRealTime.control file at {control_file_path}")
        
        # For debugging
        print(f"Using setControlFile module: {setControlFile.__file__}")
        
        # Call the update function directly
        success = setControlFile.update_control_file(control_file_path)
        
        if success:
            print(f"Successfully updated RainRealTime.control file")
            return True
        else:
            print(f"Failed to update RainRealTime.control file")
            return False
        
    except Exception as e:
        print(f"Error updating RainRealTime.control file: {e}")
        import traceback
        traceback.print_exc()  # Print the full stack trace for debugging
        return False

def run_computations(
    config_path: Optional[str] = None,
    project_key: Optional[str] = None,
    run_name_to_execute: Optional[str] = None
) -> Dict[str, int]:
    cfg = load_config(config_path)
    projects = cfg.get('hms_projects', {})
    if not projects:
        raise HmsRunnerError("No projects in config.hms_projects")
    key = project_key or next(iter(projects))
    proj_cfg = projects.get(key)
    if not proj_cfg:
        raise KeyError(f"Project '{key}' not in config")

    model_base = get_full_data_path(cfg, 'hms_model_base_subdir')
    proj_dir = os.path.join(model_base, proj_cfg['directory_name'])
    if not os.path.isdir(proj_dir):
        raise FileNotFoundError(f"Project dir not found: {proj_dir}")

    hms_file, run_file = discover_project_files(proj_dir)
    runs = parse_run_file(os.path.join(proj_dir, run_file))
    
    # Determine which runs to execute
    if run_name_to_execute:
        if run_name_to_execute not in runs:
            raise ValueError(f"Specified run '{run_name_to_execute}' not found in {run_file}")
        selected = [run_name_to_execute]
        print(f"--- Preparing to execute specified run: '{run_name_to_execute}' ---")
    else:
        # Fallback to prompting if no specific run is given
        selected = prompt_user_for_runs(runs)

    success_count = 0
    failed_runs = [] # Keep track of failures
    for run_name in selected:
        # For RainRealTime, update the control file directly
        if run_name == "RainRealTime":
            control_file_path = os.path.join(proj_dir, "RainRealTime.control")
            print(f"--- Directly updating RainRealTime.control at {control_file_path} ---")
            
            # First try using setControlFile module directly
            if os.path.exists(control_file_path):
                try:
                    # Create backup first
                    backup_path = control_file_path + ".bak"
                    if os.path.exists(control_file_path):
                        shutil.copy2(control_file_path, backup_path)
                        print(f"Created backup at {backup_path}")
                    
                    # Call setControlFile directly with absolute path
                    direct_update = setControlFile.update_control_file(control_file_path)
                    print(f"Direct update of control file result: {direct_update}")
                except Exception as e:
                    print(f"Error in direct setControlFile call: {e}")
                    import traceback
                    traceback.print_exc()
            
            # Then try with our wrapper function as a fallback
            update_success = update_rain_realtime_control(proj_dir)
            if not update_success:
                print(f"Warning: Failed to update RainRealTime.control file, proceeding with existing settings")
        
        hms_cmd, script_path = generate_script_and_paths(
            cfg,
            proj_dir,
            os.path.join(proj_dir, hms_file),
            run_name,
            proj_cfg.get('output_dss_suffix', '_RunOutput')
        )
        ok, msg = execute_run(
            proj_dir,
            run_name,
            hms_cmd,
            script_path,
            proj_cfg.get('output_dss_suffix', '_RunOutput')
        )
        if ok:
            success_count += 1
        else:
            failed_runs.append({'name': run_name, 'message': msg})
            # Optionally print error here too, or let caller handle it
            # print(f"Run '{run_name}' failed. HEC-HMS output:")
            # print("-" * 40)
            # print(msg.strip())
            # print("-" * 40)

    # Return a more detailed summary including failures
    summary = {
        'attempted': len(selected),
        'succeeded': success_count,
        'failed': len(failed_runs),
        'failures': failed_runs # List of dicts {name, message}
    }
    return summary

if __name__ == '__main__':
    # Default behavior: Prompt user for runs
    try:
        summary = run_computations() # run_name_to_execute is None
        print(f"Completed {summary['succeeded']}/{summary['attempted']} runs.")
        if summary['failed'] > 0:
            print("--- Failed Runs ---")
            for failure in summary['failures']:
                print(f"Run: {failure['name']}")
                print("Output:")
                print(failure['message'].strip())
                print("-" * 20)
    except Exception as e:
        print(f"ERROR: {e}")
        sys.exit(1)
