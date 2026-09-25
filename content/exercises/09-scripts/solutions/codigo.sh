#!/bin/bash
ls "$1" > /dev/null 2>&1
echo "Código: $?"
