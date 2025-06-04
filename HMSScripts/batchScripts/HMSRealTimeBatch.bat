@echo off
setlocal EnableDelayedExpansion

rem ===== HMS RealTime Computation Batch Script ====================================
rem This script executes HEC-HMS for real-time computation
rem Usage: HMSRealTimeBatch.bat <script_path> <hms_models_directory>

rem ===== CONFIGURATION ============================================================
set "HMS_HOME=C:\Program Files\HEC\HEC-HMS\4.12"
set "HMS_EXECUTABLE=%HMS_HOME%\HEC-HMS.cmd"

rem ===== VALIDATE ARGUMENTS =======================================================
if "%~1"=="" (
    echo ERROR: Script path argument is required
    echo Usage: HMSRealTimeBatch.bat ^<script_path^> ^<hms_models_directory^>
    exit /b 1
)

if "%~2"=="" (
    echo ERROR: HMS models directory argument is required
    echo Usage: HMSRealTimeBatch.bat ^<script_path^> ^<hms_models_directory^>
    exit /b 1
)

set "SCRIPT_PATH=%~1"
set "HMS_MODELS_DIR=%~2"

rem ===== VALIDATE HMS INSTALLATION ================================================
if not exist "%HMS_EXECUTABLE%" (
    echo ERROR: HEC-HMS executable not found at: %HMS_EXECUTABLE%
    echo Please verify HMS installation path in this batch file
    exit /b 1
)

rem ===== VALIDATE SCRIPT FILE =====================================================
if not exist "%SCRIPT_PATH%" (
    echo ERROR: Script file not found at: %SCRIPT_PATH%
    exit /b 1
)

rem ===== EXECUTE HMS ==============================================================
echo === Starting HMS RealTime Computation ===========================================
echo HMS Home: %HMS_HOME%
echo Script: %SCRIPT_PATH%
echo HMS Models Dir: %HMS_MODELS_DIR%
echo =================================================================================

rem Set environment variable for the Jython script
set "HMS_MODELS_DIR=%HMS_MODELS_DIR%"

cd /d "%HMS_HOME%"

"%HMS_EXECUTABLE%" -script "%SCRIPT_PATH%"

if errorlevel 1 (
    echo **** ERROR: HMS RealTime computation failed with exit code %errorlevel% ****
    exit /b %errorlevel%
)
echo === HMS RealTime Computation Completed Successfully =============================