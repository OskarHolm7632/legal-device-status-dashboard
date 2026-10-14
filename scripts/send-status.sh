#!/bin/sh
set -eu

service_url="${SERVICE_URL:-http://localhost:8080}"
curl --fail-with-body --request POST "${service_url}/matters/status" \
  --header 'Content-Type: application/json' \
  --data '{
    "request_id": "status-MAT-204-20260926",
    "matter_id": "MAT-204",
    "device_id": "intake-kiosk-3",
    "device_online": true,
    "intake_complete": true,
    "signed_document_delivered": true,
    "deadline": "2026-09-27T10:00:00Z"
  }'
