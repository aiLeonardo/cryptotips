#!/bin/env bash

APP_NAME="$1"

if [ -z "$APP_NAME" ]; then
    echo "Usage: $0 <program_name>"
    exit 1
fi

PID_FILE="logs/app.${APP_NAME}.pid"

if [ ! -f "$PID_FILE" ]; then
    echo "PID 文件不存在: $PID_FILE"
    exit 1
fi

PID=$(cat "$PID_FILE" | tr -d ' \t\n\r')

if [ -z "$PID" ]; then
    echo "PID 文件为空: $PID_FILE"
    exit 1
fi

if ! kill -0 "$PID" 2>/dev/null; then
    echo "[$APP_NAME] 进程 $PID 不存在或未运行."
    exit 0
fi

TIMEOUT=10

echo "发送 SIGTERM 给 [$APP_NAME] (PID: $PID)..."
kill -TERM "$PID"

for ((i=0; i<$TIMEOUT; i++)); do
    if ! kill -0 "$PID" 2>/dev/null; then
        echo "[$APP_NAME] 已优雅退出."
        exit 0
    fi
    sleep 1
done

echo "[$APP_NAME] 优雅退出超时，发送 SIGKILL..."
kill -KILL "$PID"

echo "[$APP_NAME] 已强制关闭."
