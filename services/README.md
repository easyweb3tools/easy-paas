# v2/services

This directory contains business services that are proxied behind the easyweb3 PaaS gateway.

Each service can be implemented with its own tech stack (Go, Next.js, etc.), as long as it exposes HTTP.

Gateway routing:
- PaaS: `/api/v1/services/{name}/*` -> service `base_url`

Docs:
- If a service exposes markdown at a configured `docs_path`, PaaS can serve it via `GET /api/v1/service/docs?name={name}`.
