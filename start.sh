#!/bin/bash

# 设置工作目录
cd "$(dirname "$0")"
pwd

# 检查是否存在 conda 环境
if conda info --envs | grep -q "lingzhi"; then
    echo "使用 conda 环境 lingzhi..."
    conda run -n lingzhi python python/api.py
else
    echo "未找到 conda 环境，请先创建环境："
    echo "cd python && conda env create -f environment.yml"
    exit 1
fi
