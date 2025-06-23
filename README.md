# Patroni搭建一主两从高可用集群

# 环境准备
```
Rocky Linux 9.6 x86_64
192.166.149.230 主
192.166.149.231
192.166.149.232
```

# 关闭防火墙
```
systemctl disable firewalld --now
sed -i 's/^SELINUX=.*/SELINUX=permissive/' /etc/selinux/config
setenforce 0
```

# 时间同步

# 安装PostgreSQL(略)

# 安装etcd
```
dnf install -y 'dnf-command(config-manager)'
dnf config-manager --enable pgdg-rhel9-extras
dnf install -y etcd

etcd --version
etcd Version: 3.6.1
```

# 启动所有etcd节点
```
sudo systemctl daemon-reload
sudo systemctl enable --now etcd
```

# 验证集群状态
```
etcdctl --endpoints="http://192.168.149.230:2379,http://192.168.149.231:2379,http://192.168.149.232:2379" \
        endpoint status --write-out=table
        +-----------------------------+------------------+---------+-----------------+---------+--------+-----------------------+-------+-----------+------------+-----------+------------+--------------------+--------+--------------------------+-------------------+
        |          ENDPOINT           |        ID        | VERSION | STORAGE VERSION | DB SIZE | IN USE | PERCENTAGE NOT IN USE | QUOTA | IS LEADER | IS LEARNER | RAFT TERM | RAFT INDEX | RAFT APPLIED INDEX | ERRORS | DOWNGRADE TARGET VERSION | DOWNGRADE ENABLED |
        +-----------------------------+------------------+---------+-----------------+---------+--------+-----------------------+-------+-----------+------------+-----------+------------+--------------------+--------+--------------------------+-------------------+
        | http://192.168.149.230:2379 | 7f428cfcba0836bd |   3.6.1 |           3.6.0 |   20 kB |  16 kB |                   20% |   0 B |     false |      false |         3 |         13 |                 13 |        |                          |             false |
        | http://192.168.149.231:2379 | 82ebb4f0857ed135 |   3.6.1 |           3.6.0 |   25 kB |  25 kB |                    0% |   0 B |     false |      false |         3 |         13 |                 13 |        |                          |             false |
        | http://192.168.149.232:2379 | c1a36443b24b0e35 |   3.6.1 |           3.6.0 |   25 kB |  16 kB |                   34% |   0 B |      true |      false |         3 |         13 |                 13 |        |                          |             false |
        +-----------------------------+------------------+---------+-----------------+---------+--------+-----------------------+-------+-----------+------------+-----------+------------+--------------------+--------+--------------------------+-------------------+

# 设置环境变量（可加入 ~/.bashrc 永久生效）
export ETCDCTL_ENDPOINTS="http://192.168.149.230:2379,http://192.168.149.231:2379,http://192.168.149.232:2379"
export ETCDCTL_API=3
etcdctl endpoint status --write-out=table
```

# 安装 Patroni
```
yum install -y python3-psycopg2
pip3 install python-etcd patroni[etcd3]
ln -s /usr/local/bin/patroni /usr/bin/patroni

# 生成基础patroni.yml模板
sudo -u postgres \
  PATRONI_POSTGRESQL_BIN_DIR=/usr/pgsql-17/bin \
  PATRONI_POSTGRESQL_DATA_DIR=/var/lib/pgsql/17/data \
  patroni --generate-sample-config > /etc/patroni.yml
```


# 生成etcd配置文件 /etc/etcd/etcd.conf
```
bash etcd.sh
```

# 生成patroni配置文件 /etc/patroni.yml
```
bash generate_patroni_config.sh 192.168.149.230 192.168.149.231 192.168.149.232
```

# 启动集群

清理残留状态
```
# 1. 停止所有 Patroni 服务
sudo systemctl stop patroni

# 2. 清理 ETCD 中残留的集群数据（在任意 ETCD 节点执行）
etcdctl get "" --prefix

/service/pg-prod-cluster/config
{"loop_wait":10,"retry_timeout":10,"ttl":30,"postgresql":{"parameters":{"hot_standby":"on","max_connections":200,"max_locks_per_transaction":64,"max_prepared_transactions":0,"max_replication_slots":10,"max_wal_senders":10,"max_worker_processes":8,"track_commit_timestamp":"off","wal_keep_size":"1GB","wal_level":"replica","wal_log_hints":"on","shared_buffers":"1GB","effective_cache_size":"3GB","work_mem":"16MB"},"use_pg_rewind":true,"use_slots":true}}
/service/pg-prod-cluster/initialize
7518976730489207398
/service/pg-prod-cluster/status
{"optime":22290352,"slots":{"db1":22290352},"retain_slots":["db1"]}

etcdctl del --prefix /service/pg-prod-cluster

# 3. 删除 PostgreSQL 数据目录（所有节点执行）
sudo rm -rf /var/lib/pgsql/17/data/*
```

启动第一个节点
```
# 在第一个节点启动（会自动执行initdb）
systemctl start patroni
```

检查初始化状态
```
patronictl -c /etc/patroni.yml list
+ Cluster: pg-prod-cluster (7518981908086242084) --+-----------+----------------------+
| Member | Host            | Role   | State   | TL | Lag in MB | Tags                 |
+--------+-----------------+--------+---------+----+-----------+----------------------+
| db1    | 192.168.149.230 | Leader | running |  1 |           | clonefrom: true      |
|        |                 |        |         |    |           | failover_priority: 1 |
+--------+-----------------+--------+---------+----+-----------+----------------------+
```

测试数据库是否联通
```
sudo -u postgres  psql -h 192.168.149.230
Password for user postgres:
psql (17.5)

postgres=#
```

其他节点执行
```
systemctl start patroni
```







