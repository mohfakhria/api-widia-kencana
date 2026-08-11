# Catatan untuk Claude

## Kontrak dengan frontend ada di luar repo ini

Fitur document design punya kontrak yang dipegang dua sistem berbeda. Prosa dan
kode gampang melenceng diam-diam — dan itu sudah pernah terjadi di repo ini:
satu bagian panduan sempat memperagakan bentuk `props` yang justru ditolak
bagian lain di dokumen yang sama.

Kontraknya kini **satu berkas bersama**, di repo terpisah:

```
../widia-kencana-docs-hub/docs/engineering/document-design-fe-be-alignment.md
```

Frontend menyunting berkas yang sama. Tidak ada salinan di repo ini, dan jangan
membuatnya — salinan yang tertinggal adalah persoalan yang berkas bersama ini
diadakan untuk menghapus.

**Mengubah bentuk pesan WebSocket, model isi dokumen, atau endpoint yang dipakai
editor berarti menyentuh tiga hal:**

1. Struct Go yang bersangkutan
2. Bagian yang bersangkutan di berkas bersama itu
3. Satu baris di **§8 Riwayat keputusan** pada berkas yang sama

Nomor 3 yang paling mudah terlupa dan paling merugikan — dan sekarang lebih
merugikan daripada sebelumnya. Dulu berkas kontraknya ikut dalam commit yang
sama dengan kodenya, jadi `git log` masih menyimpan jejak walau riwayatnya lupa
ditulis. Berkas bersama itu di luar repo mana pun, sehingga **tidak ada jejak
kedua**: bila §8 tidak ditambah, tidak ada satu pun cara frontend mengetahui
sesuatu berubah selain diberi tahu secara lisan.

Karena berkas itu di luar repo, ia juga tidak dapat ikut dalam commit yang
mengubah kodenya. Yang menggantikan jaminan tersebut adalah dua langkah, dan
keduanya wajib:

**Baca berkas itu SEBELUM mulai.** Frontend menyuntingnya di antara sesi, dan
tidak ada notifikasi apa pun ketika mereka melakukannya. Apa yang diingat dari
percakapan sebelumnya boleh jadi sudah tidak berlaku — §7 dapat berubah dari
"Menunggu" menjadi "Selesai", dan sebuah permintaan di §6 dapat sudah terjawab
oleh pihak sana. Mengerjakan sesuatu di atas ingatan yang basi menghasilkan
pekerjaan yang benar untuk kontrak yang sudah tidak ada.

**Sunting berkas itu SESUDAHNYA, pada saat yang sama dengan kodenya** — bukan
setelah pekerjaan Go dianggap selesai — lalu sebut di ringkasan bahwa ia sudah
disunting.

Berlaku untuk setiap perubahan yang menyentuh kontrak, sekecil apa pun.

## Sebelum mengubah document design

Baca `docs/engineering/document-design-architecture.md` lebih dulu. Di sana ada
sembilan invarian yang bila dilanggar tidak menghasilkan galat kompilasi maupun
pesan kesalahan — misalnya menambahkan mutex ke `Room`, atau membuat
`Subscriber.Send` menunggu I/O.

Bagian **Invarian** dan **Tetapan waktu** ikut diperbarui bila keputusannya
berubah. Tetapan waktu di sana saling bergantung; mengubah satu tanpa yang lain
membuang suntingan terakhir pengguna saat shutdown.

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
