#!/bin/bash
# Usage:
#
# regress/run.py test.h golden.go
set -e

if ! which goheader > /dev/null
then
    echo "You need to install goheader to run this script."
    exit 1
fi

TEST_INPUT=$1
TEST_OUTPUT=$2

goheader -s=linux -p=test "$TEST_INPUT" | diff /proc/self/fd/0 "$TEST_OUTPUT"
