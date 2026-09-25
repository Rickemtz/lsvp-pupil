#!/bin/bash
if [ "$1" -gt 10 ]; then
    echo "$1 es mayor que 10"
elif [ "$1" -lt 10 ]; then
    echo "$1 es menor que 10"
else
    echo "$1 es igual a 10"
fi
