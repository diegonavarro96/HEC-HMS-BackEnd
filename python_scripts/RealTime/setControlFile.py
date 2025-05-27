import datetime
import os
import sys

# Path to your control file
file_path = r"D:\FloodaceDocuments\HMS\HMSGit\HEC-HMS-Floodace\hms_models\LeonCreek\RainRealTime.control"

def round_down_to_hour(dt):
    """Round down a datetime to the nearest hour by setting minutes and seconds to 0."""
    return dt.replace(minute=0, second=0, microsecond=0)

def update_control_file(control_file_path=None):
    """Update the control file with current date and time settings."""
    # Use the provided path or default to the module-level file_path
    target_path = control_file_path or file_path
    
    print(f"setControlFile: Updating control file at: {target_path}")
    
    # Get the current time in UTC and round down to the hour
    now_utc = round_down_to_hour(datetime.datetime.utcnow())
    print(f"setControlFile: Current UTC time (rounded down): {now_utc.strftime('%Y-%m-%d %H:%M:%S')}")

    # Calculate start datetime (47 hours before current UTC time)
    start_datetime = round_down_to_hour(now_utc - datetime.timedelta(hours=47))
    start_time_str = start_datetime.strftime("%H:%M")
    # Use '%#d' for day without leading zero on Windows, '%-d' on Linux/macOS
    # Sticking with '%#d' as per original script and user's OS (Windows)
    start_date_fmt = start_datetime.strftime("%#d %B %Y")

    # Calculate end datetime (12 hours after current UTC time)
    end_datetime = round_down_to_hour(now_utc + datetime.timedelta(hours=12))
    end_time_str = end_datetime.strftime("%H:%M")
    end_date_fmt = end_datetime.strftime("%#d %B %Y")

    print(f"setControlFile: Calculated Start: {start_date_fmt} {start_time_str} (UTC-47h)")
    print(f"setControlFile: Calculated End:   {end_date_fmt} {end_time_str} (UTC+12h)")

    try:
        # New file lines
        with open(target_path, "r", encoding="utf-8") as f:
            lines = f.readlines()

        with open(target_path, "w", encoding="utf-8") as f:
            for line in lines:
                if line.strip().startswith("Start Date:"):
                    f.write(f"     Start Date: {start_date_fmt}\n")
                elif line.strip().startswith("End Date:"):
                    f.write(f"     End Date: {end_date_fmt}\n")
                elif line.strip().startswith("Start Time:"):
                    f.write(f"     Start Time: {start_time_str}\n")
                elif line.strip().startswith("End Time:"):
                    f.write(f"     End Time: {end_time_str}\n")
                else:
                    f.write(line)
        
        print(f"setControlFile: Successfully updated control file")
        return True
    except Exception as e:
        print(f"setControlFile: Error updating control file: {e}")
        import traceback
        traceback.print_exc()
        return False

# If the script is run directly, execute the update function
if __name__ == "__main__":
    print(f"Running setControlFile.py as main script")
    update_control_file()
    print(f"setControlFile.py execution completed")
