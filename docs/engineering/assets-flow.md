# Asset Management Flow

Dokumen ini menjelaskan cara frontend menggunakan asset management API. Prinsip utamanya:

- Frontend tidak pernah menyimpan credential MinIO.
- Frontend upload file langsung ke object storage memakai presigned URL dari backend.
- Backend tetap menjadi gatekeeper untuk authorization, metadata asset, preview/download URL, dan lifecycle status.
- Frontend menyimpan dan mengirim `asset_token`, bukan numeric `id`.

## Endpoint Ringkas

Semua endpoint di bawah membutuhkan header:

```http
Authorization: Bearer <access_token>
```

```text
POST   /api/asset-upload-request
POST   /api/asset-upload-complete/:token
GET    /api/asset-list
GET    /api/asset-detail/:token
GET    /api/asset-presign/:token
DELETE /api/asset-delete/:token
```

## Status Asset

Frontend perlu memahami status berikut:

```text
pending    Backend sudah membuat metadata dan presigned upload URL.
uploading  Reserved untuk flow multipart atau progress tracking lanjutan.
uploaded   File sudah ada di object storage dan valid.
failed     Upload gagal/expired/tidak valid.
deleted    Metadata sudah soft deleted.
```

Untuk pemakaian normal, frontend biasanya hanya menampilkan asset `uploaded`.

## Flow Upload Yang Benar

### 1. User Pilih File

Frontend ambil metadata dari `File` object:

```ts
const file = input.files[0]

const payload = {
  original_filename: file.name,
  mime_type: file.type || "application/octet-stream",
  size: file.size,
  scope: "documents"
}
```

Gunakan `scope` untuk folder object storage. Contoh scope yang disarankan:

```text
documents
quotations
purchase-orders
users/avatar
```

Backend akan sanitize scope dan filename.

### 2. Request Presigned Upload URL

```http
POST /api/asset-upload-request
Content-Type: application/json
```

Request:

```json
{
  "original_filename": "logo.png",
  "mime_type": "image/png",
  "size": 48291,
  "scope": "documents"
}
```

Response:

```json
{
  "status": "ok",
  "message": "Asset upload URL generated successfully",
  "data": {
    "asset": {
      "token": "98b5e767-0000-4000-9000-5f73d39cdd66",
      "object_name": "documents/uuid-logo.png",
      "original_filename": "logo.png",
      "stored_filename": "uuid-logo.png",
      "mime_type": "image/png",
      "extension": "png",
      "size": 48291,
      "status": "pending",
      "upload_method": "presigned_put",
      "is_private": true,
      "presigned_expires_at": "2026-07-27T04:30:00Z"
    },
    "upload_url": "https://minio.example.com/...",
    "expires_at": "2026-07-27T04:30:00Z"
  }
}
```

Frontend harus menyimpan sementara:

```text
asset.token
upload_url
expires_at
```

Jangan menyimpan `upload_url` sebagai URL permanen karena URL ini akan expired.

### 3. Upload File Ke Presigned URL

Upload langsung ke `upload_url` memakai HTTP `PUT`.

```ts
await fetch(uploadUrl, {
  method: "PUT",
  headers: {
    "Content-Type": file.type || "application/octet-stream"
  },
  body: file
})
```

Catatan:

- Jangan kirim `Authorization: Bearer ...` ke MinIO presigned URL.
- Gunakan `Content-Type` yang sama dengan `mime_type` saat request upload.
- Kalau request PUT gagal, tampilkan retry upload. Jangan call complete sebelum PUT sukses.

### 4. Complete Upload

Setelah PUT sukses, frontend wajib call complete:

```http
POST /api/asset-upload-complete/:asset_token
```

Response sukses:

```json
{
  "status": "ok",
  "message": "Asset upload completed successfully",
  "data": {
    "asset": {
      "token": "98b5e767-0000-4000-9000-5f73d39cdd66",
      "status": "uploaded",
      "uploaded_at": "2026-07-27T04:22:00Z"
    }
  }
}
```

Backend akan:

- cek object di storage,
- validasi size,
- validasi MIME type,
- ubah status menjadi `uploaded`.

Jika complete gagal karena expired, size mismatch, MIME mismatch, atau object tidak ditemukan, asset akan ditandai `failed`.

## Flow Preview / Download

Frontend tidak memakai `object_name` langsung. Untuk preview/download, minta URL sementara:

```http
GET /api/asset-presign/:asset_token
```

Response:

```json
{
  "status": "ok",
  "message": "Asset URL generated successfully",
  "data": {
    "asset": {
      "token": "98b5e767-0000-4000-9000-5f73d39cdd66",
      "status": "uploaded"
    },
    "url": "https://minio.example.com/...",
    "expires_in": 900
  }
}
```

Gunakan `url` untuk:

```tsx
<img src={url} alt={asset.original_filename} />
```

atau download:

```ts
window.open(url, "_blank")
```

Jangan cache URL ini sebagai data permanen. Simpan `asset_token`, lalu minta presigned URL baru saat diperlukan.

## Flow List Asset

Untuk menampilkan library asset milik user:

```http
GET /api/asset-list?status=uploaded&scope=documents
```

Filter opsional:

```text
status=uploaded
scope=documents
mime_type=image/png
extension=png
```

Backend otomatis membatasi hasil berdasarkan user yang login.

Response:

```json
{
  "status": "ok",
  "message": "Success",
  "data": {
    "assets": [
      {
        "token": "98b5e767-0000-4000-9000-5f73d39cdd66",
        "scope": "documents",
        "original_filename": "logo.png",
        "mime_type": "image/png",
        "extension": "png",
        "size": 48291,
        "status": "uploaded",
        "created_at": "2026-07-27T04:20:00Z"
      }
    ]
  }
}
```

List asset hanya berisi metadata. Jika butuh thumbnail/preview, minta `asset-presign` saat item terlihat atau saat user membuka detail.

## Flow Detail Asset

```http
GET /api/asset-detail/:asset_token
```

Gunakan ini saat butuh metadata lengkap asset, misalnya pada asset picker atau detail modal.

## Flow Delete Asset

```http
DELETE /api/asset-delete/:asset_token
```

Backend akan:

- menghapus object dari storage jika status asset `uploaded`,
- soft delete metadata asset.

Frontend setelah sukses:

- hapus item dari list lokal,
- hilangkan preview,
- jangan gunakan lagi `asset_token` tersebut.

## Error Handling Frontend

Recommended handling:

```text
400 invalid request      Tampilkan pesan validasi.
401 unauthorized         Refresh login/session.
403 forbidden            User tidak punya akses ke asset.
404 not found            Asset tidak ada atau bukan milik user.
503 unavailable          Storage tidak tersedia.
500 internal error       Tampilkan generic error dan retry.
```

Kasus upload khusus:

```text
PUT upload_url gagal
  Jangan call complete. Tawarkan retry selama belum expired.

complete gagal upload_expired
  Request upload baru dari awal.

complete gagal size_mismatch / mime_type_mismatch
  File yang diupload tidak sesuai metadata awal. Request upload baru.

asset status failed
  Jangan presign preview. Tampilkan "upload failed" dan opsi upload ulang.
```

## Contoh Helper Frontend

```ts
async function uploadAsset(file: File, scope = "documents") {
  const request = await api.post("/api/asset-upload-request", {
    original_filename: file.name,
    mime_type: file.type || "application/octet-stream",
    size: file.size,
    scope
  })

  const { asset, upload_url } = request.data.data

  const upload = await fetch(upload_url, {
    method: "PUT",
    headers: {
      "Content-Type": file.type || "application/octet-stream"
    },
    body: file
  })

  if (!upload.ok) {
    throw new Error("Upload to storage failed")
  }

  const complete = await api.post(`/api/asset-upload-complete/${asset.token}`)
  return complete.data.data.asset
}
```

Preview helper:

```ts
async function getAssetPreviewUrl(assetToken: string) {
  const response = await api.get(`/api/asset-presign/${assetToken}`)
  return response.data.data.url
}
```

## Integrasi Dengan Modul Lain

Modul lain sebaiknya hanya menyimpan `asset_token`.

Contoh document settings:

```json
{
  "logo_asset_token": "98b5e767-0000-4000-9000-5f73d39cdd66"
}
```

Saat perlu render/preview logo:

1. Ambil `logo_asset_token`.
2. Call `/api/asset-presign/:token`.
3. Pakai URL sementara untuk `<img>`.

Dengan pola ini asset management tetap menjadi satu pintu utama untuk upload, preview, download, dan delete.
