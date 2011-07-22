#!/bin/bash
set -e

if ! which roundup > /dev/null
then
    echo "You need to install roundup to run this script."
    echo "See: http://bmizerany.github.com/roundup"
    exit 1
fi

TARGS="define_test
struct_test
enum_test
"

{
    for targ in $TARGS
    do
        name=$(echo $targ | sed 's/\//_/' | sed 's/\-/_/' | sed 's/\-/_/' | sed 's/\-/_/' | tr -d .)
        echo "it_passes_$name() { regress/run.sh regress/$name.h regress/$name.go; }"
    done
} > all-test.sh

: ${GOMAXPROCS:=10}
export GOMAXPROCS

trap 'rm -f all-test.sh' EXIT INT
roundup ./all-test.sh
