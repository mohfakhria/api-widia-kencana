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

`.env.example` memuat seluruh variabel yang dibaca aplikasi beserta keterangan
tiap yang tidak jelas dengan sendirinya. Nilai bawaannya sudah benar untuk
pengembangan di mesin sendiri, kecuali dua kunci di bawah.

Generate key untuk encrypted JWT subject:

```bash
openssl rand -base64 32
```

Masukkan hasilnya ke:

```env
JWT_SUB_ENCRYPTION_KEY=replace-with-generated-key
```

## Configuration

Konfigurasi runtime dibaca dari `.env`, yang disalin dari
[`.env.example`](.env.example) pada langkah pemasangan di atas.

**Daftar variabelnya ada di berkas itu, dan sengaja tidak disalin ke sini.**
Salinan yang kedua selalu kalah cepat dari yang pertama: sebelum catatan ini
ditulis, salinan di README sudah kehilangan seluruh komentar `.env.example` —
termasuk yang menjelaskan kapan `COOKIE_SECURE` perlu diisi — sementara orang yang
membaca README saja tidak punya cara tahu ada yang hilang.

Nilai bawaannya aman untuk mesin sendiri dan tidak aman di tempat lain. Yang
harus diganti sebelum dipakai di luar itu disebut di [Deployment](#deployment),
bersama yang lain yang hanya berlaku di sana.

Satu variabel dibaca aplikasi tetapi **tidak** ada di `.env.example`, dan itu
disengaja: `DESIGN_FONT_DIR` bukan konfigurasi hari pertama — lihat catatan
tentang font di bawah.

Catatan berikut menjelaskan hal-hal yang tidak muat sebagai komentar di berkas
contoh:

- Refresh token session disimpan di memory proses. Session hilang setiap restart, sehingga semua user perlu login ulang setelah deploy.
- Karena session tidak dibagi antar proses, API harus dijalankan sebagai satu instance. Untuk multi-instance, store perlu dipindah ke PostgreSQL.
- `LOG_LEVEL=debug` menyalakan jejak setiap pesan WebSocket yang masuk dan keluar — arah, jenis pesan, dan ukurannya, tanpa isi payload. Sengaja tidak menyala secara bawaan karena satu geseran elemen menghasilkan puluhan pesan per detik.
- Saat `APP_ENV=local`, pemeriksaan `Origin` pada handshake WebSocket melonggar ke seluruh host loopback — `localhost`, `*.localhost`, dan `127.0.0.1`, dengan port apa pun — sehingga `FRONTEND_URL` tidak perlu disetel ulang tiap kali port atau subdomain berganti. Di luar `local`, hanya `FRONTEND_URL` yang diizinkan.
- `COOKIE_DOMAIN` sebaiknya dibiarkan kosong. Cookie menjadi host-only, terikat persis ke host yang men-set-nya, dan benar di localhost maupun production tanpa dikonfigurasi. Isi hanya bila cookie perlu dibagi ke beberapa subdomain, contoh `.example.com`.
- Flag `Secure` pada cookie mengikuti skema `APP_BASEURL` secara otomatis: `https://` menghasilkan `Secure=true`. Bila TLS diterminasi di reverse proxy dan `APP_BASEURL` menunjuk alamat internal `http://`, set `COOKIE_SECURE=true` secara eksplisit.
- Aplikasi menolak start bila `APP_ENV=production` tetapi cookie tidak `Secure`.
- Cookie memakai `SameSite=Strict`. Ini bekerja selama frontend dan API berada pada registrable domain yang sama, misal `app.example.com` dengan `api.example.com`. Bila keduanya benar-benar beda domain, `SameSite` perlu diturunkan ke `None` dan `Secure` menjadi wajib.
- `DESIGN_FONT_DIR` **kosong secara bawaan**, dan tidak perlu diisi. Export PDF memakai Helvetica inti — keadaan yang didukung, bukan keadaan darurat. Yang menjadi syaratnya ada di sisi frontend: `font-family` dipaku ke daftar yang lebar majunya sepadan dengan Helvetica, tidak diserahkan ke `sans-serif` telanjang. Aturan lengkapnya di [`document-design.md`](docs/engineering/document-design.md#22-font-berkas-yang-sama-di-kedua-sisi).
- Isi `DESIGN_FONT_DIR` hanya bila mendaftarkan berkas font sendiri; ia menunjuk direktori berkas font beserta manifes `fonts.json` di dalamnya, dan berkas yang sama wajib disajikan ke frontend lewat `@font-face`. Jalur relatif diselesaikan terhadap direktori kerja proses — di systemd berarti `WorkingDirectory`. Manifes yang cacat atau berkas yang disebut manifes tetapi tidak ditemukan **menolak start**, karena keduanya berarti export akan memakai huruf yang berbeda dari tampilan editor — jauh lebih baik diketahui saat deploy daripada saat pengguna mencetak.
- Yang menentukan hasil cetak sama dengan layar adalah **lebar maju** yang sama, bukan nama keluarga yang sama. Arial justru diciptakan sebagai pengganti Helvetica dengan lebar maju yang sama persis, dan Liberation Sans serta Nimbus Sans dirancang selebar itu pula — yang berbeda bentuk glifnya. Yang merusak adalah font yang lebarnya memang lain, seperti DejaVu Sans yang menjadi pilihan `sans-serif` bawaan di banyak Linux. Detailnya ada di `docs/engineering/document-design.md`.
- Export PDF **tidak pernah gagal** karena font. Ketebalan atau keluarga yang tidak terdaftar dibulatkan ke yang terdekat dan dicatat sebagai peringatan di log, beserta jumlah elemen yang terpengaruh. Karena itu editor hanya boleh menawarkan ketebalan 400 dan 700.
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

## Deployment

Aplikasi dijalankan sebagai **binary biasa di bawah systemd**, bukan sebagai
container. Docker dipakai hanya sebagai kotak build supaya binary yang mendarat
di server selalu dibangun oleh toolchain yang sama.

### 1. Bangun binary-nya

```bash
task build:linux
```

Hasilnya `dist/widia-api` — ELF statis (`CGO_ENABLED=0`), sekitar 25 MB, tanpa
tuntutan glibc atau pustaka apa pun, jadi berjalan di Debian, Ubuntu, maupun
Alpine. Untuk server ARM:

```bash
task build:linux ARCH=arm64
```

Tanpa Task, perintahnya:

```bash
docker build --target binary --output type=local,dest=./dist .
```

Build ini **dapat diulang**: `-trimpath`, `-buildvcs=false`, dan versi Go yang
dipatok di Dockerfile membuat sumber yang sama selalu menghasilkan berkas yang
sama persis, terlepas dari mesin mana yang membangunnya. Terverifikasi — hasil
Docker dan hasil `go build` lokal dengan bendera yang sama identik byte per byte.

Gunanya saat ada keraguan tentang apa yang sebenarnya berjalan di server:

```bash
ssh server 'sha256sum /opt/widia-api/widia-api'
git checkout <commit> && task build:linux && sha256sum dist/widia-api
```

Sidik jari yang sama berarti binary di server memang berasal dari commit itu.
Berbeda berarti ada yang men-deploy sesuatu yang tidak ada di repo — dan itu
jauh lebih baik diketahui lewat satu perintah daripada lewat gejala.

### 2. Tata letak di server

```text
/opt/widia-api/
  widia-api                  binary, milik root, mode 0755
/etc/widia-api/
  api.env                    konfigurasi, milik root, mode 0600
```

Tidak ada berkas pendamping yang perlu ikut dikirim — satu binary saja. Export
PDF memakai Helvetica inti, yang metriknya melekat pada spesifikasi PDF dan tidak
perlu disematkan.

`WorkingDirectory` di unit systemd tetap disetel: ia yang menentukan letak
direktori font seandainya suatu hari `DESIGN_FONT_DIR` diisi dengan jalur
relatif.

### 3. Konfigurasi

`/etc/widia-api/api.env` memakai format `KEY=value` seperti `.env.example`,
tetapi **bukan** berkas shell: tidak ada `export`, tidak ada substitusi
`${VAR}`, dan komentar hanya boleh satu baris penuh — bukan di belakang nilai.

```bash
sudo install -d -m 0755 /etc/widia-api
sudo install -m 0600 /dev/null /etc/widia-api/api.env
sudo -e /etc/widia-api/api.env
```

Yang wajib diganti dari contoh: `JWT_SECRET`, `JWT_SUB_ENCRYPTION_KEY`,
`PG_PASSWORD`, dan `MINIO_ROOT_PASSWORD`. Setel juga
`APP_ENV=production`, `APP_BASEURL` ke URL publik yang sesungguhnya, dan
`FRONTEND_URL` ke origin frontend — nilai terakhir itu yang memutuskan handshake
WebSocket diterima atau ditolak `403`.

### 4. Pasang unit systemd

```bash
sudo useradd --system --no-create-home --shell /usr/sbin/nologin widia
sudo install -d -o widia -g widia /opt/widia-api
sudo install -o root -g root -m 0755 dist/widia-api /opt/widia-api/widia-api
sudo install -m 0644 deploy/widia-api.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now widia-api
```

Periksa:

```bash
systemctl status widia-api
journalctl -u widia-api -f
curl http://127.0.0.1:8080/health
```

Baris `warning: .env file not found, using system env` di awal log adalah wajar
di server: konfigurasi datang lewat `EnvironmentFile`, bukan lewat berkas `.env`.

### 5. Memperbarui

```bash
task build:linux
scp dist/widia-api server:/tmp/widia-api
ssh server 'sudo install -m 0755 /tmp/widia-api /opt/widia-api/widia-api && sudo systemctl restart widia-api'
```

`install` menulis berkas baru lalu menggantinya secara atomik, jadi aman
dilakukan selagi layanan berjalan — berbeda dengan `cp` yang menimpa berkas yang
sedang dieksekusi.

Restart **tidak** boleh dipercepat dengan `SIGKILL`. `TimeoutStopSec=30s` di unit
file ada karena orchestrator dokumen butuh sampai 8 detik untuk menyimpan
suntingan terakhir setiap sesi penyuntingan yang sedang terbuka; mematikannya
lebih cepat berarti membuang pekerjaan pengguna yang belum sempat ditulis.

### Yang tidak diurus berkas-berkas ini

- **Migration.** SQL murni di `migration/`, dijalankan manual — lihat
  [Database Migration](#database-migration). Tidak ada migrator otomatis saat
  start, disengaja.
- **TLS dan reverse proxy.** Aplikasi mengikat `:APP_PORT` di **semua**
  antarmuka, jadi batasi dengan firewall dan letakkan nginx atau Caddy di
  depannya. Proxy-nya wajib meneruskan header `Upgrade` dan `Connection` untuk
  `/document-design/`, dan `proxy_read_timeout`-nya harus lebih panjang dari
  koneksi WebSocket yang menganggur.
- **Backup.** Postgres dan MinIO, di luar cakupan repo ini.

## Project Structure

```text
cmd/api/                         API entry point
deploy/                          unit systemd untuk deploy
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
GET    /api/asset-content/:token      (tanpa auth — token aset yang menjadi kredensial)
DELETE /api/asset-delete/:token

GET    /api/document-list
GET    /api/document-detail/:token
POST   /api/document-add
PUT    /api/document-update/:token
DELETE /api/document-delete/:token

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
- Wewenang asset dipisah antara membaca dan mengubah. **Membaca** — `asset-detail` dan `asset-presign` — terbuka bagi siapa pun yang login asal ia tahu tokennya, karena dokumen di aplikasi ini milik bersama: gambar yang tokennya sudah masuk ke `assetToken` sebuah elemen sudah menjadi bagian dokumen bersama itu, dan ekspor PDF memang menyematkannya untuk semua orang. **Mengubah** — `asset-upload-complete` dan `asset-delete` — hanya boleh oleh pengunggahnya, dan aset yang `uploaded_by`-nya kosong ditolak untuk siapa pun. `asset-list` tetap hanya menampilkan milik sendiri.
- `GET /api/asset-content/:token` mengalihkan (`302`) ke presigned URL yang disusun ulang tiap permintaan, dan **sengaja berada di luar `AuthRequired`**: rute ini dituju langsung oleh tag `<img>`, yang tidak dapat mengirim header `Authorization`. Token asetnya yang menjadi kredensial — UUID acak yang hanya diperoleh lewat isi dokumen. Balasannya `Cache-Control: no-store`, karena pengalihan yang tersimpan akan menunjuk ke tanda tangan yang keburu kedaluwarsa. Byte-nya tidak pernah melewati proses Go; hanya alamatnya.
- Unggahan asset berjalan tiga langkah — minta presigned URL, unggah langsung ke MinIO, lapor selesai — dan langkah ketiga sepenuhnya bergantung pada klien. Tab yang ditutup di tengah jalan meninggalkan baris `pending` dan kadang objek yang sudah telanjur mendarat. Komponen latar `asset-sweeper` membersihkannya: tiap 5 menit ia mencari unggahan yang tenggat presigned-nya lewat lebih dari 5 menit, menghapus objeknya, lalu menandai barisnya `failed` dengan `failure_code = upload_expired`. Objek yatim karenanya hidup paling lama sekitar 25 menit.
