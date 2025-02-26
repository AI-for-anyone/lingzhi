#!/bin/bash

# 检查 Python 3 是否安装
if ! command -v python3 &> /dev/null; then
    echo "错误: 未找到 Python 3"
    exit 1
fi

# 创建虚拟环境
echo "创建 Python 虚拟环境..."
python3 -m venv venv

# 激活虚拟环境并安装依赖
echo "安装依赖..."
source venv/bin/activate
pip install --upgrade pip
pip install -r requirements.txt

echo "设置完成！"
echo "虚拟环境已创建在 ./venv 目录"
deactivate
