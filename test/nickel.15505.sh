#!/bin/bash

cd "$(dirname "$0")"

if [ "$#" -eq 0 ] || [ "$1" -eq 0 ]; then
    echo "> Building qrc2zip" 1>&2
    go build -v -mod=readonly -o qrc2zip ../cmd/qrc2zip || { echo "Error: build qrc2zip failed." 1>&2; exit 1; }
fi

if [ "$#" -eq 0 ] || [ "$1" -eq 1 ]; then
    echo "> Downloading nickel from pgaskin/kobopatch-patches/testdata/4.23.15505.tar.xz" 1>&2
    wget -qO- https://github.com/pgaskin/kobopatch-patches/raw/master/testdata/4.23.15505.tar.xz | tar xJvf - nickel || { echo "Error: nickel download failed." 1>&2; exit 1; }
fi

if [ "$#" -eq 0 ] || [ "$1" -eq 2 ]; then
    echo "> Extracting resources" 1>&2
    echo "nickel@15505/EntryPoint_3";                ./qrc2zip --output "nickel.15055.EntryPoint_3.zip"                --recursive --verbose "nickel" 1 $((0x1159ff0 - 0x0010000)) $((0x0026608 - 0x0010000)) $((0x1159ba0 - 0x0010000)) || { echo "Error: qrc2zip failed." 1>&2; exit 1; }
    echo "nickel@15505/EntryPoint_4";                ./qrc2zip --output "nickel.15055.EntryPoint_4.zip"                --recursive --verbose "nickel" 1 $((0x14ba4f8 - 0x0010000)) $((0x115a390 - 0x0010000)) $((0x14ba270 - 0x0010000)) || { echo "Error: qrc2zip failed." 1>&2; exit 1; }
    echo "nickel@15505/EntryPoint_5";                ./qrc2zip --output "nickel.15055.EntryPoint_5.zip"                --recursive --verbose "nickel" 1 $((0x14d6ad0 - 0x0010000)) $((0x14ba610 - 0x0010000)) $((0x14d4528 - 0x0010000)) || { echo "Error: qrc2zip failed." 1>&2; exit 1; }
    echo "nickel@15505/EntryPoint_6";                ./qrc2zip --output "nickel.15055.EntryPoint_6.zip"                --recursive --verbose "nickel" 1 $((0x14fde20 - 0x0010000)) $((0x14d7580 - 0x0010000)) $((0x14fcdd0 - 0x0010000)) || { echo "Error: qrc2zip failed." 1>&2; exit 1; }
    echo "nickel@15505/qInitResources_resources";    ./qrc2zip --output "nickel.15055.qInitResources_resources.zip"    --recursive --verbose "nickel" 1 $((0x1159ff0 - 0x0010000)) $((0x0026608 - 0x0010000)) $((0x1159ba0 - 0x0010000)) || { echo "Error: qrc2zip failed." 1>&2; exit 1; }
    echo "nickel@15505/qInitResources_translations"; ./qrc2zip --output "nickel.15055.qInitResources_translations.zip" --recursive --verbose "nickel" 1 $((0x14ba4f8 - 0x0010000)) $((0x115a390 - 0x0010000)) $((0x14ba270 - 0x0010000)) || { echo "Error: qrc2zip failed." 1>&2; exit 1; }
    echo "nickel@15505/qInitResources_styles";       ./qrc2zip --output "nickel.15055.qInitResources_styles.zip"       --recursive --verbose "nickel" 1 $((0x14d6ad0 - 0x0010000)) $((0x14ba610 - 0x0010000)) $((0x14d4528 - 0x0010000)) || { echo "Error: qrc2zip failed." 1>&2; exit 1; }
    echo "nickel@15505/qInitResources_certificates"; ./qrc2zip --output "nickel.15055.qInitResources_certificates.zip" --recursive --verbose "nickel" 1 $((0x14fde20 - 0x0010000)) $((0x14d7580 - 0x0010000)) $((0x14fcdd0 - 0x0010000)) || { echo "Error: qrc2zip failed." 1>&2; exit 1; }
fi

if [ "$#" -eq 0 ] || [ "$1" -eq 3 ]; then
    echo "> Checking zip sha1sums" 1>&2
    cat <<-EOF | sha1sum --check - || { echo "Error: sha1sum check failed." 1>&2; exit 1; }
        05c8449f5b94e9354d9f66deccd53c2d3c4b9693  nickel.15055.EntryPoint_3.zip
        5002cb9e5cbe899dda7eaa29a017cc8830ffaa4c  nickel.15055.EntryPoint_4.zip
        53c1063ceb1a22115df83990b881be9bb7426464  nickel.15055.EntryPoint_5.zip
        df12947d32db5403812d66eb62cf5b8acb1298f7  nickel.15055.EntryPoint_6.zip
        05c8449f5b94e9354d9f66deccd53c2d3c4b9693  nickel.15055.qInitResources_resources.zip
        5002cb9e5cbe899dda7eaa29a017cc8830ffaa4c  nickel.15055.qInitResources_translations.zip
        53c1063ceb1a22115df83990b881be9bb7426464  nickel.15055.qInitResources_styles.zip
        df12947d32db5403812d66eb62cf5b8acb1298f7  nickel.15055.qInitResources_certificates.zip
EOF
fi

if [ "$#" -eq 0 ] || [ "$1" -eq 4 ]; then
    echo "> Listing files" 1>&2
    du -h *.zip || { echo "Error: list files failed". 1>&2; exit 1; }
fi

if [ "$#" -eq 0 ] || [ "$1" -eq 5 ]; then
    echo "> Cleaning up" 1>&2
    rm -fv nickel qrc2zip *.zip
fi
