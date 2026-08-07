# API Widia Kencana V2

Backend API untuk aplikasi Widia Kencana. Project ini memetakan flow bisnis utama ke struktur Clean Architecture dengan transport HTTP, persistence PostgreSQL, in-memory store untuk refresh token session, dan MinIO untuk asset storage.

## Features

- Authentication dengan access token JWT dan refresh token via HttpOnly cookie.
- Document designer realtime lewat WebSocket, diamankan tiket handshake sekali pakai.
- Export dokumen ke PDF, digambar langsung di Go tanpa headless browser.
- Encrypted JWT subject claim untuk menghindari expose raw user id di token.
- Asset management dengan presigned upload/download URL.
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
- MinIO object storage
- JWT
- bcrypt

## Prerequisites

- Go `1.24.4` atau versi kompatibel.
- PostgreSQL.
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
LOG_LEVEL=info

PG_HOST=localhost
PG_PORT=5432
PG_USER=postgres
PG_PASSWORD=postgres
PG_DB=widia_kencana

COOKIE_DOMAIN=
# COOKIE_SECURE=true

JWT_SECRET=change-this-in-env
JWT_SUB_ENCRYPTION_KEY=replace-with-base64-encoded-32-byte-key

MINIO_ENDPOINT=localhost:9002
MINIO_ROOT_USER=minioadmin
MINIO_ROOT_PASSWORD=minioadmin
MINIO_BUCKET=widia-assets
MINIO_USE_SSL=false

DESIGN_FONT_DIR=assets/fonts
```

Catatan:

- Refresh token session disimpan di memory proses. Session hilang setiap restart, sehingga semua user perlu login ulang setelah deploy.
- Karena session tidak dibagi antar proses, API harus dijalankan sebagai satu instance. Untuk multi-instance, store perlu dipindah ke PostgreSQL.
- `LOG_LEVEL=debug` menyalakan jejak setiap pesan WebSocket yang masuk dan keluar — arah, jenis pesan, dan ukurannya, tanpa isi payload. Sengaja tidak menyala secara bawaan karena satu geseran elemen menghasilkan puluhan pesan per detik.
- Saat `APP_ENV=local`, pemeriksaan `Origin` pada handshake WebSocket melonggar ke seluruh host loopback — `localhost`, `*.localhost`, dan `127.0.0.1`, dengan port apa pun — sehingga `FRONTEND_URL` tidak perlu disetel ulang tiap kali port atau subdomain berganti. Di luar `local`, hanya `FRONTEND_URL` yang diizinkan.
- `COOKIE_DOMAIN` sebaiknya dibiarkan kosong. Cookie menjadi host-only, terikat persis ke host yang men-set-nya, dan benar di localhost maupun production tanpa dikonfigurasi. Isi hanya bila cookie perlu dibagi ke beberapa subdomain, contoh `.example.com`.
- Flag `Secure` pada cookie mengikuti skema `APP_BASEURL` secara otomatis: `https://` menghasilkan `Secure=true`. Bila TLS diterminasi di reverse proxy dan `APP_BASEURL` menunjuk alamat internal `http://`, set `COOKIE_SECURE=true` secara eksplisit.
- Aplikasi menolak start bila `APP_ENV=production` tetapi cookie tidak `Secure`.
- Cookie memakai `SameSite=Strict`. Ini bekerja selama frontend dan API berada pada registrable domain yang sama, misal `app.example.com` dengan `api.example.com`. Bila keduanya benar-benar beda domain, `SameSite` perlu diturunkan ke `None` dan `Secure` menjadi wajib.
- `DESIGN_FONT_DIR` menunjuk direktori berkas font untuk export PDF, beserta manifes `fonts.json` di dalamnya. Direktori yang tidak ada bukan error: export tetap berjalan dengan keluarga bawaan `helvetica`. Manifes yang cacat atau berkas yang disebut manifes tetapi tidak ditemukan **menolak start**, karena keduanya berarti export akan memakai huruf yang berbeda dari tampilan editor — jauh lebih baik diketahui saat deploy daripada saat pengguna mencetak.
- Berkas font yang sama wajib disajikan ke frontend. Nama keluarga yang sama tidak cukup: Helvetica di macOS dan Arial di Windows punya lebar glif yang berbeda, dan selisihnya menumpuk menjadi pemenggalan baris yang berbeda antara layar dan hasil cetak. Detailnya ada di `docs/engineering/document-design.md`.
- MinIO local yang umum dipakai di project ini: console `9001`, API `9002`.
- `MINIO_ROOT_USER` dan `MINIO_ROOT_PASSWORD` digunakan sebagai credential MinIO.

## Database Migration

Migration disimpan sebagai pure SQL per table di folder `migration/`. Project belum menambahkan migration runner Go, sehingga migration dijalankan manual atau memakai tool eksternal.

Perhatikan bahwa berkas migration memakai `CREATE TABLE IF NOT EXISTS`, sehingga kolom yang ditambahkan belakangan **tidak** ikut terpasang pada database yang tabelnya sudah ada. Menjalankan ulang berkasnya tidak akan menghasilkan apa-apa. Untuk database lama, jalankan `ALTER TABLE` secara manual. Kolom yang ditambahkan setelah rilis awal:

```sql
ALTER TABLE documents
    ADD COLUMN IF NOT EXISTS content JSONB NOT NULL DEFAULT '{"pages": []}',
    ADD COLUMN IF NOT EXISTS content_version BIGINT NOT NULL DEFAULT 0;
```

Urutan baseline yang aman:

```text
users.sql
projects.sql
workflows.sql
workflow_stages.sql
workflow_steps.sql
document_papers.sql
documents.sql
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
internal/infrastructure/         Config, database, server, security, storage
internal/persistence/postgres/   PostgreSQL repositories
internal/persistence/memory/     In-memory stores (refresh token session)
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
- `docs/collection/asset.json`
- `docs/collection/document.json`
- `docs/collection/document_design.json`
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

POST   /api/asset-upload-request
POST   /api/asset-upload-complete/:token
GET    /api/asset-list
GET    /api/asset-detail/:token
GET    /api/asset-presign/:token
DELETE /api/asset-delete/:token

GET    /api/document-list
GET    /api/document-detail/:token
POST   /api/document-add
PUT    /api/document-update/:token
DELETE /api/document-delete/:token

GET    /api/document-design-fonts
POST   /api/document-design-ticket/:token
WS     /document-design/:token?ticket=
POST   /api/document-export/:token

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
  "data": {
    "entity_key": {}
  }
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

Payload response tidak meletakkan object atau array langsung pada `data`, tetapi selalu dibungkus dengan key entity:

```json
{
  "status": "ok",
  "message": "Success",
  "data": {
    "workflow": {
      "id": 1,
      "name": "Standard Project Flow"
    }
  }
}
```

List response juga memakai key entity plural:

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

- Arsitektur document design di backend ada di `docs/engineering/document-design-architecture.md`: alur hulu ke hilir, kenapa memakai orchestrator, invarian yang tidak boleh dilanggar, model kegagalan, dan alasan di balik tiap tetapan waktu. Baca ini sebelum mengubah kodenya.
- Kontrak WebSocket untuk document design ada di `docs/engineering/websocket-contract.md`: setiap pesan, apa yang memicunya, close code, dan riwayat perubahan kontrak.
- Panduan membangun editor ada di `docs/engineering/document-design.md`: model isi dokumen, aturan agar tampilan editor sama dengan hasil cetak, export PDF, dan daftar periksa penyesuaian.
- Kontrak dalam bentuk definisi tipe TypeScript ada di `docs/engineering/document-design.d.ts`, untuk disalin atau diimpor frontend.
- Aturan bahwa keempatnya berubah dalam satu commit ada di `CLAUDE.md`.
- Clean Architecture reference ada di `docs/engineering/Clean_Architecture.md`.
- README structure guideline ada di `docs/engineering/readme-structure.md`.
- Unit test baru belum menjadi scope utama project ini; `go test ./...` dipakai sebagai compile/smoke verification.
- Endpoint update/delete pada project dan workflow memakai soft-delete via field `status = deleted` untuk flow delete.
