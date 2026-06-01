#!/bin/bash
# Plomvix Smoke Test — All Sprints (1–13 Patch)
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
echo " Plomvix Smoke Test — Sprints 1–11"
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
PLOMVIX_STORAGE_RETENTION_DAYS=0 ./plomvix > /tmp/plomvix_smoke.log 2>&1 &
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

# ── SPRINT 7: Cold Tier ──────────────────────────────────────────
echo ""
echo "── Sprint 7: Cold Tier ──"

echo "Step 15: Health includes cold stats"
HEALTH=$(curl -sf http://localhost:8080/health)
echo "$HEALTH" | python3 -c "import sys,json; d=json.load(sys.stdin)['data']; assert 'cold' in d" 2>/dev/null \
    && pass "cold block in health" || fail "cold block missing"

echo ""
echo "Step 16: Manual tier flush with retention_days=0"
RESP=$(curl -sf -X POST http://localhost:8080/admin/tier/flush \
    -H "Authorization: Bearer $TOKEN")
MOVED=$(echo "$RESP" | python3 -c "import sys,json; print(json.load(sys.stdin)['data']['records_moved'])")
FLUSH_FILES=$(echo "$RESP" | python3 -c "import sys,json; print(json.load(sys.stdin)['data']['parquet_files'])")
[ "$MOVED" -ge 4 ] && pass "tier flush: $MOVED records moved" || fail "tier flush: $MOVED records moved"
[ "$FLUSH_FILES" -ge 1 ] && pass "parquet files=$FLUSH_FILES" || fail "parquet files=$FLUSH_FILES"

echo ""
echo "Step 17: Query after cold tier flush (returns from cold tier)"
RESP=$(curl -sf "http://localhost:8080/query/logs" -H "Authorization: Bearer $TOKEN")
TOTAL=$(echo "$RESP" | python3 -c "import sys,json; print(json.load(sys.stdin)['data']['total'])")
[ "$TOTAL" -ge 4 ] && pass "query after flush total=$TOTAL" || fail "query after flush total=$TOTAL"

echo ""
echo "Step 18: Tier flush requires auth"
STATUS=$(curl -s -o /dev/null -w "%{http_code}" -X POST http://localhost:8080/admin/tier/flush)
[ "$STATUS" -eq 401 ] && pass "tier flush no auth → 401" || fail "tier flush no auth got $STATUS"

echo ""
echo "Step 19: KV records are not tiered (still queryable after flush)"
RESP=$(curl -sf "http://localhost:8080/query/kv/smoke:test:key" -H "Authorization: Bearer $TOKEN")
COUNT=$(echo "$RESP" | python3 -c "import sys,json; print(json.load(sys.stdin)['data']['count'])")
[ "$COUNT" -eq 1 ] && pass "KV still present after flush, count=1" || fail "KV after flush count=$COUNT"

# ── SPRINT 8: Multi-Format Parsers ────────────────────────────────
echo ""
echo "── Sprint 8: Multi-Format Parsers ──"

echo "Step 20: JSON bare array ingest"
STATUS=$(curl -s -o /dev/null -w "%{http_code}" \
    -X POST http://localhost:8080/ingest/logs \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d '[{"level":"info","message":"bare array test"}]')
[ "$STATUS" -eq 201 ] && pass "JSON bare array → 201" || fail "JSON bare array got $STATUS"

echo ""
echo "Step 21: CSV ingest"
RESP=$(curl -sf -X POST http://localhost:8080/ingest/logs \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: text/csv" \
    --data-binary $'level,message,host\ninfo,started,web-01\nwarn,highmem,web-02')
INGESTED=$(echo "$RESP" | python3 -c "import sys,json; print(json.load(sys.stdin)['data']['ingested'])")
[ "$INGESTED" -eq 2 ] && pass "CSV ingested=$INGESTED" || fail "CSV ingested=$INGESTED"

echo ""
echo "Step 22: Logfmt ingest"
STATUS=$(curl -s -o /dev/null -w "%{http_code}" \
    -X POST http://localhost:8080/ingest/logs \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: text/x-logfmt" \
    --data-binary $'level=info msg="logfmt test" host=web-01')
[ "$STATUS" -eq 201 ] && pass "Logfmt → 201" || fail "Logfmt got $STATUS"

echo ""
echo "Step 23: Syslog ingest"
STATUS=$(curl -s -o /dev/null -w "%{http_code}" \
    -X POST http://localhost:8080/ingest/logs \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/x-syslog" \
    --data-binary '<34>1 2024-01-15T10:30:00Z web-01 app 1 - - syslog test')
[ "$STATUS" -eq 201 ] && pass "Syslog → 201" || fail "Syslog got $STATUS"

echo ""
echo "Step 24: Empty body → 400"
STATUS=$(curl -s -o /dev/null -w "%{http_code}" \
    -X POST http://localhost:8080/ingest/logs \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d '')
[ "$STATUS" -eq 400 ] && pass "empty body → 400" || fail "empty body got $STATUS"

echo ""
echo "Step 25: Header-only CSV → 400"
STATUS=$(curl -s -o /dev/null -w "%{http_code}" \
    -X POST http://localhost:8080/ingest/logs \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: text/csv" \
    --data-binary $'level,message\n')
[ "$STATUS" -eq 400 ] && pass "header-only CSV → 400" || fail "header-only CSV got $STATUS"

echo ""
echo "Step 26: Unsupported format on metrics → 400"
STATUS=$(curl -s -o /dev/null -w "%{http_code}" \
    -X POST http://localhost:8080/ingest/metrics \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: text/csv" \
    --data-binary $'name,value\ncpu,1')
[ "$STATUS" -eq 400 ] && pass "CSV on metrics → 400" || fail "CSV on metrics got $STATUS"

echo ""
echo "Step 27: Query includes multi-format records"
RESP=$(curl -sf "http://localhost:8080/query/logs" -H "Authorization: Bearer $TOKEN")
TOTAL=$(echo "$RESP" | python3 -c "import sys,json; print(json.load(sys.stdin)['data']['total'])")
[ "$TOTAL" -ge 9 ] && pass "query total=$TOTAL (all formats)" || fail "query total=$TOTAL"

# ── SPRINT 6: Query Engine ───────────────────────────────────────
echo ""
echo "── Sprint 6: Query Engine ──"

echo "Step 28: GET /query/logs"
RESP=$(curl -sf "http://localhost:8080/query/logs" -H "Authorization: Bearer $TOKEN")
TOTAL=$(echo "$RESP" | python3 -c "import sys,json; print(json.load(sys.stdin)['data']['total'])")
[ "$TOTAL" -ge 4 ] && pass "total=$TOTAL (all ingested logs)" || fail "query logs total=$TOTAL"

echo ""
echo "Step 29: GET /query/logs with filter"
RESP=$(curl -sf "http://localhost:8080/query/logs?filter=level%3Dinfo" -H "Authorization: Bearer $TOKEN")
TOTAL=$(echo "$RESP" | python3 -c "import sys,json; print(json.load(sys.stdin)['data']['total'])")
[ "$TOTAL" -ge 2 ] && pass "filter level=info → $TOTAL records" || fail "filter returned $TOTAL"

echo ""
echo "Step 30: Query metrics with numeric filter"
RESP=$(curl -sf "http://localhost:8080/query/metrics?filter=value%3E50" -H "Authorization: Bearer $TOKEN")
TOTAL=$(echo "$RESP" | python3 -c "import sys,json; print(json.load(sys.stdin)['data']['total'])")
[ "$TOTAL" -ge 1 ] && pass "numeric filter value>50 → $TOTAL" || fail "numeric filter returned $TOTAL"

echo ""
echo "Step 31: GET /query/metrics"
RESP=$(curl -sf "http://localhost:8080/query/metrics?name=cpu.usage" -H "Authorization: Bearer $TOKEN")
TOTAL=$(echo "$RESP" | python3 -c "import sys,json; print(json.load(sys.stdin)['data']['total'])")
[ "$TOTAL" -ge 1 ] && pass "metrics name=cpu.usage → $TOTAL" || fail "metrics query returned $TOTAL"

echo ""
echo "Step 32: GET /query/kv/{key} — found"
RESP=$(curl -sf "http://localhost:8080/query/kv/smoke:test:key" -H "Authorization: Bearer $TOKEN")
COUNT=$(echo "$RESP" | python3 -c "import sys,json; print(json.load(sys.stdin)['data']['count'])")
[ "$COUNT" -eq 1 ] && pass "KV found, count=1" || fail "KV found count=$COUNT"

echo ""
echo "Step 33: GET /query/kv/{key} — not found"
RESP=$(curl -sf "http://localhost:8080/query/kv/nonexistent" -H "Authorization: Bearer $TOKEN")
COUNT=$(echo "$RESP" | python3 -c "import sys,json; print(json.load(sys.stdin)['data']['count'])")
[ "$COUNT" -eq 0 ] && pass "KV not found, count=0" || fail "KV not found count=$COUNT"

echo ""
echo "Step 34: GET /query/schema/logs"
RESP=$(curl -sf "http://localhost:8080/query/schema/logs" -H "Authorization: Bearer $TOKEN")
echo "$RESP" | python3 -c "import sys,json; d=json.load(sys.stdin)['data']; assert 'level' in str(d.get('fields',{}))" 2>/dev/null \
    && pass "schema has level field" || fail "schema missing level field"

echo ""
echo "Step 35: GET /query/schema/metrics"
RESP=$(curl -sf "http://localhost:8080/query/schema/metrics" -H "Authorization: Bearer $TOKEN")
echo "$RESP" | python3 -c "import sys,json; d=json.load(sys.stdin)['data']; assert 'name' in str(d.get('fields',{}))" 2>/dev/null \
    && pass "metrics schema has name field" || fail "metrics schema missing name"

echo ""
echo "Step 36: Invalid filter → 400"
STATUS=$(curl -s -o /dev/null -w "%{http_code}" \
    "http://localhost:8080/query/logs?filter=noop" \
    -H "Authorization: Bearer $TOKEN")
[ "$STATUS" -eq 400 ] && pass "invalid filter → 400" || fail "invalid filter got $STATUS"

echo ""
echo "Step 37: Invalid schema type → 400"
STATUS=$(curl -s -o /dev/null -w "%{http_code}" \
    "http://localhost:8080/query/schema/invalid" \
    -H "Authorization: Bearer $TOKEN")
[ "$STATUS" -eq 400 ] && pass "invalid schema type → 400" || fail "invalid schema got $STATUS"

echo ""
echo "Step 38: Pagination — limit + offset"
RESP=$(curl -sf "http://localhost:8080/query/logs?limit=2&offset=1" -H "Authorization: Bearer $TOKEN")
COUNT=$(echo "$RESP" | python3 -c "import sys,json; d=json.load(sys.stdin)['data']; print(d['count'])")
TOTAL=$(echo "$RESP" | python3 -c "import sys,json; d=json.load(sys.stdin)['data']; print(d['total'])")
[ "$COUNT" -le 2 ] && [ "$TOTAL" -ge 2 ] && pass "limit=2 offset=1 → count=$COUNT" || fail "pagination count=$COUNT total=$TOTAL"

# ── SPRINT 9: Admin APIs & Docs ──────────────────────────────────
echo ""
echo "── Sprint 9: Admin APIs & Docs ──"

echo "Step 39: /openapi.json is public"
STATUS=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/openapi.json)
[ "$STATUS" -eq 200 ] && pass "/openapi.json → 200" || fail "/openapi.json → $STATUS"

echo ""
echo "Step 40: /openapi.json is valid JSON"
curl -sf http://localhost:8080/openapi.json | python3 -m json.tool > /dev/null \
    && pass "spec is valid JSON" || fail "spec is not valid JSON"

echo ""
echo "Step 41: /docs is public"
STATUS=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/docs)
[ "$STATUS" -eq 200 ] && pass "/docs → 200" || fail "/docs → $STATUS"

echo ""
echo "Step 42: /docs contains Stoplight Elements"
curl -sf http://localhost:8080/docs | grep -q "elements-api" \
    && pass "docs page has <elements-api>" || fail "docs page missing <elements-api>"

echo ""
echo "Step 43: GET /admin/stats"
RESP=$(curl -sf http://localhost:8080/admin/stats \
    -H "Authorization: Bearer $TOKEN")
echo "$RESP" | python3 -c "import sys,json; d=json.load(sys.stdin)['data']; assert 'wal' in d; assert 'runtime' in d" 2>/dev/null \
    && pass "/admin/stats returns wal/runtime blocks" || fail "/admin/stats missing blocks"

echo ""
echo "Step 44: GET /admin/info"
RESP=$(curl -sf http://localhost:8080/admin/info \
    -H "Authorization: Bearer $TOKEN")
echo "$RESP" | python3 -c "import sys,json; v=json.load(sys.stdin)['data']['version']; assert v is not None" 2>/dev/null \
    && pass "/admin/info returns version" || fail "/admin/info missing version"

echo ""
echo "Step 45: POST /admin/wal/rotate"
STATUS=$(curl -s -o /dev/null -w "%{http_code}" \
    -X POST http://localhost:8080/admin/wal/rotate \
    -H "Authorization: Bearer $TOKEN")
[ "$STATUS" -eq 200 ] && pass "/admin/wal/rotate → 200" || fail "/admin/wal/rotate → $STATUS"

echo ""
echo "Step 46: GET /admin/schema"
RESP=$(curl -sf http://localhost:8080/admin/schema \
    -H "Authorization: Bearer $TOKEN")
echo "$RESP" | python3 -c "import sys,json; d=json.load(sys.stdin)['data']; assert 'logs' in d" 2>/dev/null \
    && pass "/admin/schema lists data types" || fail "/admin/schema missing logs"

echo ""
echo "Step 47: DELETE /admin/schema/logs"
STATUS=$(curl -s -o /dev/null -w "%{http_code}" \
    -X DELETE http://localhost:8080/admin/schema/logs \
    -H "Authorization: Bearer $TOKEN")
[ "$STATUS" -eq 200 ] && pass "/admin/schema/logs DELETE → 200" || fail "/admin/schema/logs DELETE → $STATUS"

echo ""
echo "Step 48: DELETE /admin/schema/unknown → 400"
STATUS=$(curl -s -o /dev/null -w "%{http_code}" \
    -X DELETE http://localhost:8080/admin/schema/unknown \
    -H "Authorization: Bearer $TOKEN")
[ "$STATUS" -eq 400 ] && pass "/admin/schema/unknown DELETE → 400" || fail "/admin/schema/unknown DELETE → $STATUS"

echo ""
echo "Step 49: Admin endpoints require auth"
STATUS=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/admin/stats)
[ "$STATUS" -eq 401 ] && pass "no auth → 401" || fail "no auth got $STATUS"

# ── SPRINT 11: UI Foundation ──────────────────────────────────────
echo ""
echo "── Sprint 11: UI Foundation ──"

echo "Step 50: GET /app"
STATUS=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/app)
[ "$STATUS" -eq 200 ] && pass "/app → 200" || fail "/app → $STATUS"

echo ""
echo "Step 51: GET /app/explore"
STATUS=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/app/explore)
[ "$STATUS" -eq 200 ] && pass "/app/explore → 200" || fail "/app/explore → $STATUS"

echo ""
echo "Step 52: GET /app/admin"
STATUS=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/app/admin)
[ "$STATUS" -eq 200 ] && pass "/app/admin → 200" || fail "/app/admin → $STATUS"

echo ""
echo "Step 53: /health still returns 200 with UI routes added"
STATUS=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/health)
[ "$STATUS" -eq 200 ] && pass "/health → 200" || fail "/health → $STATUS"

echo ""
echo "Step 54: SPA serves index.html for unknown route"
RESP=$(curl -sf http://localhost:8080/app/unknown-page)
echo "$RESP" | grep -q "Plomvix" && pass "/app/unknown-page → SPA fallback" || fail "/app/unknown-page → no SPA fallback"

echo ""
echo "Step 55: /app returns text/html"
CONTENT_TYPE=$(curl -s -o /dev/null -w "%{content_type}" http://localhost:8080/app)
echo "$CONTENT_TYPE" | grep -q "text/html" && pass "Content-Type text/html" || fail "Content-Type $CONTENT_TYPE"

# ── Health Stats ──────────────────────────────────────────────────
echo ""
echo "── Health & Stats ──"

echo "Step 56: Health shows total_data_writes > 0"
HEALTH=$(curl -sf http://localhost:8080/health)
TOTAL_WRITES=$(echo "$HEALTH" | python3 -c "import sys,json; print(json.load(sys.stdin)['data']['hot']['total_data_writes'])")
WAL_ENTRIES=$(echo "$HEALTH" | python3 -c "import sys,json; print(json.load(sys.stdin)['data']['wal']['total_entries'])")
[ "$TOTAL_WRITES" -gt 0 ] && pass "total_data_writes=$TOTAL_WRITES" || fail "total_data_writes=$TOTAL_WRITES"
echo "  WAL entries: $WAL_ENTRIES | Hot writes: $TOTAL_WRITES"

# ── Graceful Shutdown ─────────────────────────────────────────────
echo ""
echo "── Shutdown ──"

echo "Step 57: Graceful shutdown"
kill -SIGTERM "$SERVER_PID"
wait "$SERVER_PID" 2>/dev/null
SHUTDOWN_CODE=$?
SERVER_PID=""
[ "$SHUTDOWN_CODE" -eq 0 ] && pass "clean shutdown (exit $SHUTDOWN_CODE)" || fail "shutdown exit $SHUTDOWN_CODE"

# ── UI Dependency Audit ──────────────────────────────────────────
echo ""
echo "── UI Dependency Audit ──"

echo "Step 58: UI dependencies are pinned (no ^/~)"
(cd ui && npm run check:deps) > /dev/null 2>&1 && pass "check:deps" || fail "check:deps"

echo "Step 59: UI build"
(cd ui && npm run build) > /dev/null 2>&1 && pass "npm run build" || fail "npm run build"

echo "Step 60: UI typecheck"
(cd ui && npm run typecheck) > /dev/null 2>&1 && pass "tsc -b" || fail "tsc -b"

echo "Step 61: UI tests"
(cd ui && npm test) > /dev/null 2>&1 && pass "npm test" || fail "npm test"

echo "Step 62: No Bootstrap/MUI/Ant/Chakra"
! grep -q "bootstrap\|@mui\|antd\|chakra" ui/package.json ui/src 2>/dev/null && pass "no forbidden frameworks" || fail "forbidden framework found"

echo "Step 63: shadcn components exist"
test -f ui/src/components/ui/button.tsx && test -f ui/src/components/ui/card.tsx && pass "shadcn components" || fail "shadcn components missing"

# ── Full Test Suite ───────────────────────────────────────────────
echo ""
echo "── Test Suite ──"

echo "Step 64: Run all tests"
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
    echo "!!! $FAIL CHECK(S) FAILED !!!"
    exit 1
fi

echo "All $PASS smoketest checks passed."
