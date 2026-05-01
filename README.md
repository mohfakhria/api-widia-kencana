# API Widia Kencana V2

Base project v2 ini memetakan flow bisnis utama v1 ke struktur Clean Architecture.

## Flow parity dari v1

Route bisnis yang sudah dipetakan ke v2:

- `POST /api/login`
- `POST /api/refresh-token`
- `POST /api/logout`
- `GET /api/me`
- `GET /api/quotation-list`
- `GET /api/quotation-detail/:id`
- `POST /api/quotation-add`
- `PUT /api/quotation-update/:id`

Endpoint `/health` adalah tambahan operasional v2 dan bukan bagian dari flow bisnis v1.

## Utility developer

Helper developer yang dipertahankan untuk parity dengan v1:

- `pkg/password.Hash(...)` untuk generate bcrypt hash tanpa masuk ke runtime API
