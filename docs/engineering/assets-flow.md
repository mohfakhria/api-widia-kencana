# Asset Management Flow

Dokumen ini menjelaskan cara frontend menggunakan asset management API. Prinsip utamanya:

- Frontend tidak pernah menyimpan credential MinIO.
- Frontend upload file langsung ke object storage memakai presigned URL dari backend.
- Backend tetap menjadi gatekeeper untuk authorization, metadata asset, preview/download URL, dan lifecycle status.
- Frontend menyimpan dan mengirim `asset_token`, bukan numeric `id`.

## Endpoint Ringkas

Enam endpoint pertama membutuhkan header:

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

Satu endpoint lagi sengaja **di luar autentikasi**:

```text
GET    /api/asset-content/:token
```

Ia dituju langsung oleh tag `<img>`, yang tidak dapat mengirim header
`Authorization`. Token asetnya yang menjadi kredensial — UUID acak yang hanya
diperoleh lewat isi dokumen. Lihat [Menampilkan Gambar](#menampilkan-gambar).

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

Ini satu-satunya contoh di dokumen ini yang **utuh** — contoh lain sengaja
disingkat. Bentuk `asset` di bawah sama persis di semua endpoint yang
mengembalikannya:

```json
{
  "status": "ok",
  "message": "Success",
  "data": {
    "asset": {
      "token": "98b5e767-0000-4000-9000-5f73d39cdd66",
      "scope": "documents",
      "object_name": "documents/98b5e767-0000-4000-9000-5f73d39cdd66-logo.png",
      "original_filename": "logo.png",
      "stored_filename": "98b5e767-0000-4000-9000-5f73d39cdd66-logo.png",
      "mime_type": "image/png",
      "extension": "png",
      "size": 48291,
      "etag": "\"d41d8cd98f00b204e9800998ecf8427e\"",
      "status": "uploaded",
      "upload_method": "presigned_put",
      "is_private": true,
      "uploaded_at": "2026-07-27T04:22:00Z",
      "created_at": "2026-07-27T04:15:00Z",
      "updated_at": "2026-07-27T04:22:00Z"
    }
  }
}
```

**Enam field bersifat opsional dan HILANG dari JSON ketika tidak berlaku** —
bukan dikirim sebagai `null`: `presigned_expires_at`, `uploaded_at`,
`failed_at`, `deleted_at`, `failure_code`, `failure_message`. Aset `pending`
membawa `presigned_expires_at` tanpa `uploaded_at`; aset `uploaded` sebaliknya.
Frontend harus memeriksa keberadaan fieldnya, bukan nilainya.

## Flow Delete Asset

```http
DELETE /api/asset-delete/:asset_token
```

Backend akan:

- menghapus object dari storage jika status asset `uploaded`,
- soft delete metadata asset.

Response sukses — **tanpa `data`**, berbeda dari endpoint lain:

```json
{
  "status": "ok",
  "message": "Asset deleted successfully",
  "data": null
}
```

Frontend setelah sukses:

- hapus item dari list lokal,
- hilangkan preview,
- jangan gunakan lagi `asset_token` tersebut.

## Menampilkan Gambar

Untuk menampilkan gambar, **jangan** panggil `asset-presign` lebih dulu. Pakai
`asset-content` langsung sebagai sumber tag `<img>`:

```tsx
<img src={`/api/asset-content/${asset.token}`} alt={asset.original_filename} />
```

Balasannya bukan JSON melainkan pengalihan:

```http
HTTP/1.1 302 Found
Location: http://minio.example.com/widia-assets/documents/...?X-Amz-Signature=...
Cache-Control: no-store
```

Kenapa ini lebih baik daripada `asset-presign` untuk menampilkan:

- **URL-nya tetap dan tidak pernah kedaluwarsa.** Yang kedaluwarsa hanya sasaran
  pengalihannya, dan itu disusun ulang setiap permintaan. Tidak ada yang perlu
  disegarkan, dan tidak ada gambar yang mendadak gagal dimuat setelah lima belas
  menit.
- **Tanpa panggilan pendahuluan.** `asset-presign` menuntut satu permintaan per
  gambar sebelum apa pun tergambar.
- `Cache-Control: no-store` disetel sengaja: pengalihan yang tersimpan akan
  menunjuk tanda tangan yang keburu mati, dan gejalanya gambar yang gagal lalu
  sembuh sendiri — jenis kesalahan yang paling sulit dilacak.

Batasnya:

- **Tanpa header `Authorization`**, karena `<img>` tidak dapat mengirimnya.
  Tokennya yang menjadi kredensial: siapa pun yang memegang UUID itu dapat
  membaca gambarnya.
- Menjawab `400` bila asetnya belum `uploaded`, dan `404` bila sudah tidak ada.

Pakai `asset-presign` hanya ketika Anda membutuhkan URL bertandanya **sendiri**,
misalnya untuk mengunduh atau meneruskannya ke luar aplikasi.

## Error Handling Frontend

Recommended handling:

```text
400 invalid request      Tampilkan pesan validasi.
401 unauthorized         Refresh login/session.
403 forbidden            Bukan pengunggahnya. HANYA dari asset-upload-complete
                         dan asset-delete.
404 not found            Asset tidak ada, atau sudah soft deleted.
503 unavailable          Storage tidak tersedia.
500 internal error       Tampilkan generic error dan retry.
```

**Membaca dan mengubah punya wewenang yang berbeda, dan ini mudah disalahpahami.**
`asset-detail`, `asset-presign`, dan `asset-content` terbuka bagi siapa pun yang
login asal ia tahu tokennya — kepemilikan **tidak** diperiksa. Aset milik orang
lain menjawab `200`, bukan `403` maupun `404`. Itu disengaja: dokumen di
aplikasi ini milik bersama, dan gambar yang tokennya sudah masuk ke `assetToken`
sebuah elemen memang menjadi bagian dokumen bersama itu.

Yang dijaga kepemilikan hanya **mengubah** — `asset-upload-complete` dan
`asset-delete` — dan aset yang `uploaded_by`-nya kosong ditolak untuk siapa pun.
`asset-list` tetap hanya menampilkan milik sendiri.

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
2. Pakai `/api/asset-content/:token` langsung sebagai `src` — tanpa panggilan
   pendahuluan, dan tanpa yang perlu disegarkan. Lihat
   [Menampilkan Gambar](#menampilkan-gambar).

`asset-presign` hanya bila URL bertandanya sendiri yang dibutuhkan, misalnya
untuk mengunduh.

Dengan pola ini asset management tetap menjadi satu pintu utama untuk upload, preview, download, dan delete.
