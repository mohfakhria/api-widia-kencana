# API Widia Kencana V2

Backend API untuk aplikasi Widia Kencana. Project ini memetakan flow bisnis utama ke struktur Clean Architecture dengan transport HTTP, persistence PostgreSQL, Redis untuk refresh token, dan MinIO untuk asset storage.

## Features

- Authentication dengan access token JWT dan refresh token via HttpOnly cookie.
- Encrypted JWT subject claim untuk menghindari expose raw user id di token.
- Quotation management dengan list/detail/create/update.
- Purchase order by quotation, termasuk upsert item dan optional asset upload ke MinIO.
- Project CRUD.
- Workflow master CRUD, termasuk stage dan step.
- SQL migration manual per table di folder `migration/`.
- Postman collection di folder `docs/collection/`.

## Tech Stack

- Go `1.24.4`
- Gin HTTP framework
- PostgreSQL
- Redis
- MinIO object storage
- JWT
- bcrypt

## Prerequisites

- Go `1.24.4` atau versi kompatibel.
- PostgreSQL.
- Redis, bila refresh token ingin aktif.
- MinIO, bila flow asset upload ingin digunakan.
- `jq`, opsional untuk validasi JSON collection.

## Installation

```bash
git clone git@github.com:mohfakhria/api-widia-kencana.git
cd api-widia-kencana-v2
go mod download
cp .env.example .env
```

Generate key untuk encrypted JWT subject:

```bash
openssl rand -base64 32
```

Masukkan hasilnya ke:

```env
JWT_SUB_ENCRYPTION_KEY=replace-with-generated-key
```

## Configuration

Konfigurasi runtime dibaca dari `.env`.

```env
APP_ENV=local
APP_PORT=8080
APP_BASEURL=http://localhost:8080
FRONTEND_URL=http://localhost:3000

PG_HOST=localhost
PG_PORT=5432
PG_USER=postgres
PG_PASSWORD=postgres
PG_DB=widia_kencana

REDIS_ENABLED=true
REDIS_HOST=localhost
REDIS_PORT=6379

JWT_SECRET=change-this-in-env
JWT_SUB_ENCRYPTION_KEY=replace-with-base64-encoded-32-byte-key

MINIO_ENDPOINT=localhost:9002
MINIO_ROOT_USER=minioadmin
MINIO_ROOT_PASSWORD=minioadmin
MINIO_BUCKET=widia-assets
MINIO_USE_SSL=false
```

Catatan:

- Jika `REDIS_ENABLED=false`, login tetap mengembalikan access token, tetapi refresh token cookie tidak dibuat dan endpoint refresh token akan disabled.
- MinIO local yang umum dipakai di project ini: console `9001`, API `9002`.
- `MINIO_ROOT_USER` dan `MINIO_ROOT_PASSWORD` digunakan sebagai credential MinIO.

## Database Migration

Migration disimpan sebagai pure SQL per table di folder `migration/`. Project belum menambahkan migration runner Go, sehingga migration dijalankan manual atau memakai tool eksternal.

Urutan baseline yang aman:

```text
users.sql
projects.sql
workflows.sql
workflow_stages.sql
workflow_steps.sql
quotations.sql
quotation_sections.sql
quotation_items.sql
quotation_details.sql
purchase_order.sql
purchase_order_detail.sql
assets.sql
purchase_order_assets.sql
```

Contoh menjalankan manual dengan `psql`:

```bash
psql "$DATABASE_URL" -f migration/users.sql
```

## Running The API

```bash
go run ./cmd/api
```

Health check:

```bash
curl http://localhost:8080/health
```

## Project Structure

```text
cmd/api/                         API entry point
internal/bootstrap/              Application wiring
internal/delivery/http/          HTTP handlers, router, middleware, DTO
internal/domain/                 Domain errors and entities
internal/infrastructure/         Config, database, server, security, cache, storage
internal/persistence/postgres/   PostgreSQL repositories
internal/persistence/redis/      Redis stores
internal/usecase/                Application use cases
internal/usecase/port/input/     Input ports
internal/usecase/port/output/    Output ports
migration/                       Manual SQL migrations
docs/collection/                 Postman collections
docs/engineering/                Engineering notes and references
pkg/                             Shared utility packages
```

## API Documentation

Postman collection tersedia di:

- `docs/collection/auth.json`
- `docs/collection/quotation.json`
- `docs/collection/purchase_order.json`
- `docs/collection/project.json`
- `docs/collection/workflow.json`
- `docs/collection/all.json`

Regenerate collection gabungan:

```bash
go run docs/collection/merge.go
```

Endpoint utama:

```text
POST   /api/login
POST   /api/refresh-token
POST   /api/logout
GET    /api/me

GET    /api/quotation-list
GET    /api/quotation-detail/:id
POST   /api/quotation-add
PUT    /api/quotation-update/:id

POST   /api/purchase-order-upsert
GET    /api/purchase-order/:quotationID

GET    /api/project-list
GET    /api/project-detail/:id
POST   /api/project-add
PUT    /api/project-update/:id
DELETE /api/project-delete/:id

GET    /api/workflow-list
GET    /api/workflow-detail/:id
POST   /api/workflow-add
PUT    /api/workflow-update/:id
DELETE /api/workflow-delete/:id

GET    /api/workflow-stage-list/:workflowID
GET    /api/workflow-stage-detail/:id
POST   /api/workflow-stage-add
PUT    /api/workflow-stage-update/:id
DELETE /api/workflow-stage-delete/:id

GET    /api/workflow-step-list/:workflowStageID
GET    /api/workflow-step-detail/:id
POST   /api/workflow-step-add
PUT    /api/workflow-step-update/:id
DELETE /api/workflow-step-delete/:id
```

## Response Convention

Success response selalu memakai shape:

```json
{
  "status": "ok",
  "message": "Success",
  "data": {}
}
```

Jika tidak ada payload response:

```json
{
  "status": "ok",
  "message": "Operation successful",
  "data": null
}
```

List response tidak mengembalikan array langsung pada `data`, tetapi dibungkus object sesuai entity:

```json
{
  "status": "ok",
  "message": "Success",
  "data": {
    "projects": []
  }
}
```

Error response:

```json
{
  "status": "error",
  "message": "error message"
}
```

## Testing

Compile and smoke check semua package:

```bash
GOCACHE=$(pwd)/.gocache go test ./...
```

Validasi JSON collection:

```bash
jq empty docs/collection/*.json
```

## Development Notes

- Clean Architecture reference ada di `docs/engineering/Clean_Architecture.md`.
- README structure guideline ada di `docs/engineering/readme-structure.md`.
- Unit test baru belum menjadi scope utama project ini; `go test ./...` dipakai sebagai compile/smoke verification.
- Endpoint update/delete pada project dan workflow memakai soft-delete via field `status = deleted` untuk flow delete.
