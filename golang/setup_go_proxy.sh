#!/bin/bash

# Set Go proxy environment variables
export https_proxy=http://127.0.0.1:7890
export http_proxy=http://127.0.0.1:7890
export all_proxy=socks5://127.0.0.1:7890
export GOPROXY=https://goproxy.cn,direct
export GOSUMDB=off
export GO111MODULE=on

# Print current Go environment
echo "Go proxy environment variables set:"
echo "https_proxy=$https_proxy"
echo "http_proxy=$http_proxy"
echo "all_proxy=$all_proxy"
echo "GOPROXY=$GOPROXY"
echo "GOSUMDB=$GOSUMDB"
echo "GO111MODULE=$GO111MODULE"

# You can source this file to set these variables in your current shell:
# source setup_go_proxy.sh
