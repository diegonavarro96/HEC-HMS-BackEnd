@echo off
setlocal EnableDelayedExpansion

rem ===== CONFIGURACIÓN BÁSICA ====================================
set "VORTEX_HOME=C:\Program Files\HEC\vortex-0.11.25"
set "JYTHON_HOME=C:\Jython2.7.4"
set "JYTHON_SCRIPT=D:\FloodaceDocuments\HMS\HMSBackend\python_scripts\Jython_Scripts\MergeGRIBFilesRealTimeHRRJython.py"
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

rem ===== CHEQUEAR HEAP ASIGNADO ==================================
echo === JVM heap check ==========================================
"%VORTEX_HOME%\jre\bin\java.exe" -Xmx%HEAP_GB%g -XX:+PrintFlagsFinal -version 2>&1 | findstr /i "MaxHeapSize"
echo =============================================================

rem ===== EJECUTAR EL SCRIPT JYTHON ===============================
"%VORTEX_HOME%\jre\bin\java.exe" ^
    -Xmx%HEAP_GB%g ^
    "-Djava.library.path=%VORTEX_HOME%\bin;%VORTEX_HOME%\bin\gdal" ^
    -cp "%CLASSPATH%" ^
    org.python.util.jython "%JYTHON_SCRIPT%" %*

if errorlevel 1 (
    echo **** ERROR: Pass 2 falló. Revisa el log. ****
    exit /b 1
)

endlocal