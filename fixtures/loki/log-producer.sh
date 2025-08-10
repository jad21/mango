#!/bin/sh
set -eu
i=0
while :; do
    echo "log line $i from $$"
    i=$((i+1))
    sleep 1
done
