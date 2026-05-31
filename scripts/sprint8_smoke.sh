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
echo "=== Step 3: Boot server ==="
export PATH="/usr/local/go/bin:$PATH"
export LD_LIBRARY_PATH="/home/raj/project/plomvix/.rocksdb/usr/lib/x86_64-linux-gnu"
./plomvix > /tmp/plomvix_s8.log 2>&1 &
SERVER_PID=$!
sleep 3

echo ""
echo "=== Step 4: Login ==="
TOKEN=$(curl -sf -X POST http://localhost:8080/auth/login \
    -H "Content-Type: application/json" \
    -d '{"username":"admin","password":"changeme"}' | jq -r '.data.token')
echo "Token acquired"

echo ""
echo "=== Step 5: JSON wrapper ingest still works ==="
STATUS=$(curl -s -o /dev/null -w "%{http_code}" \
    -X POST http://localhost:8080/ingest/logs \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d '{"records":[{"level":"info","message":"json wrapper test"}]}')
[ "$STATUS" -eq 201 ] && echo "PASS: JSON wrapper ingest 201" \
    || { echo "FAIL: JSON wrapper ingest got $STATUS"; exit 1; }

echo ""
echo "=== Step 6: JSON bare array works ==="
STATUS=$(curl -s -o /dev/null -w "%{http_code}" \
    -X POST http://localhost:8080/ingest/logs \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d '[{"level":"info","message":"json array test"}]')
[ "$STATUS" -eq 201 ] && echo "PASS: JSON array ingest 201" \
    || { echo "FAIL: JSON array ingest got $STATUS"; exit 1; }

echo ""
echo "=== Step 7: CSV ingest ==="
RESP=$(curl -sf -X POST http://localhost:8080/ingest/logs \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: text/csv" \
    --data-binary $'level,message\ninfo,a\nwarn,b')
INGESTED=$(echo "$RESP" | jq -r '.data.ingested')
[ "$INGESTED" -eq 2 ] && echo "PASS: CSV ingested=$INGESTED" \
    || { echo "FAIL: CSV ingested=$INGESTED, want 2"; exit 1; }

echo ""
echo "=== Step 8: Logfmt ingest ==="
STATUS=$(curl -s -o /dev/null -w "%{http_code}" \
    -X POST http://localhost:8080/ingest/logs \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: text/x-logfmt" \
    --data-binary $'level=info msg="logfmt test" host=web-01')
[ "$STATUS" -eq 201 ] && echo "PASS: logfmt ingest 201" \
    || { echo "FAIL: logfmt ingest got $STATUS"; exit 1; }

echo ""
echo "=== Step 9: Syslog ingest ==="
STATUS=$(curl -s -o /dev/null -w "%{http_code}" \
    -X POST http://localhost:8080/ingest/logs \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/x-syslog" \
    --data-binary '<34>1 2024-01-15T10:30:00Z web-01 testapp 1 - - syslog test message')
[ "$STATUS" -eq 201 ] && echo "PASS: syslog ingest 201" \
    || { echo "FAIL: syslog ingest got $STATUS"; exit 1; }

echo ""
echo "=== Step 10: Empty body returns 400 ==="
STATUS=$(curl -s -o /dev/null -w "%{http_code}" \
    -X POST http://localhost:8080/ingest/logs \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d '')
[ "$STATUS" -eq 400 ] && echo "PASS: empty body → 400" \
    || { echo "FAIL: expected 400, got $STATUS"; exit 1; }

echo ""
echo "=== Step 11: Header-only CSV returns 400 ==="
STATUS=$(curl -s -o /dev/null -w "%{http_code}" \
    -X POST http://localhost:8080/ingest/logs \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: text/csv" \
    --data-binary $'level,message\n')
[ "$STATUS" -eq 400 ] && echo "PASS: header-only CSV → 400" \
    || { echo "FAIL: expected 400, got $STATUS"; exit 1; }

echo ""
echo "=== Step 12: Unsupported format on metrics returns 400 ==="
STATUS=$(curl -s -o /dev/null -w "%{http_code}" \
    -X POST http://localhost:8080/ingest/metrics \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: text/csv" \
    --data-binary $'name,value\ncpu,1')
[ "$STATUS" -eq 400 ] && echo "PASS: metrics CSV → 400" \
    || { echo "FAIL: expected 400, got $STATUS"; exit 1; }

echo ""
echo "=== Step 13: Query logs includes multi-format records ==="
RESP=$(curl -sf "http://localhost:8080/query/logs" \
    -H "Authorization: Bearer $TOKEN")
TOTAL=$(echo "$RESP" | jq '.data.total')
[ "$TOTAL" -ge 5 ] && echo "PASS: total=$TOTAL" \
    || { echo "FAIL: total=$TOTAL, want >= 5"; exit 1; }

echo ""
echo "=== Step 14: Graceful shutdown ==="
kill -SIGTERM "$SERVER_PID"
wait "$SERVER_PID"
EXIT_CODE=$?
SERVER_PID=""
[ "$EXIT_CODE" -eq 0 ] && echo "PASS: clean shutdown" \
    || { echo "FAIL: exit code $EXIT_CODE"; exit 1; }

echo ""
echo "================================================"
echo "  ALL STEPS PASSED — Sprint 8 smoke test DONE  "
echo "================================================"
