#!/bin/bash

cur_dir=$PWD
output_dir=$cur_dir/output
mkdir -p $output_dir

pushd 01-mqtt-subscriber
    ./build.sh
    if [ $? -ne 0 ]; then
        echo "compile failed"
        exit 1
    fi
    cp -f mqtt-subscriber $output_dir
popd

pushd 02-kafka-producer
    ./build.sh
    if [ $? -ne 0 ]; then
        echo "compile failed"
        exit 1
    fi
    cp -f kafka-producer $output_dir
popd

pushd 03-kafka-consumer
    ./build.sh
    if [ $? -ne 0 ]; then
        echo "compile failed"
        exit 1
    fi
    cp -f kafka-consumer $output_dir
popd
