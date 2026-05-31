#!/bin/bash
# Plomvix Smoke Test — All Sprints (1–6)
# Run from project root: bash smoke_test.sh

set -euo pipefail

SERVER_PID=""
ROCKSDB_LIB="/home/raj/project/plomvix/.rocksdb/usr/lib/x86_64-linux-gnu"
export PATH="/usr/local/go/bin:$PATH"
export LD_LIBRARY_PATH="${ROCKSDB_LIB}:${LD_LIBRARY_PATH:-}"

cleanup() {
    if [ -n "$SERVER_PID" ] && kill -0 "$SERVER_PID" 2>/dev/null; then
        kill -SIGTERM "$SERVER_PID" 2>/dev/null || true
        wait "$SERVER_PID" 2>/dev/null || true
    fi
    rm -f /tmp/plomvix_bad.yaml
}
trap cleanup EXIT

PASS=0
FAIL=0
pass() { PASS=$((PASS + 1)); echo "  PASS: $1"; }
fail() { FAIL=$((FAIL + 1)); echo "  FAIL: $1"; }

echo "================================================"
echo " Plomvix Smoke Test — Sprints 1–6"
echo " $(date)"
echo "================================================"

# ── SPRINT 1: Build & Core ──────────────────────────────────────
echo ""
echo "── Sprint 1: Build & Core ──"

echo "Step 1: Clean build"
go mod tidy > /dev/null 2>&1
make vet 2>&1 && pass "vet" || fail "vet"
make build 2>&1 && pass "build ($(ls -lh plomvix | awk '{print $5}'))" || fail "build"

echo ""
echo "Step 2: Version flag"
./plomvix --version 2>&1 | grep -q "0.1.0" && pass "version 0.1.0" || fail "version"

echo ""
echo "Step 3: Bad config rejection"
cat > /tmp/plomvix_bad.yaml << 'BADCFG'
env: development
server:
  host: ""
  port: 0
  read_timeout: 30
  write_timeout: 30
  idle_timeout: 60
storage:
  data_dir: ./data
  wal_flush_threshold: 0
  hot_tier_max_size: 10737418240
  retention_days: 30
compression:
  hot_tier: snappy
  cold_tier: zstd
indexing:
  auto_index_timestamp: true
auth:
  default_admin_username: admin
  default_admin_password: changeme
  jwt_secret: plomvix-change-in-prod
  jwt_expiry_seconds: 3600
  api_key_length: 32
logging:
  level: info
  format: pretty
BADCFG
set +e
./plomvix --config /tmp/plomvix_bad.yaml > /dev/null 2>&1
BAD_EXIT=$?
set -e
[ "$BAD_EXIT" -ne 0 ] && pass "bad config rejected (exit $BAD_EXIT)" || fail "bad config should have failed"

echo ""
echo "Step 4: Boot server"
./plomvix > /tmp/plomvix_smoke.log 2>&1 &
SERVER_PID=$!
sleep 3
curl -sf http://localhost:8080/health > /dev/null && pass "health endpoint reachable" || fail "health endpoint unreachable"

echo ""
echo "Step 5: X-Request-ID header"
REQ_ID=$(curl -sI http://localhost:8080/health | grep -i "x-request-id" | awk '{print $2}' | tr -d '\r\n')
[ "${#REQ_ID}" -eq 36 ] && pass "UUID format ($REQ_ID)" || fail "request ID length ${#REQ_ID} != 36"

# ── SPRINT 2: Auth ───────────────────────────────────────────────
echo ""
echo "── Sprint 2: Auth ──"

echo "Step 6: Login"
TOKEN=$(curl -sf -X POST http://localhost:8080/auth/login \
    -H "Content-Type: application/json" \
    -d '{"username":"admin","password":"changeme"}' \
    | python3 -c "import sys,json; print(json.load(sys.stdin)['data']['token'])")
[ -n "$TOKEN" ] && pass "JWT acquired (${TOKEN:0:20}...)" || fail "login failed"

echo ""
echo "Step 7: Login with wrong password"
STATUS=$(curl -s -o /dev/null -w "%{http_code}" -X POST http://localhost:8080/auth/login \
    -H "Content-Type: application/json" \
    -d '{"username":"admin","password":"wrong"}')
[ "$STATUS" -eq 401 ] && pass "wrong password → 401" || fail "wrong password got $STATUS"

echo ""
echo "Step 8: Protected endpoint without auth"
STATUS=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/query/logs)
[ "$STATUS" -eq 401 ] && pass "no auth → 401" || fail "no auth got $STATUS"

# ── SPRINT 5: Ingestion ──────────────────────────────────────────
echo ""
echo "── Sprint 5: Ingestion API ──"

echo "Step 9: POST /ingest/logs"
STATUS=$(curl -s -o /tmp/plomvix_resp.json -w "%{http_code}" \
    -X POST http://localhost:8080/ingest/logs \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d '{"records":[{"level":"info","message":"smoke log","fields":{"source":"test"}}]}')
INGESTED=$(python3 -c "import json; print(json.load(open('/tmp/plomvix_resp.json'))['data']['ingested'])")
[ "$STATUS" -eq 201 ] && [ "$INGESTED" -eq 1 ] && pass "201, ingested=1" || fail "log ingest: $STATUS, ingested=$INGESTED"

echo ""
echo "Step 10: POST /ingest/metrics"
STATUS=$(curl -s -o /dev/null -w "%{http_code}" \
    -X POST http://localhost:8080/ingest/metrics \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d '{"records":[{"name":"cpu.usage","value":87.5,"tags":{"host":"svr1"}}]}')
[ "$STATUS" -eq 201 ] && pass "201" || fail "metric ingest: $STATUS"

echo ""
echo "Step 11: POST /ingest/json"
STATUS=$(curl -s -o /dev/null -w "%{http_code}" \
    -X POST http://localhost:8080/ingest/json \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d '{"records":[{"data":{"event":"smoke_test","ok":true,"amount":99.99}}]}')
[ "$STATUS" -eq 201 ] && pass "201" || fail "json ingest: $STATUS"

echo ""
echo "Step 12: POST /ingest/kv"
STATUS=$(curl -s -o /dev/null -w "%{http_code}" \
    -X POST http://localhost:8080/ingest/kv \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d '{"records":[{"key":"smoke:test:key","value":"hello_plomvix"}]}')
[ "$STATUS" -eq 201 ] && pass "201" || fail "kv ingest: $STATUS"

echo ""
echo "Step 13: Batch ingest 3 logs"
RESP=$(curl -sf -X POST http://localhost:8080/ingest/logs \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d '{"records":[{"level":"info","message":"a"},{"level":"warn","message":"b"},{"level":"error","message":"c"}]}')
INGESTED=$(echo "$RESP" | python3 -c "import sys,json; print(json.load(sys.stdin)['data']['ingested'])")
[ "$INGESTED" -eq 3 ] && pass "batch 3 → ingested=3" || fail "batch: ingested=$INGESTED"

echo ""
echo "Step 14: Empty records → 400"
STATUS=$(curl -s -o /dev/null -w "%{http_code}" \
    -X POST http://localhost:8080/ingest/logs \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d '{"records":[]}')
[ "$STATUS" -eq 400 ] && pass "empty records → 400" || fail "empty records got $STATUS"

# ── SPRINT 6: Query Engine ───────────────────────────────────────
echo ""
echo "── Sprint 6: Query Engine ──"

echo "Step 15: GET /query/logs"
RESP=$(curl -sf "http://localhost:8080/query/logs" -H "Authorization: Bearer $TOKEN")
TOTAL=$(echo "$RESP" | python3 -c "import sys,json; print(json.load(sys.stdin)['data']['total'])")
[ "$TOTAL" -ge 4 ] && pass "total=$TOTAL (all ingested logs)" || fail "query logs total=$TOTAL"

echo ""
echo "Step 16: GET /query/logs with filter"
RESP=$(curl -sf "http://localhost:8080/query/logs?filter=level%3Dinfo" -H "Authorization: Bearer $TOKEN")
TOTAL=$(echo "$RESP" | python3 -c "import sys,json; print(json.load(sys.stdin)['data']['total'])")
[ "$TOTAL" -ge 2 ] && pass "filter level=info → $TOTAL records" || fail "filter returned $TOTAL"

echo ""
echo "Step 17: Query metrics with numeric filter"
RESP=$(curl -sf "http://localhost:8080/query/metrics?filter=value%3E50" -H "Authorization: Bearer $TOKEN")
TOTAL=$(echo "$RESP" | python3 -c "import sys,json; print(json.load(sys.stdin)['data']['total'])")
[ "$TOTAL" -ge 1 ] && pass "numeric filter value>50 → $TOTAL" || fail "numeric filter returned $TOTAL"

echo ""
echo "Step 18: GET /query/metrics"
RESP=$(curl -sf "http://localhost:8080/query/metrics?name=cpu.usage" -H "Authorization: Bearer $TOKEN")
TOTAL=$(echo "$RESP" | python3 -c "import sys,json; print(json.load(sys.stdin)['data']['total'])")
[ "$TOTAL" -ge 1 ] && pass "metrics name=cpu.usage → $TOTAL" || fail "metrics query returned $TOTAL"

echo ""
echo "Step 19: GET /query/kv/{key} — found"
RESP=$(curl -sf "http://localhost:8080/query/kv/smoke:test:key" -H "Authorization: Bearer $TOKEN")
COUNT=$(echo "$RESP" | python3 -c "import sys,json; print(json.load(sys.stdin)['data']['count'])")
[ "$COUNT" -eq 1 ] && pass "KV found, count=1" || fail "KV found count=$COUNT"

echo ""
echo "Step 20: GET /query/kv/{key} — not found"
RESP=$(curl -sf "http://localhost:8080/query/kv/nonexistent" -H "Authorization: Bearer $TOKEN")
COUNT=$(echo "$RESP" | python3 -c "import sys,json; print(json.load(sys.stdin)['data']['count'])")
[ "$COUNT" -eq 0 ] && pass "KV not found, count=0" || fail "KV not found count=$COUNT"

echo ""
echo "Step 21: GET /query/schema/logs"
RESP=$(curl -sf "http://localhost:8080/query/schema/logs" -H "Authorization: Bearer $TOKEN")
echo "$RESP" | python3 -c "import sys,json; d=json.load(sys.stdin)['data']; assert 'level' in str(d.get('fields',{}))" 2>/dev/null \
    && pass "schema has level field" || fail "schema missing level field"

echo ""
echo "Step 22: GET /query/schema/metrics"
RESP=$(curl -sf "http://localhost:8080/query/schema/metrics" -H "Authorization: Bearer $TOKEN")
echo "$RESP" | python3 -c "import sys,json; d=json.load(sys.stdin)['data']; assert 'name' in str(d.get('fields',{}))" 2>/dev/null \
    && pass "metrics schema has name field" || fail "metrics schema missing name"

echo ""
echo "Step 23: Invalid filter → 400"
STATUS=$(curl -s -o /dev/null -w "%{http_code}" \
    "http://localhost:8080/query/logs?filter=noop" \
    -H "Authorization: Bearer $TOKEN")
[ "$STATUS" -eq 400 ] && pass "invalid filter → 400" || fail "invalid filter got $STATUS"

echo ""
echo "Step 24: Invalid schema type → 400"
STATUS=$(curl -s -o /dev/null -w "%{http_code}" \
    "http://localhost:8080/query/schema/invalid" \
    -H "Authorization: Bearer $TOKEN")
[ "$STATUS" -eq 400 ] && pass "invalid schema type → 400" || fail "invalid schema got $STATUS"

echo ""
echo "Step 25: Pagination — limit + offset"
RESP=$(curl -sf "http://localhost:8080/query/logs?limit=2&offset=1" -H "Authorization: Bearer $TOKEN")
COUNT=$(echo "$RESP" | python3 -c "import sys,json; d=json.load(sys.stdin)['data']; print(d['count'])")
TOTAL=$(echo "$RESP" | python3 -c "import sys,json; d=json.load(sys.stdin)['data']; print(d['total'])")
[ "$COUNT" -le 2 ] && [ "$TOTAL" -ge 2 ] && pass "limit=2 offset=1 → count=$COUNT" || fail "pagination count=$COUNT total=$TOTAL"

# ── Health Stats ──────────────────────────────────────────────────
echo ""
echo "── Health & Stats ──"

echo "Step 26: Health shows total_data_writes > 0"
HEALTH=$(curl -sf http://localhost:8080/health)
TOTAL_WRITES=$(echo "$HEALTH" | python3 -c "import sys,json; print(json.load(sys.stdin)['data']['hot']['total_data_writes'])")
WAL_ENTRIES=$(echo "$HEALTH" | python3 -c "import sys,json; print(json.load(sys.stdin)['data']['wal']['total_entries'])")
[ "$TOTAL_WRITES" -gt 0 ] && pass "total_data_writes=$TOTAL_WRITES" || fail "total_data_writes=$TOTAL_WRITES"
echo "  WAL entries: $WAL_ENTRIES | Hot writes: $TOTAL_WRITES"

# ── Graceful Shutdown ─────────────────────────────────────────────
echo ""
echo "── Shutdown ──"

echo "Step 27: Graceful shutdown"
kill -SIGTERM "$SERVER_PID"
wait "$SERVER_PID" 2>/dev/null
SHUTDOWN_CODE=$?
SERVER_PID=""
[ "$SHUTDOWN_CODE" -eq 0 ] && pass "clean shutdown (exit $SHUTDOWN_CODE)" || fail "shutdown exit $SHUTDOWN_CODE"

# ── Full Test Suite ───────────────────────────────────────────────
echo ""
echo "── Test Suite ──"

echo "Step 28: Run all tests"
make test 2>&1 | grep -E "^(ok|FAIL|---)" || true
make test > /dev/null 2>&1 && pass "all tests pass" || fail "test failures"

# ── Results ───────────────────────────────────────────────────────
echo ""
echo "================================================"
echo "  SMOKE TEST COMPLETE"
echo "  Passed: $PASS  |  Failed: $FAIL"
echo "================================================"

if [ "$FAIL" -gt 0 ]; then
    echo ""
    echo "!!! $FAIL TEST(S) FAILED !!!"
    exit 1
fi

echo "All $PASS smoketest checks passed."
