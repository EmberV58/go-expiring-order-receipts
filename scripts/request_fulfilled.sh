#!/bin/sh
set -eu

curl --fail-with-body -X POST http://localhost:8080/orders/update \
  -H 'Content-Type: application/json' \
  -d '{"order_id":"ord_42","customer_id":"cus_7","status":"fulfilled","receipt_key":"receipts/ord_42.pdf"}'
