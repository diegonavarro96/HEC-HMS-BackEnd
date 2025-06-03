# HEC-HMS Backend - Guía de Instalación para WSL/Linux

## Requisitos Previos

### Software Requerido

1. **HEC-HMS 4.12** - Ya instalado en `/opt/hms`
2. **PostgreSQL** - Base de datos
3. **Go 1.21+** - Lenguaje de programación principal
4. **Python 3.10** - Para scripts de procesamiento
5. **Java 8+** - Para ejecutar Jython
6. **Jython 2.7.3** - Para scripts que usan bibliotecas Java de HEC

### Dependencias del Sistema

```bash
# Actualizar sistema
sudo apt update && sudo apt upgrade -y

# Instalar dependencias básicas
sudo apt install -y \
    build-essential \
    curl \
    wget \
    git \
    postgresql \
    postgresql-contrib \
    openjdk-11-jdk \
    gdal-bin \
    libgdal-dev
```

## Pasos de Instalación

### 1. Instalar Jython

```bash
# Descargar Jython standalone
wget https://repo1.maven.org/maven2/org/python/jython-standalone/2.7.3/jython-standalone-2.7.3.jar

# Mover a /opt
sudo mv jython-standalone-2.7.3.jar /opt/jython.jar

# Verificar instalación
java -jar /opt/jython.jar --version
```

### 2. Configurar PostgreSQL

```bash
# Iniciar PostgreSQL
sudo service postgresql start

# Crear usuario y base de datos
sudo -u postgres psql << EOF
CREATE USER hms_user WITH PASSWORD 'your_secure_password_here';
CREATE DATABASE hms_backend OWNER hms_user;
GRANT ALL PRIVILEGES ON DATABASE hms_backend TO hms_user;
EOF

# Actualizar el archivo .env con las credenciales correctas
cd Go/
# Editar .env y actualizar DB_PASSWORD
```

### 3. Instalar Conda/Miniconda

```bash
# Descargar Miniconda
wget https://repo.anaconda.com/miniconda/Miniconda3-latest-Linux-x86_64.sh

# Instalar
bash Miniconda3-latest-Linux-x86_64.sh -b -p $HOME/miniconda3

# Activar conda
eval "$($HOME/miniconda3/bin/conda shell.bash hook)"

# Agregar al .bashrc
echo 'eval "$($HOME/miniconda3/bin/conda shell.bash hook)"' >> ~/.bashrc
```

### 4. Crear Entornos Python

```bash
# Entorno principal HMS
conda create -n hechmsfloodace python=3.10 -y
conda activate hechmsfloodace
conda install -c conda-forge requests beautifulsoup4 watchdog pyyaml flask flask-cors -y

# Entorno para procesamiento GRIB
conda create -n grib2cog python=3.10 -y
conda activate grib2cog
conda install -c conda-forge xarray rioxarray cfgrib eccodes -y
```

### 5. Crear Directorios Necesarios

```bash
# Desde la raíz del proyecto
mkdir -p hms_models/LeonCreek/{Rainfall,dssArchive}
mkdir -p grb_downloads
mkdir -p data/cogs_output
mkdir -p gis_data/shapefiles
mkdir -p dss_files/incoming
mkdir -p logs
mkdir -p CSV
mkdir -p /tmp/floodace_temp
```

### 6. Compilar y Ejecutar el Backend Go

```bash
cd Go/

# Descargar dependencias
go mod download

# Generar certificados SSL (para desarrollo)
openssl req -x509 -newkey rsa:4096 -keyout server.key -out server.crt -days 365 -nodes \
    -subj "/C=US/ST=State/L=City/O=Organization/OU=Unit/CN=localhost"

# Ejecutar migraciones de base de datos
psql -U hms_user -d hms_backend -f sql/schema.sql

# Compilar
go build -o hms-backend .

# Ejecutar
./hms-backend
```

## Verificación de la Instalación

### 1. Verificar HEC-HMS

```bash
/opt/hms/bin/hec-hms.sh -help
```

### 2. Probar Jython con bibliotecas HEC

```bash
# Crear script de prueba
cat > test_jython.py << 'EOF'
import sys
print("Jython version:", sys.version)
print("Jython path:", sys.path)
EOF

# Ejecutar
java -jar /opt/jython.jar test_jython.py
```

### 3. Verificar API

```bash
# Health check
curl -k https://localhost:8443/health
```

## Problemas Comunes

### 1. Error de permisos en scripts

```bash
# Hacer ejecutables todos los scripts .sh
find . -name "*.sh" -type f -exec chmod +x {} \;
```

### 2. Error de bibliotecas HEC en Jython

Los scripts Jython necesitan acceso a las bibliotecas JAR de HEC-HMS:

```bash
# Verificar que existan las bibliotecas
ls /opt/hms/lib/*.jar
```

### 3. Error de conexión a PostgreSQL

```bash
# Verificar que PostgreSQL esté ejecutándose
sudo service postgresql status

# Si no está activo
sudo service postgresql start
```

### 4. Rutas de archivos DSS

Asegúrate de que los modelos HMS y archivos DSS estén en las rutas correctas según `config.yaml`.

## Docker (Opcional)

Para ejecutar en Docker, primero necesitas asegurarte de que todo funcione en WSL. Luego puedes usar los Dockerfiles proporcionados:

```bash
# Construir imagen
docker build -t hms-backend .

# Ejecutar con docker-compose
docker-compose up -d
```

## Notas Importantes

1. **Rutas**: Todas las rutas Windows han sido convertidas a rutas WSL/Linux
2. **Scripts**: Los archivos .bat han sido reemplazados por scripts .sh
3. **Permisos**: Asegúrate de que el usuario tenga permisos para escribir en todos los directorios de datos
4. **HEC-HMS**: La versión Linux de HEC-HMS puede tener diferencias menores con la versión Windows

## Soporte

Si encuentras problemas:

1. Revisa los logs en el directorio `logs/`
2. Verifica que todas las dependencias estén instaladas correctamente
3. Asegúrate de que las rutas en `config.yaml` sean correctas para tu sistema