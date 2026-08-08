# Kontrak WebSocket — Document Design

Apa yang melintas di socket: setiap pesan, apa yang memicunya, siapa yang
menerimanya, dan apa yang wajib dilakukan penerima.

> **Dokumen ini menjelaskan amplopnya. Isinya dijelaskan di
> [`document-design.md`](document-design.md).**
>
> Bentuk `snapshot.content` — halaman, elemen, dan propertinya — beserta cara
> menggambarnya ada di sana, dan sengaja tidak diulang di sini. Yang punya field
> `type` atau kode penutupan ada di dokumen ini.

---

## Riwayat perubahan

Kolom terakhir menjawab pertanyaan yang sebenarnya: **apakah saya perlu
mengubah sesuatu?**

| Tanggal | Perubahan | Perlu tindakan |
|---|---|---|
| 2026-08-08 | Antrean keluar per klien dinaikkan 64 → **256** pesan | Tidak ada, bentuk pesannya tetap. Klien punya toleransi tersendat empat kali lebih panjang sebelum diputus; throttle saat menggeser tetap dianjurkan |
| 2026-08-08 | `page.update` beserta siaran `page.updated`; field baru `hidden`/`locked` pada halaman dan `locked`/`groupId` pada elemen | Tambahan, **tetapi**: `element.update` mengganti elemen seutuhnya, jadi mulai kirim balik `locked` dan `groupId` pada setiap update — yang tidak disertakan akan terhapus. Halaman `hidden` **tidak ikut tercetak** |
| 2026-08-08 | `page.create`, `page.delete`, `page.reorder` beserta siaran `page.*` dan kode error `page_rejected` | Tambahan. Halaman kini dapat ditambah dan dibuang saat sesi berjalan — **berhenti** menganggap daftar halaman tetap sepanjang koneksi. Halaman terakhir tidak dapat dihapus |
| 2026-08-08 | **Penyuntingan lewat WebSocket**: `element.create`, `element.update`, `element.delete`, `element.reorder`, beserta siaran `element.*` dan kode error `element_rejected` | Fitur baru. `version` kini bergerak setiap ada perubahan — **mulai pakai** deteksi celah nomor di [3.6](#36-versi). Berhenti menyunting `documents.content` langsung di database |
| 2026-08-07 | Denyut siaran `cursor` dilonggarkan 50 ms → **70 ms** | Tidak ada, bentuk pesannya tetap. Sesuaikan hanya bila durasi interpolasi dipatok pada 50 ms |
| 2026-08-07 | Pesan `cursor` dan `cursor.move` — kursor langsung antar penyunting | Tambahan. Tangani bila ingin menampilkan kursor orang lain; abaikan dengan aman bila belum |
| 2026-08-06 | Pesan `presence` — siapa yang sedang membuka dokumen | Tambahan. Tangani bila ingin menampilkan tumpukan avatar; abaikan dengan aman bila belum |
| 2026-08-06 | Field `page` pada `snapshot` — ukuran halaman dalam titik | Tambahan, tetapi **berhenti** menghitung ukuran kertas sendiri dari endpoint detail dokumen |
| 2026-08-06 | `GET /api/document-design-fonts` | Tambahan. Isi pilihan font dari sini, jangan dipatok di frontend |
| 2026-08-06 | `POST /api/document-export/:token` | Tambahan. Menghasilkan PDF dari keadaan terkini |
| 2026-08-06 | Model isi dokumen ditutup, satuan menjadi titik | **Memutus kompatibilitas.** Isi berbentuk lama ditolak dan koneksi ditutup `1011`. Lihat daftar periksa penyesuaian di panduan |

Perubahan pada bentuk pesan selalu muncul di tabel ini pada commit yang sama
dengan perubahan kodenya.

---

## 1. Handshake

### 1.1 Tiket

`WebSocket` di browser tidak dapat mengirim header `Authorization`, sehingga
access token tidak bisa ikut saat handshake. Tiket ini penggantinya.

```http
POST /api/document-design-ticket/{documentToken}
Authorization: Bearer <access_token>
```

```jsonc
{
  "status": "ok",
  "message": "Design ticket issued successfully",
  "data": {
    "design_ticket": {
      "ticket": "zl0c3GKGkm0o90SnaaMBb_vHVgL2c5qktRbzCiiU0vI",
      "expires_in": 30
    }
  }
}
```

`documentToken` adalah `documents.token`, dari `GET /api/document-list` atau
`GET /api/document-detail/:token`.

| Sifat tiket | |
|---|---|
| Umur | **30 detik** |
| Pemakaian | **sekali**, hangus begitu handshake dilakukan |
| Terikat | satu dokumen; dipakai untuk dokumen lain ditolak |
| Kuota | maksimal **5 tiket hidup per user**; permintaan keenam menggusur yang tertua |

**Ambil tiket tepat sebelum menyambung, dan ambil tiket baru pada setiap
penyambungan ulang.** Jangan menyimpannya di state.

| Kegagalan | Arti |
|---|---|
| `401` | Access token tidak ada atau tidak sah — jalankan refresh token dulu |
| `400` | `documentToken` bukan UUID |
| `404` | Dokumen tidak ada atau berstatus `deleted` |

Tiket juga membawa nama pemiliknya, dipakai server untuk menyusun daftar
kehadiran. Tidak ada yang perlu dikirim frontend untuk itu.

### 1.2 Membuka koneksi

```
ws://localhost:8081/document-design/{documentToken}?ticket={ticket}
```

Header `Origin` diperiksa terhadap `FRONTEND_URL`. Bila tidak cocok, handshake
ditolak **HTTP 403 sebelum upgrade** — dan browser tidak dapat membaca status
itu, sehingga gejalanya sama persis dengan jaringan mati.

Saat `APP_ENV=local`, seluruh host loopback ikut diizinkan tanpa perlu menyetel
`FRONTEND_URL` setiap kali port atau subdomain berganti:

```
localhost      localhost:<port>
*.localhost    *.localhost:<port>     ← portal.localhost:3000, dan seterusnya
127.0.0.1      127.0.0.1:<port>
```

Pelonggaran ini **hanya** berlaku pada `APP_ENV=local`; staging dan produksi
tetap terkurung pada `FRONTEND_URL`. Ia juga tetap terbatas pada loopback:
`.localhost` adalah TLD yang dicadangkan RFC 6761 dan tidak dapat didaftarkan
publik, sehingga `localhost.evil.com` tetap ditolak.

### 1.3 Batas dan denyut

| | |
|---|---|
| Ukuran satu pesan | **1 MB** |
| Koneksi bersamaan per user | **10** |
| Ping dari server | tiap 30 detik, tenggang pong 10 detik |
| Antrean keluar | 256 pesan; klien yang tertinggal lebih jauh **diputus** |

Ping dan pong ditangani browser otomatis — tidak ada yang perlu ditulis. Koneksi
yang tidak responsif diputus paling lambat 40 detik setelah benar-benar mati.

Seluruh pesan wajib berupa **JSON di dalam text frame**. Frame biner menutup
koneksi dengan `1003`.

---

## 2. Pesan dari klien

### 2.1 `document.get`

```jsonc
{ "type": "document.get" }
```

| | |
|---|---|
| Kapan dikirim | setelah `onopen`, dan kapan pun klien menyadari keadaannya tertinggal |
| Balasan | `snapshot`, lalu `presence` |
| Bila ditolak | `error` dengan kode `document_unavailable` |

**Server diam sampai diminta.** Membuka koneksi tidak mengirim apa pun.

Boleh dipanggil **berulang** pada koneksi yang sama — itulah jalur pemulihan,
dan tidak perlu memutus koneksi untuk memakainya.

Klien baru menjadi anggota room pada saat permintaan ini diproses, bukan saat
socket terbuka. Sebelum itu ia belum terhitung hadir, dan tidak akan menerima
siaran apa pun.

### 2.2 `cursor.move`

```jsonc
{ "type": "cursor.move", "page": "18f10d20-42f7-4a4c-80df-63dc46ab0022", "x": 120.5, "y": 340.25 }
```

| | |
|---|---|
| Kapan dikirim | saat pointer bergerak di atas kanvas |
| Balasan | **tidak ada** — kursor disiarkan pada denyut, bukan dibalas per pesan |
| Bila cacat | dijatuhkan diam-diam |

`page` **wajib** dan tidak boleh kosong; pesan tanpanya dijatuhkan. Koordinat
dalam titik, sama seperti seluruh isi dokumen.

**Pengirim harus sudah menjadi anggota**, yaitu sudah mengirim `document.get`.
Kursor dari koneksi yang belum meminta dokumen diabaikan sepenuhnya — orang itu
belum muncul di `presence`, sehingga kursornya akan sampai ke layar orang lain
sebagai id yang tidak dapat dipetakan ke nama maupun warna mana pun.

**Tidak pernah dibalas, bahkan saat ditolak.** Membalas kesalahan pada laju
puluhan kali per detik hanya akan membanjiri klien dengan pesan yang tidak dapat
ia perbuat apa-apa, sekaligus memenuhi antrean keluarnya sendiri.

**Batasi lajunya dengan throttle, bukan debounce.** Debounce menunggu jeda —
dipakai di sini ia menahan kiriman sampai mouse berhenti, sehingga kursor orang
lain diam terus lalu melompat. `requestAnimationFrame` adalah pembatas yang
paling sederhana dan sudah selaras dengan saat browser punya posisi terbaru.

Mengirim lebih sering daripada denyut tidak menghasilkan satu pun pesan tambahan
bagi orang lain: backend memadatkan gerakan menjadi satu posisi terakhir per
orang. Yang didapat dari membatasi hanyalah lalu lintas naik yang lebih hemat.

### 2.3 `element.create`

```jsonc
{
  "type": "element.create",
  "page": "18f10d20-42f7-4a4c-80df-63dc46ab0022",
  "element": { "id": "…", "type": "rect", "x": 40, "y": 40, "w": 120, "h": 80, "fill": "#eef" }
}
```

| | |
|---|---|
| Kapan dikirim | pengguna menambahkan sesuatu ke kanvas |
| Balasan | `element.created` ke **semua** penghuni, termasuk pengirim |
| Bila ditolak | `error` dengan kode `element_rejected` |

`page` **wajib** — hanya pesan ini yang membawanya, karena hanya ini yang perlu
menyebut ke mana elemennya dipasang.

`id` elemen dibuat **frontend**, dan wajib unik se-dokumen termasuk lintas
halaman. Id yang sudah dipakai ditolak; pakai UUID.

Elemen baru masuk di **akhir** daftar halaman itu, yaitu paling atas.

### 2.4 `element.update`

```jsonc
{ "type": "element.update", "element": { "id": "…", "type": "rect", "x": 60, "y": 40, "w": 120, "h": 80 } }
```

| | |
|---|---|
| Kapan dikirim | pengguna menggeser, mengubah ukuran, atau menyunting properti |
| Balasan | `element.updated` ke semua penghuni |
| Bila sasarannya sudah lenyap | **tidak ada apa-apa** — lihat di bawah |

**Kirim elemen UTUH, bukan hanya field yang berubah.** Elemen berid sama diganti
seluruhnya. Field yang tidak Anda sertakan kembali ke nilai bawaannya — ini
bukan patch.

**Letaknya dalam urutan gambar tidak berubah.** Yang memindahkan hanya
`element.reorder`.

Elemen yang tidak ada **didiamkan**: tanpa error, tanpa siaran, tanpa kenaikan
`version`. Itu disengaja. Orang lain yang menghapus elemen tepat sebelum
suntingan Anda tiba adalah kejadian biasa, dan Anda toh sedang menerima
`element.deleted` untuknya — keadaannya menyatu dengan sendirinya.

### 2.5 `element.delete`

```jsonc
{ "type": "element.delete", "id": "…" }
```

Cukup id; halaman dicari backend. Elemen yang sudah tidak ada didiamkan, dengan
alasan yang sama seperti `element.update`.

### 2.6 `element.reorder`

```jsonc
{ "type": "element.reorder", "id": "…", "index": 2 }
```

Urutan elemen di dalam sebuah halaman **adalah** urutan gambar: yang belakangan
menutupi yang terdahulu. `element.update` tidak memindahkan apa pun, jadi inilah
satu-satunya cara menaikkan elemen yang tertutup.

`index` dihitung dari nol di dalam halaman elemen itu sendiri. Nol berarti paling
bawah.

**Index di luar batas tidak ditolak melainkan dijepit** ke ujung terdekat, karena
batasnya berubah setiap kali ada yang menambah atau menghapus elemen. Siaran
`element.reordered` membawa letak **sesungguhnya** — pakai angka itu, bukan yang
Anda kirim.

### 2.7 Menahan laju saat menggeser

Menggeser elemen menghasilkan puluhan `element.update` per detik bila dikirim
apa adanya. Berbeda dari `cursor`, siaran perubahan **tidak boleh hilang**,
sehingga klien yang tertinggal terlalu jauh diputus, bukan dilewati.

**Throttle ke sekitar 20–30 per detik selama menggeser, lalu kirim satu
`element.update` terakhir yang pasti saat tombol dilepas.** Mata tidak dapat
membedakannya dari 60, dan tanpa itu klien lain yang tersendat sesaat akan
kehilangan koneksinya.

### 2.8 `page.create`

```jsonc
{ "type": "page.create", "id": "9c1f…", "index": 2 }
```

| | |
|---|---|
| Kapan dikirim | pengguna menambahkan halaman |
| Balasan | `page.created` ke **semua** penghuni, termasuk pengirim |
| Bila ditolak | `error` dengan kode `page_rejected` |

`id` dibuat **frontend** dan wajib unik se-dokumen. Pakai UUID.

**`index` boleh tidak disertakan, dan itu berarti "di akhir".** Perhatikan bahwa
`index: 0` adalah hal yang berbeda — ia berarti "di paling depan". Jangan
mengirim `0` ketika yang Anda maksud "di akhir".

Halaman baru **selalu kosong**. Belum ada penyalinan halaman.

Ditolak bila id sudah dipakai, atau bila dokumen sudah punya **200 halaman**.

### 2.9 `page.update`

```jsonc
{ "type": "page.update", "id": "9c1f…", "hidden": true, "locked": false }
```

| | |
|---|---|
| Kapan dikirim | pengguna menyembunyikan, menampilkan, mengunci, atau membuka halaman |
| Balasan | `page.updated` ke **semua** penghuni, termasuk pengirim |
| Bila muatannya kurang | `error` dengan kode `malformed_message` |

**Kedua boolean WAJIB, selalu keduanya.** Pesan yang hanya menyebut salah satunya
**ditolak**, bukan diperlakukan sebagai `false`. Tanpa aturan itu, mengirim
`{"hidden": true}` akan sekalian **membuka kunci** halaman itu diam-diam, dan
perubahan yang tidak Anda minta tersiar ke semua orang sebagai perubahan yang sah.

**Tidak ada field `elements`.** Ini perbedaan pokok dari `element.update`. Elemen
adalah daun sehingga dikirim utuh; halaman memuat elemen, dan mengirim halaman
utuh berarti setiap perubahan `hidden` ikut menimpa seluruh isinya — dua orang
yang menyunting elemen di halaman itu akan saling menghapus pekerjaan.

Nilai yang **sudah sama persis** tidak menghasilkan siaran dan tidak menaikkan
`version`. Aman mengirimnya berulang, tetapi tidak ada gunanya.

#### `hidden` — tidak ikut tercetak

Halaman tersembunyi **dilewati ekspor seluruhnya**, termasuk aset di atasnya yang
tidak akan diunduh. Ia bukan sekadar disembunyikan dari editor.

Boleh menyembunyikan halaman terlihat yang terakhir. Dokumen yang seluruh
halamannya tersembunyi menghasilkan PDF berisi **satu halaman kosong** — sama
seperti dokumen yang memang belum punya halaman, dan dengan alasan yang sama: itu
memang yang akan tercetak.

#### `locked` — tidak ditegakkan backend

`locked` adalah **penanda**, bukan penjagaan. Backend menyimpannya dan
menyiarkannya, tetapi tidak menolak satu pun penyuntingan atas halaman atau elemen
yang terkunci. Ia mencegah kecelakaan di editor, bukan mencegah klien yang memang
mengirim perubahan.

### 2.10 `page.delete`

```jsonc
{ "type": "page.delete", "id": "9c1f…" }
```

Membuang halaman **beserta seluruh elemen di atasnya**. Siarannya hanya menyebut
id halaman; buang halaman itu dan semua elemennya ikut.

**Halaman terakhir tidak dapat dihapus** — permintaannya dibalas `page_rejected`.
Sembunyikan atau matikan tombolnya ketika dokumen tinggal satu halaman.

> Alasannya bukan kerewelan: dokumen yang tersimpan tanpa halaman dianggap kosong
> saat dimuat berikutnya, lalu **ditimpa panduan bawaan** — sehingga template yang
> dirawat lama berubah menjadi teks tutorial, dan itu terjadi bukan saat
> penghapusannya melainkan ketika orang berikutnya membuka dokumen.

Halaman yang memang sudah tidak ada didiamkan, sama seperti elemen.

### 2.11 `page.reorder`

```jsonc
{ "type": "page.reorder", "id": "9c1f…", "index": 0 }
```

`index` dihitung dari nol dan **wajib** di sini — berbeda dari `page.create`,
karena memindahkan tanpa menyebut tujuan tidak berarti apa-apa. Di luar batas
dijepit, dan siaran `page.reordered` membawa letak sesungguhnya.

### 2.12 Yang belum ada

`cursor.hide` belum ada: kursor orang yang menggeser pointer keluar kanvas baru
hilang ketika ia menutup tab, atau ketika ia pergi.

Penyalinan halaman belum ada — halaman baru selalu kosong.

**Undo/redo bukan urusan backend.** Di editor kolaboratif, undo yang benar adalah
membatalkan langkah **sendiri**, bukan langkah terakhir siapa pun — dan itu
dibangun frontend dari riwayat lokal, lalu dikirim sebagai pesan penyuntingan
biasa.

Jenis di luar yang disebut di bab ini dibalas `unsupported_message_type`.

---

## 3. Pesan dari server

Ringkasannya:

| Pesan | Dipicu oleh | Diterima | Kewajiban penerima |
|---|---|---|---|
| `snapshot` | klien mengirim `document.get` | hanya peminta | ganti seluruh keadaan kanvas |
| `presence` | daftar **orang** berubah, atau `document.get` | semua, atau hanya peminta — lihat 3.2 | perbarui daftar penyunting |
| `cursor` | denyut 70 ms bila ada yang berubah, atau `document.get` | semua bila penghuni ≥ 2; hanya peminta bila baru bergabung | ganti seluruh keadaan kursor |
| `element.created` | `element.create` diterapkan | **semua**, pengirim ikut | sisipkan elemen di akhir halaman itu |
| `element.updated` | `element.update` diterapkan | **semua**, pengirim ikut | ganti elemen berid sama, jangan pindahkan urutannya |
| `element.deleted` | `element.delete` diterapkan | **semua**, pengirim ikut | buang elemen berid itu |
| `element.reordered` | `element.reorder` diterapkan | **semua**, pengirim ikut | pindahkan elemen ke `index` |
| `page.created` | `page.create` diterapkan | **semua**, pengirim ikut | sisipkan halaman kosong di `index` |
| `page.updated` | `page.update` diterapkan | **semua**, pengirim ikut | setel `hidden` dan `locked` halaman itu |
| `page.deleted` | `page.delete` diterapkan | **semua**, pengirim ikut | buang halaman itu **beserta seluruh elemennya** |
| `page.reordered` | `page.reorder` diterapkan | **semua**, pengirim ikut | pindahkan halaman ke `index` |
| `error` | permintaan ditolak | hanya peminta | **jangan** sambung ulang |

### 3.1 `snapshot`

```jsonc
{
  "type": "snapshot",
  "version": 3,
  "page": { "width": 595.276, "height": 841.89 },
  "content": {
    "pages": [
      {
        "id": "18f10d20-42f7-4a4c-80df-63dc46ab0022",
        "elements": [ /* … */ ]
      }
    ]
  }
}
```

| Field | Arti |
|---|---|
| `version` | nomor revisi dokumen; lihat [3.4](#35-versi) |
| `page` | ukuran satu halaman **dalam titik**, berlaku untuk seluruh halaman dokumen ini |
| `content` | isi kanvas — bentuknya di [`document-design.md`](document-design.md) |

`page` adalah satu-satunya sumber ukuran kanvas yang perlu dipakai. Endpoint
detail dokumen mengembalikan ukuran dalam satuan asli kertas — milimeter untuk
A4, inci untuk Continuous Form — dan mengonversinya sendiri hanya menambah satu
tempat yang bisa meleset.

Isi yang tidak lolos validasi tidak menghasilkan `snapshot` melainkan menutup
koneksi dengan `1011`, beserta alasan yang menyebut persis apa yang salah.

### 3.2 `presence`

```jsonc
{
  "type": "presence",
  "users": [
    { "id": "7",  "name": "Fahmi Ardiyanto" },
    { "id": "12", "name": "Dewi Anggraini" }
  ]
}
```

Tidak ada field jumlah tersendiri — `users.length` sudah menjawabnya, termasuk
untuk menampilkan "+6" pada tumpukan avatar. Warna juga tidak dikirim; turunkan
sendiri dari `id` supaya orang yang sama berwarna sama di semua layar.

**Yang didaftar orang, bukan koneksi.** Satu orang dengan tiga tab tetap satu
entri, dan ia baru hilang ketika tab terakhirnya tertutup. Ini bukan kehalusan
teoretis: frontend membuka lebih dari satu koneksi tiap kali halaman dimuat,
sehingga menghitung koneksi akan menampilkan satu orang sebagai beberapa
penyunting.

Urutannya menurut nama, jadi susunan avatar tidak berubah-ubah sendiri.

**Kapan pesan ini dipicu:**

| Kejadian | Siapa yang menerima |
|---|---|
| Seseorang bergabung, dan ia belum hadir sebelumnya | **semua**, termasuk yang baru bergabung |
| Seseorang membuka tab kedua | hanya tab baru itu — bagi yang lain daftarnya tidak berubah |
| `document.get` diulang sebagai pemulihan | hanya yang meminta |
| Seseorang menutup satu dari beberapa tabnya | **tidak ada** — ia masih hadir |
| Seseorang menutup tab terakhirnya | semua yang tersisa |

### 3.3 `cursor`

```jsonc
{
  "type": "cursor",
  "cursors": [
    { "id": "1", "page": "18f10d20-42f7-4a4c-80df-63dc46ab0022", "x": 88,    "y": 120 },
    { "id": "2", "page": "18f10d20-42f7-4a4c-80df-63dc46ab0022", "x": 120.5, "y": 340.25 }
  ]
}
```

**Seluruh kursor sekaligus**, bukan hanya yang baru bergerak. Ganti seluruh
keadaan kursor dengan isi array ini; jangan menggabungkannya satu per satu.

**Termasuk kursor penerima sendiri.** Saring `id` sendiri saat menggambar.
Muatannya satu untuk semua penerima — menyaring di server berarti menyusun
payload berbeda untuk tiap orang, dan itu mengalikan pekerjaan penyandian
sebanyak jumlah penghuni.

Diurutkan menurut `id`, sehingga isinya dapat diulang.

Nama dan warna tidak ikut. Nama sudah ada dari [`presence`](#32-presence), dan
warna diturunkan frontend dari `id` supaya orang yang sama berwarna sama di semua
layar.

**Setiap `id` di sini dijamin ada juga di `presence`.** Kursor hanya dimiliki
anggota, dan hilang begitu orangnya benar-benar pergi.

**Dikunci per orang, bukan per koneksi.** Satu orang dengan beberapa tab punya
satu kursor, yaitu posisi dari tab yang terakhir bergerak.

| Kapan pesan ini datang | |
|---|---|
| Denyut **70 ms**, hanya bila ada yang berubah | siaran ke semua penghuni |
| Baru bergabung lewat `document.get` | langsung, hanya ke pendatang |
| Ada yang pergi | kursornya hilang dari daftar berikutnya |
| Tidak ada yang bergerak | **tidak ada pesan sama sekali** |
| Penghuni kurang dari dua | tidak disiarkan |

Kiriman ke pendatang tidak menunggu denyut. Denyut hanya menyiarkan saat ada yang
berubah, jadi orang yang bergabung ketika semua sedang diam tidak akan pernah
melihat satu kursor pun tanpa kiriman langsung itu.

**Pesan ini boleh hilang.** Berbeda dari `snapshot`, `presence`, dan `error` yang
memutus klien tertinggal terlalu jauh, `cursor` justru dilewati bila antrean
klien penuh — dan siaran baru **menggantikan** siaran kursor yang masih mengantre
alih-alih menumpuk di belakangnya. Klien yang tersendat karenanya menerima posisi
terkini saat ia menyusul, bukan tumpukan posisi basi.

Terukur pada denyut 50 ms: 521 `cursor.move` dalam tiga detik menghasilkan
**61 pesan keluar**, tepat 20 per detik, dengan latensi p99 45 ms. Angka itu
berskala dengan denyutnya.

**Gambarlah dengan interpolasi.** Denyut 70 ms terlihat melompat bila digambar
mentah, tetapi mulus dengan `transition` CSS atau lerp per frame — dan itu membuat
tunda rata-rata 25 ms tidak terasa.

### 3.4 Siaran perubahan

```jsonc
{ "type": "element.created",   "version": 43, "page": "…", "element": { /* elemen utuh */ } }
{ "type": "element.updated",   "version": 44, "element": { /* elemen utuh */ } }
{ "type": "element.deleted",   "version": 45, "id": "…" }
{ "type": "element.reordered", "version": 46, "id": "…", "index": 2 }
```

**Pengirimnya ikut menerima siarannya sendiri.** Ini berbeda dari `cursor`, dan
disengaja: tanpanya nomor `version` pengirim tertinggal setiap kali ia menyunting,
lalu siaran pertama dari orang lain terlihat melompat dan ia memuat ulang seluruh
dokumen tanpa sebab.

Terapkan optimistis di layar lebih dulu; siaran yang kembali cuma menaikkan nomor
versi Anda — sekaligus menjadi tanda bahwa perubahan itu sungguh diterima.

**Bentuk lampau membedakannya dari perintah.** `element.created` adalah
pemberitahuan; `element.create` adalah permintaan. Keduanya tidak pernah tertukar.

| Field | Ada pada | Arti |
|---|---|---|
| `version` | semuanya | nomor revisi **setelah** perubahan ini; lihat [3.6](#36-versi) |
| `page` | `created` | halaman tempat elemen dipasang |
| `element` | `created`, `updated` | elemen **utuh**, bukan hanya yang berubah |
| `id` | `deleted`, `reordered` | elemen yang dimaksud |
| `index` | `reordered` | letak **sesungguhnya** setelah dijepit, bukan yang diminta |

Siaran halaman mengikuti pola yang sama persis:

```jsonc
{ "type": "page.created",   "version": 60, "id": "…", "index": 2 }
{ "type": "page.updated",   "version": 61, "id": "…", "hidden": true, "locked": false }
{ "type": "page.deleted",   "version": 62, "id": "…" }
{ "type": "page.reordered", "version": 63, "id": "…", "index": 0 }
```

Pada `page.updated`, **kedua boolean selalu ada** — termasuk ketika bernilai
`false`. Siaran yang mengatakan "sekarang tidak tersembunyi" harus dapat
dibedakan dari siaran yang tidak menyebut apa-apa.

`page.deleted` **tidak menyebutkan elemen di atasnya satu per satu.** Anda sudah
tahu isi halaman itu dari `snapshot` dan siaran sebelumnya; buang halamannya, dan
seluruh elemennya ikut.

Id elemen yang ikut terbuang kembali **bebas** — backend tidak menyimpan jejak id
yang pernah dipakai, jadi ia akan menerima `element.create` yang memakainya lagi.
Pakai UUID dan pertanyaan itu tidak pernah muncul.

Pada `page.created` dan `page.reordered`, `index` **selalu ada**, termasuk ketika
nilainya nol. Halaman di posisi paling depan adalah keadaan yang sah dan sering.

Siaran ini **tidak pernah dijatuhkan**. Klien yang antrean keluarnya penuh
diputus — menerima sebagian menghasilkan kanvas yang salah tanpa ada yang tahu,
sedangkan diputus lalu `document.get` menghasilkan yang benar.

Putusnya terjadi **tanpa close frame**, sehingga di browser ia muncul sebagai
`1006` dan tidak dapat dibedakan dari jaringan yang mati. Perlakukan sama:
sambung ulang, lalu `document.get`. Ini berbeda dari `1011`, yang berarti berhenti
dan jangan sambung ulang.

Perubahan yang tidak berlaku — `element.update` atas elemen yang sudah dihapus,
misalnya — **tidak menghasilkan siaran apa pun dan tidak menaikkan `version`**.
Tidak ada celah nomor yang tercipta karenanya.

### 3.5 `error`

```jsonc
{ "type": "error", "code": "unsupported_message_type", "message": "…" }
```

> **Pesan `error` tidak menutup koneksi.** Ia jalur yang terpisah sama sekali
> dari close code, dan menyambung ulang **bukan** tindakan yang tepat untuk
> salah satu pun di antaranya.

| Code | Arti | Tindakan |
|---|---|---|
| `document_unavailable` | Room tidak dapat melayani `document.get`; pesannya menyebut sebabnya | Tunggu belasan detik, kirim `document.get` lagi. Room bermasalah dibuang setelah masa tenggangnya dan dimuat ulang, sehingga percobaan berikutnya sering berhasil. **Jangan** sambung ulang koneksinya |
| `malformed_message` | JSON tidak dapat diurai | Buang antrean optimistik, kirim `document.get` untuk memulai bersih |
| `missing_message_type` | Field `type` kosong atau tidak ada | Bug frontend |
| `unsupported_message_type` | Jenis pesan belum didukung backend | Bug frontend, atau fitur yang memang belum ada |
| `page_rejected` | Permintaan halaman ditolak: id sudah dipakai, batas 200 halaman tersentuh, atau **halaman terakhir** hendak dihapus | Batalkan perubahan optimistik Anda. Untuk halaman terakhir, matikan tombolnya alih-alih menunggu penolakan |
| `element_rejected` | Muatan penyuntingan ditolak: elemen tidak sah, properti tak dikenal, halaman tidak ada, atau id sudah dipakai | **Batalkan perubahan optimistik Anda.** Pesannya menyebut persis apa yang salah — ini hampir selalu bug frontend, bukan keadaan yang dapat dicoba lagi |

Bila `document_unavailable` diabaikan, antarmuka akan menggantung dengan kanvas
kosong tanpa tanda apa pun — koneksinya hidup, hanya isinya yang tidak pernah
datang.

### 3.6 Versi

`version` adalah nomor revisi **dokumen** — satu dokumen, satu penghitung, naik
setiap kali ada perubahan yang berhasil diterapkan.

Rancangannya: **permintaan digerakkan frontend, penyelarasan digerakkan
backend.** Frontend menyimpan versi terakhir yang diterimanya, dan setiap kali
backend mengirim versi yang berbeda, frontend mengganti keadaannya dengan yang
baru. Tidak ada nomor urut yang perlu dikirim frontend.

Setiap perubahan yang **berhasil diterapkan** menaikkannya tepat satu. Yang
tidak berlaku — menyunting elemen yang sudah dihapus orang lain — tidak
menaikkannya sama sekali.

**Deteksi celah, jangan bangun rekonsiliasi.** Simpan versi terakhir yang Anda
terima. Bila siaran berikutnya membawa nomor yang bukan `versi+1`, ada yang
terlewat — kirim `document.get` dan ganti seluruh keadaan Anda dengan snapshot
yang datang. Itu saja; tidak ada penggabungan, tidak ada pemutaran ulang.

```js
if (msg.version !== lastVersion + 1) { send({ type: "document.get" }); return; }
apply(msg);
lastVersion = msg.version;
```

Backend tidak menyimpan riwayat dan tidak dapat mengirim ulang siaran yang
terlewat. `document.get` adalah satu-satunya jalur pemulihan, dan ia memang cukup.

Nomor ini juga naik sekali di luar penyuntingan: ketika dokumen kosong diisi
panduan bawaan saat pertama dibuka.

---

## 4. Jaminan urutan

Yang benar-benar dijamin backend, dan boleh diandalkan:

- **Pada klien yang baru bergabung, urutannya selalu `snapshot` → `presence` →
  `cursor`.** Ketiganya dimasukkan ke antrean keluar pada langkah yang sama oleh
  server. `cursor` dilewati bila memang belum ada kursor sama sekali.
- **Tidak akan pernah ada delta sebelum snapshot yang menjadi dasarnya.**
  Keanggotaan room dan pengiriman snapshot terjadi pada langkah yang sama, jadi
  mustahil ada siaran perubahan yang menyalip snapshot pertama.
- **Urutan pesan dalam satu koneksi dipertahankan.** Tidak ada jalur yang dapat
  menyalip jalur lain.
- **Semua penghuni menerima siaran perubahan dalam urutan yang sama.** Satu
  orchestrator menerapkan perubahan satu per satu, jadi urutan `version` adalah
  urutan penerapan — sama bagi semua orang, tanpa kecuali.
- **Siaran perubahan tidak pernah dijatuhkan.** Klien yang tidak sanggup
  mengikutinya diputus, bukan dilewati.

Yang **tidak** dijamin:

- Urutan antar koneksi. Dua orang dapat menerima `presence` yang sama pada saat
  yang berbeda.
- Bahwa setiap perubahan menghasilkan tepat satu pesan. Server sengaja menahan
  siaran yang tidak mengubah apa pun — lihat tabel pemicu di 3.2.
- Bahwa setiap `cursor.move` menghasilkan satu `cursor`. Gerakan dipadatkan:
  yang keluar adalah posisi terakhir per denyut, bukan setiap gerakan.
- Bahwa setiap `cursor` sampai. Ia satu-satunya pesan yang boleh hilang, dan yang
  baru menggantikan yang masih mengantre.
- Bahwa setiap pesan penyuntingan menghasilkan siaran. Yang sasarannya sudah
  lenyap didiamkan — tanpa error, tanpa siaran, tanpa kenaikan `version`.
- Bahwa daftar halaman tetap sepanjang koneksi. Halaman dapat muncul dan hilang
  kapan saja; jangan menyimpan indeks halaman sebagai identitas, pakai id-nya.
- Bahwa dua orang yang menyunting elemen yang **sama** berakhir dengan hasil yang
  masing-masing kira. Berlaku menang-terakhir per elemen: yang tiba belakangan
  menang seluruhnya. Tidak ada penggabungan properti.

---

## 5. Penutupan koneksi

### 5.1 Close code

| Code | Reason | Kapan | Tindakan frontend |
|---|---|---|---|
| `1008` | `invalid or expired design ticket` | saat handshake | Terbitkan tiket baru, sambung ulang |
| `1013` | `too many concurrent design connections` | saat handshake | Sudah 10 koneksi untuk user ini; tunggu, lalu coba lagi |
| `1011` | alasan spesifik dari server | **di tengah sesi** | Berhenti; tampilkan pesannya, jangan sambung ulang otomatis |
| `1003` | `only JSON text frames are supported` | kapan saja | Bug frontend: ada frame biner terkirim |
| `1000` | — | — | Penutupan normal |

`1011` **tidak** terjadi saat membuka koneksi. Ia muncul ketika room berhenti
dapat melayani selagi frontend sudah menjadi anggota — misalnya isinya tidak
lagi dapat disimpan karena dokumennya dihapus orang lain. Server memutus koneksi
beserta alasannya, karena membiarkan orang menyunting sesuatu yang tidak akan
pernah tersimpan lebih buruk daripada menghentikannya.

Room yang bermasalah **sebelum** frontend menjadi anggota tidak menutup koneksi;
ia dibalas `error` berkode `document_unavailable`.

### 5.2 `1006` punya dua sebab yang berlawanan

Handshake yang ditolak — `Origin` tidak diizinkan, atau backend tidak terjangkau
— gagal **sebelum** upgrade, sehingga tidak ada close frame sama sekali. Browser
melaporkannya sebagai `1006`, sama persis dengan jaringan yang putus di tengah
sesi. Keduanya butuh tindakan yang berlawanan.

Pembedanya murah: catat apakah `onopen` pernah menyala.

| | Sebab | Tindakan |
|---|---|---|
| `1006` **setelah** `onopen` menyala | Jaringan putus, server berhenti | Backoff seperti biasa |
| `1006` **tanpa** `onopen` pernah menyala | Handshake ditolak — `Origin`, URL, atau backend mati | Berhenti setelah 2–3 percobaan, tampilkan pesan konfigurasi |

Yang kedua tidak akan sembuh dengan menunggu. Mencoba terus hanya menghasilkan
percobaan tanpa akhir tanpa petunjuk apa pun di sisi browser — sementara log
backend sudah menyebutkan `Origin` yang ditolak beserta `Host` yang diharapkan.

Catatan: saat backend berhenti secara wajar, klien pun kemungkinan besar melihat
`1006` alih-alih `1000`, karena socket sudah tertutup sebelum close frame sempat
terkirim. Tindakannya sama, jadi tidak ada yang perlu dibedakan.

### 5.3 Penyambungan ulang

**Setiap** jalur penyambungan ulang — termasuk backoff — wajib menerbitkan tiket
baru. Tiket lama sudah hangus atau kedaluwarsa, dan memakainya kembali hanya
menghasilkan `1008` yang menyesatkan.

---

## 6. Contoh utuh

```js
const API = 'http://localhost:8081';
const WS = 'ws://localhost:8081';

async function openDesign(documentToken, accessToken) {
  // 1. Tiket selalu diambil baru, tepat sebelum menyambung.
  const res = await fetch(`${API}/api/document-design-ticket/${documentToken}`, {
    method: 'POST',
    headers: { Authorization: `Bearer ${accessToken}` },
  });
  if (res.status === 401) throw new Error('access token kedaluwarsa');
  if (!res.ok) throw new Error(`gagal menerbitkan tiket: ${res.status}`);

  const { data } = await res.json();
  const ticket = data.design_ticket.ticket;

  const ws = new WebSocket(`${WS}/document-design/${documentToken}?ticket=${ticket}`);
  let localVersion = -1;

  // Pembeda antara handshake yang ditolak dan jaringan yang putus di tengah.
  let everOpened = false;

  // 2. Server diam sampai diminta.
  ws.onopen = () => {
    everOpened = true;
    ws.send(JSON.stringify({ type: 'document.get' }));
  };

  ws.onmessage = (event) => {
    const message = JSON.parse(event.data);

    switch (message.type) {
      case 'snapshot':
        // page berisi ukuran halaman dalam titik; dibutuhkan untuk menyiapkan kanvas.
        renderCanvas(message.content, message.page);
        localVersion = message.version;
        return;

      case 'presence':
        // Daftar orang, bukan koneksi. Selalu datang setelah snapshot pertama.
        renderAvatars(message.users);
        return;

      case 'cursor':
        // Seluruh kursor sekaligus — ganti, jangan gabungkan. Saring id sendiri,
        // dan gambar dengan interpolasi supaya denyutnya tidak terlihat melompat.
        renderCursors(message.cursors.filter((c) => c.id !== myUserId));
        return;

      case 'error':
        // Pesan error TIDAK menutup koneksi. Menyambung ulang bukan jawabannya.
        switch (message.code) {
          case 'document_unavailable':
            // Room bermasalah; ia dimuat ulang setelah masa tenggangnya habis.
            return setTimeout(() => ws.send(JSON.stringify({ type: 'document.get' })), 15000);
          case 'malformed_message':
            // Tidak ada yang dapat dikorelasikan; mulai bersih.
            return ws.send(JSON.stringify({ type: 'document.get' }));
          default:
            return console.warn('server menolak:', message.code, message.message);
        }

      default:
        // Jenis pesan baru diabaikan dengan aman, bukan dianggap kesalahan.
        return;
    }
  };

  ws.onclose = (event) => {
    switch (event.code) {
      case 1008:                       // tiket basi — selalu ambil tiket baru
        return retryWithNewTicket();
      case 1013:                       // batas koneksi
        return setTimeout(retryWithNewTicket, 5000);
      case 1011:                       // room berhenti melayani
      case 1003:                       // frame biner: bug frontend
        return showError(event.reason);
      default:
        if (!everOpened) {
          // Handshake ditolak — Origin, URL, atau backend mati. Menunggu tidak
          // akan menyembuhkannya; log backend menyebutkan sebabnya.
          return showConfigurationError();
        }
        return scheduleReconnect();    // jaringan putus: backoff biasa
    }
  };

  return ws;
}

function retryWithNewTicket() {
  return openDesign(documentToken, accessToken);   // selalu mulai dari langkah 1
}
```

Perhatikan cabang `default` pada `onmessage`: jenis pesan yang belum dikenal
diabaikan, bukan dianggap kesalahan. Itu yang membuat penambahan pesan baru di
masa depan tidak merusak klien yang belum diperbarui.

---

## Empat jebakan yang paling sering menggigit

**Tiket hanya berlaku 30 detik dan sekali pakai.** Setiap penyambungan ulang
wajib mengambil tiket baru.

**Kegagalan `Origin` tidak terlihat dari browser.** Tidak ada status, tidak ada
`reason`. Periksa `FRONTEND_URL` — atau pastikan `APP_ENV=local` bila frontend
dijalankan di host loopback selain itu. Log backend menyebutkan `Origin` yang
ditolak beserta `Host` yang diharapkan.

**Access token harus segar dulu.** `401` pada endpoint tiket berarti alur
refresh token yang bermasalah, bukan WebSocket.

**Kuota 5 tiket per user.** Dua alur yang sama-sama meminta tiket akan saling
menggusur, dan gejalanya muncul sebagai `1008` pada tiket yang baru saja
diambil.
