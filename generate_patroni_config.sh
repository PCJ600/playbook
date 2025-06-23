#!/bin/bash

# 定义基础变量
CLUSTER_NAME="pg-prod-cluster"
POSTGRES_VERSION="17"
POSTGRES_BIN_DIR="/usr/pgsql-${POSTGRES_VERSION}/bin"
POSTGRES_DATA_DIR="/var/lib/pgsql/${POSTGRES_VERSION}/data"
SUPERUSER_PASSWORD="password123"
REPLICATION_PASSWORD="password123"
REWIND_PASSWORD="password123"

# 检查是否提供了3个IP地址
if [ "$#" -ne 3 ]; then
    echo "Usage: $0 <IP1> <IP2> <IP3>"
    exit 1
fi

IP1=$1
IP2=$2
IP3=$3

# 为每个节点生成配置文件
for i in 1 2 3; do
    case $i in
        1) NODE_IP=$IP1; NODE_NAME="db1"; FAILOVER_PRIORITY=1 ;;
        2) NODE_IP=$IP2; NODE_NAME="db2"; FAILOVER_PRIORITY=0 ;;
        3) NODE_IP=$IP3; NODE_NAME="db3"; FAILOVER_PRIORITY=0 ;;
    esac

    cat > patroni_${NODE_NAME}.yml <<EOF
scope: ${CLUSTER_NAME}
name: ${NODE_NAME}

log:
  format: '%(asctime)s %(levelname)s: %(message)s'
  level: INFO
  max_queue_size: 1000
  traceback_level: ERROR
  type: plain

restapi:
  connect_address: ${NODE_IP}:8008
  listen: ${NODE_IP}:8008

etcd3:
  hosts:
    - "${IP1}:2379"
    - "${IP2}:2379"
    - "${IP3}:2379"
  protocol: http
  api_version: v3

bootstrap:
  dcs:
    loop_wait: 10
    retry_timeout: 10
    ttl: 30
    postgresql:
      initdb:
        - encoding: UTF8
        - locale: en_US.UTF-8
        - data-checksums
      parameters:
        hot_standby: 'on'
        max_connections: 200
        max_locks_per_transaction: 64
        max_prepared_transactions: 0
        max_replication_slots: 10
        max_wal_senders: 10
        max_worker_processes: 8
        track_commit_timestamp: 'off'
        wal_keep_size: "1GB"
        wal_level: replica
        wal_log_hints: 'on'
        shared_buffers: "1GB"
        effective_cache_size: "3GB"
        work_mem: "16MB"
      use_pg_rewind: true
      use_slots: true

postgresql:
  authentication:
    replication:
      password: '${REPLICATION_PASSWORD}'
      username: replicator
    rewind:
      password: '${REWIND_PASSWORD}'
      username: rewind_user
    superuser:
      password: '${SUPERUSER_PASSWORD}'
      username: postgres
  bin_dir: ${POSTGRES_BIN_DIR}
  connect_address: ${NODE_IP}:5432
  data_dir: ${POSTGRES_DATA_DIR}
  listen: ${NODE_IP}:5432
  parameters:
    password_encryption: md5
  pg_hba:
    - host all all all md5
    - host replication replicator all md5

tags:
  clonefrom: true
  failover_priority: ${FAILOVER_PRIORITY}
  noloadbalance: false
  nostream: false
  nosync: false
EOF

    echo "Generated patroni_${NODE_NAME}.yml for ${NODE_IP}"
done

echo "All configuration files have been generated successfully."
