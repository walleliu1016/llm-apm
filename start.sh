#!/bin/bash
# LLM-APM Server Startup Script
#
# 所有配置选项（环境变量）：
#
# | 变量                        | 默认值      | 说明                        |
# |-----------------------------|-------------|-----------------------------|
# | APM_HOST                    | 127.0.0.1   | HTTP 服务绑定地址           |
# | APM_PORT                    | 14318       | HTTP 服务端口               |
# | APM_DATA_DIR                | ~/.llm-apm  | 数据存储目录                |
# | APM_GREPTIMEDB_HOST         | 127.0.0.1   | GreptimeDB 主机地址         |
# | APM_GREPTIMEDB_EMBEDDED     | true        | 是否启动嵌入式 GreptimeDB   |
# | APM_GREPTIMEDB_HTTP_PORT    | 4000        | GreptimeDB HTTP 端口        |
# | APM_GREPTIMEDB_GRPC_PORT    | 14001       | GreptimeDB GRPC 端口        |
# | APM_GREPTIMEDB_MYSQL_PORT   | 14002       | GreptimeDB MySQL 端口       |
# | APM_LOG_LEVEL               | info        | 日志级别 (debug/info/warn/error) |
# | APM_DATA_TTL                | 60d         | 数据保留时间                |
# | APM_TENANT_ID               | ""          | 多租户 ID（预留）           |
#
# 远程 GreptimeDB 示例：
#   export APM_GREPTIMEDB_HOST=192.168.1.100
#   export APM_GREPTIMEDB_EMBEDDED=false
#   export APM_GREPTIMEDB_HTTP_PORT=4000
#   ./start.sh

cd "$(dirname "$0")/server"

# ===== 默认配置 =====
export APM_HOST="${APM_HOST:-127.0.0.1}"
export APM_PORT="${APM_PORT:-14318}"
export APM_DATA_DIR="${APM_DATA_DIR:-~/.llm-apm}"
export APM_GREPTIMEDB_HOST="${APM_GREPTIMEDB_HOST:-127.0.0.1}"
export APM_GREPTIMEDB_EMBEDDED="${APM_GREPTIMEDB_EMBEDDED:-true}"
export APM_GREPTIMEDB_HTTP_PORT="${APM_GREPTIMEDB_HTTP_PORT:-4000}"
export APM_GREPTIMEDB_GRPC_PORT="${APM_GREPTIMEDB_GRPC_PORT:-14001}"
export APM_GREPTIMEDB_MYSQL_PORT="${APM_GREPTIMEDB_MYSQL_PORT:-14002}"
export APM_LOG_LEVEL="${APM_LOG_LEVEL:-info}"
export APM_DATA_TTL="${APM_DATA_TTL:-60d}"
export APM_TENANT_ID="${APM_TENANT_ID:-}"

echo "Starting LLM-APM Server..."
echo "  Dashboard: http://${APM_HOST}:${APM_PORT}/"
echo "  GreptimeDB host: ${APM_GREPTIMEDB_HOST}"
echo "  GreptimeDB embedded: ${APM_GREPTIMEDB_EMBEDDED}"
echo "  GreptimeDB HTTP port: ${APM_GREPTIMEDB_HTTP_PORT}"
echo "  Data dir: ${APM_DATA_DIR}"
echo "  Log level: ${APM_LOG_LEVEL}"

../bin/llm-apm-server