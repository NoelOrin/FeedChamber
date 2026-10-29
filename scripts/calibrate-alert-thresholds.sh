#!/usr/bin/env bash
# 按真实流量校准可观测性阈值（change 2026-10-28-observability-gaps / 16.2）。
#
# 背景：`High5xxErrorRate` / `HighP99Latency` / `DBPoolSaturation` 三条告警的阈值
# （5% / 1s / 90%）是经验值，没有用真实流量回归过。真实分位数无法在本地合成——
# 用相对基线替换会得到"永远不响"或"一直响"的假规则。
#
# 本脚本的作用：在**已有真实流量的环境**里，把当前分位数打出来，便于人工判断阈值
# 该收紧还是放宽。它只读不写，不改规则文件。
#
# 用法：
#   ./scripts/calibrate-alert-thresholds.sh                      # 用 http://localhost:9090
#   PROM_URL=http://prom.internal:9090 ./scripts/calibrate-alert-thresholds.sh
#   WINDOW=7d ./scripts/calibrate-alert-thresholds.sh            # 拉长观察窗口
#
# 输出每一行都带"怎么读"的注释，直接照结论调 deploy/prometheus/rules/orinforum.yml。

set -euo pipefail

PROM_URL="${PROM_URL:-http://localhost:9090}"
WINDOW="${WINDOW:-24h}"

if ! command -v jq >/dev/null 2>&1; then
  echo "需要 jq：brew install jq / apt-get install jq" >&2
  exit 1
fi

if ! curl -fsS "${PROM_URL}/-/healthy" >/dev/null 2>&1; then
  echo "连不上 Prometheus：${PROM_URL}" >&2
  echo "确认端口转发或 PROM_URL 是否正确。" >&2
  exit 1
fi

# query_instant <promql> <label>
query_instant() {
  local q="$1" label="$2"
  local out
  out=$(curl -fsS --get "${PROM_URL}/api/v1/query" \
        --data-urlencode "query=${q}" | jq -r '.data.result[0].value[1] // "n/a"')
  printf '  %-46s %s\n' "${label}" "${out}"
}

echo "══════════════════════════════════════════════════════════════"
echo " 阈值校准报告  window=${WINDOW}  prom=${PROM_URL}"
echo "══════════════════════════════════════════════════════════════"

echo
echo "── 1) 5xx 错误率（当前告警阈值 5%）───────────────────────────"
query_instant \
  "sum(rate(orinforum_http_requests_total{status=~\"5..\"}[${WINDOW}])) / clamp_min(sum(rate(orinforum_http_requests_total[${WINDOW}])), 0.001)" \
  "窗口内 5xx 占比"
query_instant \
  "sum(rate(orinforum_http_requests_total[${WINDOW}]))" \
  "总请求速率 (req/s)"
cat <<'READ'
  怎么读：健康站点通常 <0.1%。若长窗口值本身就 >1%，说明阈值太松（已在持续错误）
  或统计口径包含了扫描器打出的 404→5xx；若长期为 0，可考虑把 5% 收紧到 1% 以提早发现。
READ

echo
echo "── 2) P99 延迟（当前告警阈值 1s）─────────────────────────────"
for q in 0.50 0.90 0.99 0.999; do
  query_instant \
    "histogram_quantile(${q}, sum by (le) (rate(orinforum_http_request_duration_seconds_bucket[${WINDOW}])))" \
    "P$(( ${q#0.} ))  延迟 (s)"
done
cat <<'READ'
  怎么读：把 P99 与 P50 对比。P99 若长期显著高于 1s，先看是哪个 handler（按 route 分组）；
  若 P50 都接近 1s，说明 1s 阈值对本部署太低，应按 P99 实测的 2~3 倍设置。
READ

echo
echo "── 3) DB 连接池（当前告警阈值 90% 使用率 / 5min 等待 >10）─────"
query_instant \
  "orinforum_db_pool_in_use / clamp_min(orinforum_db_pool_max_open_connections, 1)" \
  "池使用率"
query_instant "orinforum_db_pool_in_use" "in_use"
query_instant "orinforum_db_pool_max_open_connections" "max_open"
query_instant \
  "increase(orinforum_db_pool_wait_count_total[${WINDOW}])" \
  "窗口内累计等待次数"
cat <<'READ'
  怎么读：若"窗口内累计等待次数"在长窗口下仍为 0，说明池容量充足，90% 阈值可维持；
  若使用率长期 >70% 且伴随等待，应优先扩容 DB_MAX_OPEN_CONNS 而不是抬高阈值。
READ

echo
echo "── 4) NATS / JetStream（16.1 新增指标，同样需要真实基线）─────"
query_instant "gnatsd_varz_slow_consumers" "慢消费者"
query_instant \
  "gnatsd_jetstream_storage_used_bytes / clamp_min(gnatsd_jetstream_storage_max_bytes, 1)" \
  "JetStream 存储水位"
query_instant "gnatsd_varz_connections" "NATS 连接数"
cat <<'READ'
  怎么读：存储水位告警设在 85%。若长期 <10%，说明当前 max_bytes 远超需求；
  接近 85% 则应在写满（会中断 KV 写入与事件发布）前扩容 volume 或调 max_bytes。
READ

echo
echo "══════════════════════════════════════════════════════════════"
echo "结论写回：deploy/prometheus/rules/orinforum.yml 的 orinforum-http / orinforum-business 组"
echo "══════════════════════════════════════════════════════════════"
