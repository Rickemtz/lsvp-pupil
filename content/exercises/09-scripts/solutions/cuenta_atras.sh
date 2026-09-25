#!/bin/bash
n=$1
while [ "$n" -ge 0 ]; do
    echo "$n"
    ((n--))
done
