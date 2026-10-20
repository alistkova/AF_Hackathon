#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

pnpm --dir web exec openapi-typescript ../contracts/openapi/gateway.yaml \
  -o src/api/schema.d.ts

cd services/agent
uv run datamodel-codegen \
  --input ../../contracts/openapi/agent.yaml \
  --input-file-type openapi \
  --output app/contracts/generated.py \
  --output-model-type pydantic_v2.BaseModel \
  --disable-timestamp
