#!/bin/sh
set -eu

BASE_URL=${BASE_URL:-http://localhost:8080}

curl --fail-with-body --request POST "$BASE_URL/order-updates" \
  --header 'Content-Type: application/json' \
  --data '{"order_id":"ORD-1042","phone":"+15550001042","kind":"checkout_confirmed","detail":"payment accepted","sms_opt_out":true,"suppressed":false}'
