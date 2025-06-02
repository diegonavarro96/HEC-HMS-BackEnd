@echo off
setlocal EnableDelayedExpansion

rem ===== HMS Historical Computation Batch Script ==================================
rem This script executes HEC-HMS for historical computation
rem Usage: HMSHistoricalBatch.bat <script_path>

rem ===== CONFIGURATION ============================================================
set "HMS_HOME=C:\Program Files\HEC\HEC-HMS\4.12"
set "HMS_EXECUTABLE=%HMS_HOME%\HEC-HMS.cmd"

rem ===== VALIDATE ARGUMENTS =======================================================
if "%~1"=="" (
    echo ERROR: Script path argument is required
    echo Usage: HMSHistoricalBatch.bat ^<script_path^>
    exit /b 1
)

set "SCRIPT_PATH=%~1"

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
echo === Starting HMS Historical Computation =========================================
echo HMS Home: %HMS_HOME%
echo Script: %SCRIPT_PATH%
echo =================================================================================

cd /d "%HMS_HOME%"

"%HMS_EXECUTABLE%" -script "%SCRIPT_PATH%"

if errorlevel 1 (
    echo **** ERROR: HMS Historical computation failed with exit code %errorlevel% ****
    exit /b %errorlevel%
)

echo === HMS Historical Computation Completed Successfully ===========================
endlocal