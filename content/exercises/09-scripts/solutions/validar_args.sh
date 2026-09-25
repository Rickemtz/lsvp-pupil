#!/bin/bash
if [ $# -eq 0 ]; then
    echo "Uso: validar_args.sh ARCHIVO"
    exit 1
fi
echo "Recibí: $1"
