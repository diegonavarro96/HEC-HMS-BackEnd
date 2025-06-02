@echo off
setlocal EnableDelayedExpansion

rem ===== CONFIGURACIÓN BÁSICA ====================================
set "VORTEX_HOME=C:\Program Files\HEC\vortex-0.11.25"
set "JYTHON_HOME=C:\Jython2.7.4"
set "JYTHON_SCRIPT=D:\FloodaceDocuments\HMS\HMSBackend\python_scripts\Jython_Scripts\CombineTwoDssFiles.py"
set "HEAP_GB=32"          rem ← ajusta el -Xmx a tus necesidades

rem ===== RUTAS Y VARIABLES DE ENTORNO ============================
set "PATH=%VORTEX_HOME%\bin\gdal;%VORTEX_HOME%\bin\netcdf;%PATH%"
set "GDAL_DATA=%VORTEX_HOME%\bin\gdal\gdal-data"
set "PROJ_LIB=%VORTEX_HOME%\bin\gdal\projlib"

rem ----- CLASSPATH ------------------------------------------------
rem  Importante: la barra antes del * para que Windows expanda los JAR
set "CLASSPATH=%VORTEX_HOME%\lib\*;%JYTHON_HOME%\jython.jar"

rem ===== LIMITAR EL PARALELISMO (evita el ConcurrentImporter) ====
set "JAVA_TOOL_OPTIONS=-Djava.util.concurrent.ForkJoinPool.common.parallelism=1"

rem ===== VERIFICAR ARGUMENTOS =====================================
if "%~3"=="" (
    echo ERROR: Se requieren 3 argumentos
    echo USAGE: %~nx0 ^<RainfallRealTime.dss^> ^<RainfallRealTimePass2.dss^> ^<RainfallRealTimePass1And2.dss^>
    exit /b 1
)

rem ===== CHEQUEAR HEAP ASIGNADO ==================================
echo === JVM heap check ==========================================
"%VORTEX_HOME%\jre\bin\java.exe" -Xmx%HEAP_GB%g -XX:+PrintFlagsFinal -version 2>&1 | findstr /i "MaxHeapSize"
echo =============================================================

rem ===== EJECUTAR EL SCRIPT JYTHON ===============================
echo Combinando Pass1 y Pass2 DSS files...
echo   Source 1: %~1
echo   Source 2: %~2
echo   Destino:  %~3
echo.

"%VORTEX_HOME%\jre\bin\java.exe" ^
    -Xmx%HEAP_GB%g ^
    "-Djava.library.path=%VORTEX_HOME%\bin;%VORTEX_HOME%\bin\gdal" ^
    -cp "%CLASSPATH%" ^
    org.python.util.jython "%JYTHON_SCRIPT%" %*

if errorlevel 1 (
    echo **** ERROR: Combinación de Pass1 y Pass2 falló. Revisa el log. ****
    exit /b 1
)

echo.
echo === Combinación de Pass1 y Pass2 completada exitosamente ===

endlocal