#!/bin/bash
if [ -f "$1" ]; then
    wc -l < "$1" > salida.txt
else
    echo "El archivo $1 no existe"
    exit 1
fi
