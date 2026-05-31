#!/bin/bash
set -euo pipefail

echo "=== Clearing stale data ==="
rm -rf data/hot/ data/wal/ data/cold/

SERVER_PID=""
cleanup() {
    if [ -n "$SERVER_PID" ] && kill -0 "$SERVER_PID" 2>/dev/null; then
        kill -SIGTERM "$SERVER_PID" 2>/dev/null || true
        wait "$SERVER_PID" 2>/dev/null || true
    fi
}
trap cleanup EXIT

echo "=== Step 1: Build ==="
CGO_ENABLED=1 make vet
CGO_ENABLED=1 make build

echo ""
echo "=== Step 2: Run all tests ==="
CGO_ENABLED=1 make test

echo ""
echo "=== Step 3: Boot server with retention_days=0 to enable flush in smoke test ==="
PLOMVIX_STORAGE_RETENTION_DAYS=0 LD_LIBRARY_PATH="/home/raj/project/plomvix/.rocksdb/usr/lib/x86_64-linux-gnu" \
    ./plomvix > /tmp/plomvix_s7.log 2>&1 &
SERVER_PID=$!
sleep 3

echo ""
echo "=== Step 4: Login ==="
TOKEN=$(curl -sf -X POST http://localhost:8080/auth/login \
    -H "Content-Type: application/json" \
    -d '{"username":"admin","password":"changeme"}' | jq -r '.data.token')
echo "Token acquired"

echo ""
echo "=== Step 5: Ingest test data ==="
curl -sf -X POST http://localhost:8080/ingest/logs \
    -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
    -d '{"records":[{"level":"info","message":"tier smoke test"}]}' > /dev/null
echo "Data ingested"

echo ""
echo "=== Step 6: Health includes cold stats ==="
HEALTH=$(curl -sf http://localhost:8080/health)
echo "$HEALTH" | jq '.data.cold' | grep -q "parquet_files" \
    && echo "PASS: cold block in health" \
    || { echo "FAIL: cold block missing"; exit 1; }

echo ""
echo "=== Step 7: Manual flush with retention_days=0 actually moves records ==="
RESP=$(curl -sf -X POST http://localhost:8080/admin/tier/flush \
    -H "Authorization: Bearer $TOKEN")
echo "$RESP" | jq .
MOVED=$(echo "$RESP" | jq '.data.records_moved')
[ "$MOVED" -ge 1 ] \
    && echo "PASS: records_moved=$MOVED >= 1" \
    || { echo "FAIL: records_moved=$MOVED, want >= 1"; exit 1; }

echo ""
echo "=== Step 8: Cold parquet file created ==="
FILES=$(echo "$RESP" | jq '.data.parquet_files')
[ "$FILES" -ge 1 ] \
    && echo "PASS: parquet_files=$FILES" \
    || { echo "FAIL: parquet_files=$FILES, want >= 1"; exit 1; }

echo ""
echo "=== Step 9: Query logs still returns data after flush (from cold tier) ==="
RESP=$(curl -sf "http://localhost:8080/query/logs" \
    -H "Authorization: Bearer $TOKEN")
TOTAL=$(echo "$RESP" | jq '.data.total')
[ "$TOTAL" -ge 1 ] \
    && echo "PASS: query after flush returns $TOTAL records" \
    || { echo "FAIL: query returned 0 after flush"; exit 1; }

echo ""
echo "=== Step 10: Tier flush requires auth ==="
STATUS=$(curl -s -o /dev/null -w "%{http_code}" \
    -X POST http://localhost:8080/admin/tier/flush)
[ "$STATUS" -eq 401 ] && echo "PASS: no auth → 401" \
    || { echo "FAIL: expected 401, got $STATUS"; exit 1; }

echo ""
echo "=== Step 11: Graceful shutdown ==="
kill -SIGTERM "$SERVER_PID"
wait "$SERVER_PID"
EXIT_CODE=$?
SERVER_PID=""
[ "$EXIT_CODE" -eq 0 ] && echo "PASS: clean shutdown" \
    || { echo "FAIL: exit code $EXIT_CODE"; exit 1; }

echo ""
echo "================================================"
echo "  ALL STEPS PASSED — Sprint 7 smoke test DONE  "
echo "================================================"
