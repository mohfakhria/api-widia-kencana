# Catatan untuk Claude

## Kontrak dengan frontend berubah bersama-sama

Fitur document design punya kontrak yang dipegang dua sistem berbeda. Prosa dan
kode gampang melenceng diam-diam — dan itu sudah pernah terjadi di repo ini:
satu bagian panduan sempat memperagakan bentuk `props` yang justru ditolak
bagian lain di dokumen yang sama.

**Mengubah bentuk pesan WebSocket, model isi dokumen, atau endpoint yang dipakai
editor berarti satu commit menyentuh empat hal:**

1. Struct Go yang bersangkutan
2. `docs/engineering/document-design.d.ts` — kontrak dalam bentuk yang dapat dieksekusi
3. Bagian yang bersangkutan di `websocket-contract.md` atau `document-design.md`
4. Satu baris di **Riwayat perubahan** pada `websocket-contract.md`

Nomor 4 yang paling mudah terlupa dan paling merugikan. Tanpanya, satu-satunya
cara frontend mengetahui ada yang berubah adalah diberi tahu secara lisan.

### Pembagian antara kedua dokumen

> Kontrak menjelaskan **amplopnya**. Panduan menjelaskan **isinya**.

Apa pun yang punya field `type` atau kode penutupan masuk
`websocket-contract.md`. Apa pun tentang bentuk dokumen dan cara menggambarnya
masuk `document-design.md`. Jangan menjelaskan hal yang sama di keduanya —
tautkan saja.

## Verifikasi

```bash
gofmt -w . && go vet ./... && go build ./...
```

Jangan membuat berkas `*_test.go`. Verifikasi dilakukan manual oleh pemilik
repo; yang berguna dari sisi Claude adalah menyebutkan **apa yang perlu dicoba**
setelah perubahan — terutama jalur kegagalan dan tempat yang sensitif terhadap
konkurensi yang layak dijalankan dengan `-race`.

Berkas sementara untuk memeriksa perilaku boleh ditaruh di `tmp/` (sudah masuk
`.gitignore`) dan dihapus setelah selesai.

## Kebiasaan lain di repo ini

- Komentar kode ditulis dalam bahasa Indonesia, menjelaskan **kenapa**, bukan apa.
- Migration adalah SQL murni per tabel di `migration/`, dijalankan manual. Berkasnya
  memakai `CREATE TABLE IF NOT EXISTS`, jadi kolom yang ditambahkan belakangan
  tidak ikut terpasang pada database lama — sebutkan `ALTER TABLE`-nya di README.
- Koleksi Postman digabung ulang dengan `go run docs/collection/merge.go` setelah
  berkas per fitur diubah.
- Jangan commit tanpa diminta.
