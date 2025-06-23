# 定义基础参数
ETCD_CLUSTER_TOKEN="etcd-cluster-token-$(date +%s)"  # 自动生成唯一token
DATA_DIR="/var/lib/etcd/default.etcd"
AUTO_COMPACTION_RETENTION="24"  # 自动压缩保留24小时

# 定义节点IP（修改为您的实际IP）
declare -A NODES=(
  [etcd1]="192.168.149.230"
  [etcd2]="192.168.149.231"
  [etcd3]="192.168.149.232"
)

# 生成初始集群字符串
INITIAL_CLUSTER=""
for node in "${!NODES[@]}"; do
  INITIAL_CLUSTER+="${node}=http://${NODES[$node]}:2380,"
done
INITIAL_CLUSTER=${INITIAL_CLUSTER%,}  # 移除末尾逗号

# 为每个节点生成配置
for node in "${!NODES[@]}"; do
  cat > etcd-${node}.conf <<EOF
# [member]
ETCD_NAME="${node}"
ETCD_DATA_DIR="${DATA_DIR}"
ETCD_LISTEN_PEER_URLS="http://${NODES[$node]}:2380"
ETCD_LISTEN_CLIENT_URLS="http://${NODES[$node]}:2379,http://127.0.0.1:2379"

# [cluster]
ETCD_INITIAL_ADVERTISE_PEER_URLS="http://${NODES[$node]}:2380"
ETCD_INITIAL_CLUSTER="${INITIAL_CLUSTER}"
ETCD_INITIAL_CLUSTER_TOKEN="${ETCD_CLUSTER_TOKEN}"
ETCD_INITIAL_CLUSTER_STATE="new"
ETCD_ADVERTISE_CLIENT_URLS="http://${NODES[$node]}:2379"

# [tuning]
ETCD_AUTO_COMPACTION_RETENTION="${AUTO_COMPACTION_RETENTION}"
EOF

  echo "Generated config file: etcd-${node}.conf"
done

echo "All etcd cluster config files generated successfully."
echo "Cluster token: ${ETCD_CLUSTER_TOKEN}"
