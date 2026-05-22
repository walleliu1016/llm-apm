#!/bin/bash
# LLM-APM Server Startup Script
# 设置 GreptimeDB HTTP 端口（如果您的 GreptimeDB 运行在不同端口，请修改）

cd "$(dirname "$0")/server"

export APM_GREPTIMEDB_HTTP_PORT=4000

echo "Starting LLM-APM Server..."
echo "Dashboard: http://127.0.0.1:14318/"
echo "GreptimeDB port: $APM_GREPTIMEDB_HTTP_PORT"

../bin/llm-apm-server