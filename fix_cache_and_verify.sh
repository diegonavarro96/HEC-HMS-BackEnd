#\!/bin/bash

echo "Limpiando caché corrupto y verificando IDs de Google Drive..."

# 1. Limpiar el caché corrupto
if [ -f .setup_cache ]; then
    echo "Contenido actual del caché:"
    cat .setup_cache
    echo ""
    echo "Limpiando caché..."
    rm -f .setup_cache
fi

# 2. Verificar solo los archivos de Google Drive
echo "Ejecutando verificación de Google Drive..."
./setup_hms_backend.sh --verify-gdrive

