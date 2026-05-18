#!/bin/bash
# Benchmark: direct DeepSeek vs MagicGateway proxy
#
# Usage:
#   ./bench/run.sh <deepseek-api-key> <proxy-api-key> [gateway-url] [iterations]
#
# Example:
#   ./bench/run.sh sk-xxx mg-xxx http://localhost:8080 10
#
# Prerequisites: Go (for bench.go) or set BENCH_BIN to a pre-built binary.

set -euo pipefail

DEEPSEEK_KEY="${1:?Usage: $0 <deepseek-key> <proxy-key> [gateway-url] [iterations]}"
PROXY_KEY="${2:?}"
GATEWAY_URL="${3:-http://localhost:8080}"
ITERATIONS="${4:-10}"

BENCH_DIR="$(cd "$(dirname "$0")" && pwd)"
BENCH_BIN="${BENCH_BIN:-}"  # pre-built binary path, or build from source

# ---- Build bench binary if needed ----
if [ -z "$BENCH_BIN" ]; then
    BENCH_BIN="$BENCH_DIR/bench"
    echo "=== Building bench binary ==="
    cd "$BENCH_DIR"
    go build -o "$BENCH_BIN" bench.go
    echo ""
fi

# ---- Warm-up (excluded from results) ----
echo "=== Warm-up (2 rounds, excluded from results) ==="
for i in 1 2; do
    "$BENCH_BIN" direct "$DEEPSEEK_KEY" > /dev/null 2>&1 || true
    "$BENCH_BIN" proxy "$PROXY_KEY" "$GATEWAY_URL" > /dev/null 2>&1 || true
done
echo "Warm-up done."
echo ""

# ---- Benchmark rounds ----
echo "=== Running $ITERATIONS iterations ==="
echo ""

DIRECT_TTFT=()
DIRECT_TOTAL=()
DIRECT_TPS=()
PROXY_TTFT=()
PROXY_TOTAL=()
PROXY_TPS=()

for i in $(seq 1 $ITERATIONS); do
    # Alternate order each round to control for time-of-day variance
    if [ $((i % 2)) -eq 1 ]; then
        d=$("$BENCH_BIN" direct "$DEEPSEEK_KEY" 2>/dev/null)
        p=$("$BENCH_BIN" proxy "$PROXY_KEY" "$GATEWAY_URL" 2>/dev/null)
    else
        p=$("$BENCH_BIN" proxy "$PROXY_KEY" "$GATEWAY_URL" 2>/dev/null)
        d=$("$BENCH_BIN" direct "$DEEPSEEK_KEY" 2>/dev/null)
    fi

    # Parse JSON results
    d_ttft=$(echo "$d" | python3 -c "import sys,json; print(json.load(sys.stdin).get('ttft_ms',0))" 2>/dev/null || echo 0)
    d_total=$(echo "$d" | python3 -c "import sys,json; print(json.load(sys.stdin).get('total_time_ms',0))" 2>/dev/null || echo 0)
    d_tps=$(echo "$d" | python3 -c "import sys,json; print(json.load(sys.stdin).get('tokens_per_second',0))" 2>/dev/null || echo 0)
    p_ttft=$(echo "$p" | python3 -c "import sys,json; print(json.load(sys.stdin).get('ttft_ms',0))" 2>/dev/null || echo 0)
    p_total=$(echo "$p" | python3 -c "import sys,json; print(json.load(sys.stdin).get('total_time_ms',0))" 2>/dev/null || echo 0)
    p_tps=$(echo "$p" | python3 -c "import sys,json; print(json.load(sys.stdin).get('tokens_per_second',0))" 2>/dev/null || echo 0)

    DIRECT_TTFT+=("$d_ttft")
    DIRECT_TOTAL+=("$d_total")
    DIRECT_TPS+=("$d_tps")
    PROXY_TTFT+=("$p_ttft")
    PROXY_TOTAL+=("$p_total")
    PROXY_TPS+=("$p_tps")

    overhead=$(python3 -c "print(round($p_total - $d_total, 1))" 2>/dev/null || echo "N/A")
    printf "  Round %2d  |  direct: %7.0f ms (TTFT %5.0f ms, %5.0f tok/s)  |  proxy: %7.0f ms (TTFT %5.0f ms, %5.0f tok/s)  |  overhead: %s ms\n" \
        "$i" "$d_total" "$d_ttft" "$d_tps" "$p_total" "$p_ttft" "$p_tps" "$overhead"
done

echo ""

# ---- Aggregate statistics ----
calc_stats() {
    python3 -c "
import sys, json
vals = [float(x) for x in sys.argv[1:]]
vals.sort()
n = len(vals)
mean = sum(vals) / n
median = vals[n//2]
p95 = vals[int(n*0.95)] if n >= 20 else vals[-1]
print(f'{mean:.0f} / {median:.0f} / {p95:.0f}')
" "$@"
}

echo "=== Results ($ITERATIONS iterations) ==="
echo ""
printf "%-18s  %16s  %16s  %16s\n" "" "Direct" "Proxy" "Overhead"
printf "%-18s  %16s  %16s  %16s\n" "" "------" "-----" "--------"

d_ttft_stat=$(calc_stats "${DIRECT_TTFT[@]}")
p_ttft_stat=$(calc_stats "${PROXY_TTFT[@]}")
d_total_stat=$(calc_stats "${DIRECT_TOTAL[@]}")
p_total_stat=$(calc_stats "${PROXY_TOTAL[@]}")
d_tps_stat=$(calc_stats "${DIRECT_TPS[@]}")
p_tps_stat=$(calc_stats "${PROXY_TPS[@]}")

# Extract means for overhead calculation
d_ttft_mean=$(echo "$d_ttft_stat" | awk -F' / ' '{print $1}')
p_ttft_mean=$(echo "$p_ttft_stat" | awk -F' / ' '{print $1}')
d_total_mean=$(echo "$d_total_stat" | awk -F' / ' '{print $1}')
p_total_mean=$(echo "$p_total_stat" | awk -F' / ' '{print $1}')

ttft_oh=$(python3 -c "print(round($p_ttft_mean - $d_ttft_mean, 1))")
total_oh=$(python3 -c "print(round($p_total_mean - $d_total_mean, 1))")
pct=$(python3 -c "print(round(($p_total_mean - $d_total_mean) / $d_total_mean * 100, 1))")

printf "%-18s  %16s  %16s  %16s\n" "TTFT (avg/m/p95)" "$d_ttft_stat ms" "$p_ttft_stat ms" "+${ttft_oh} ms"
printf "%-18s  %16s  %16s  %16s\n" "Total (avg/m/p95)" "$d_total_stat ms" "$p_total_stat ms" "+${total_oh} ms"
printf "%-18s  %16s  %16s  %16s\n" "Tok/s (avg/m/p95)" "$d_tps_stat" "$p_tps_stat" ""
echo ""
echo "Average overhead: +${total_oh} ms (+${pct}%)"
echo ""

# ---- Diagnosis ----
echo "=== Diagnosis ==="
if [ "$(python3 -c "print(1 if $pct < 5 else 0)")" = "1" ]; then
    echo "Overhead < 5% — gateway adds negligible latency. User concern likely due to:"
    echo "  1. Network distance (client → gateway vs client → DeepSeek)"
    echo "  2. Different prompt/token count (not comparing like-for-like)"
    echo "  3. Cold start (first request after idle)"
elif [ "$(python3 -c "print(1 if $pct < 15 else 0)")" = "1" ]; then
    echo "Overhead 5-15% — noticeable but expected for proxy. Check:"
    echo "  1. Is the gateway on the same LAN as the client?"
    echo "  2. SQLite DB size (large usage_logs table may slow API key lookup)"
else
    echo "Overhead > 15% — investigate:"
    echo "  1. Network latency: ping the gateway server from the client"
    echo "  2. Gateway server load: CPU/memory usage"
    echo "  3. DNS resolution time on the gateway"
    echo "  4. Check 'duration_ms' in gateway usage_logs vs client-side timing"
fi
