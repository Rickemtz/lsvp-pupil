#!/bin/bash
cat "$1" 2>> error.log || echo "Error al mostrar el archivo"
