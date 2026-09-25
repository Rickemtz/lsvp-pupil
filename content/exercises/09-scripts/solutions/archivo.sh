#!/bin/bash
if [ -f "$1" ]; then
    ls -l "$1"
else
    echo "El archivo $1 no existe"
fi
