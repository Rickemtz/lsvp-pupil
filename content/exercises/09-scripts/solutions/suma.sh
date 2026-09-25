#!/bin/bash
suma=0
for ((i = 1; i <= $1; i++)); do
    ((suma += i))
done
echo "La suma es $suma"
