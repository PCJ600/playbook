#!/bin/bash

cur_dir=$(pwd)
output_dir=$cur_dir/output
mkdir -p $output_dir

pushd 01-edge-gateway
    go build -o edge-gateway .
    cp -f edge-gateway $output_dir
popd

pushd 02-edge-proxy
    go build -o edge-proxy .
    cp -f edge-proxy $output_dir
popd

pushd 03-mq-consumer
    go build -o mq-consumer .
    cp -f mq-consumer $output_dir
popd

cd $output_dir
nohup ./edge-gateway >> edge-gateway.log 2>&1 &
nohup ./edge-proxy >> edge-proxy.log 2>&1 &
nohup ./mq-consumer >> mq-consumer.log 2>&1 &
