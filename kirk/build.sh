#!/bin/bash

DIR=$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )
cd $DIR

test -d build || mkdir build
cd build

PKG="docker-1.13.0-kirk-v1.tar.gz"

if ! test -f "$PKG"; then
    wget -O $PKG https://dn-qcos.qbox.me/$PKG
fi

echo "f9218367c6c8cea6a1448efe7a63fa69 $PKG" | md5sum -c &>/dev/null
if [ $? -ne 0 ]; then
    echo "$PKG md5sum mismatch, please remove it"
    exit -1
fi

test -d _package || mkdir -p _package
rm -rf _package/*

tar -C _package -xzvf $PKG
cp ../start.sh _package/start.sh
