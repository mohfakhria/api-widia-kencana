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
| Antrean keluar | 64 pesan; klien yang tertinggal lebih jauh **diputus** |

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

### 2.3 Yang belum ada

Penyuntingan **belum dapat dilakukan lewat pesan mana pun**. Jenis di luar kedua
yang disebut di atas dibalas `unsupported_message_type`.

`cursor.hide` juga belum ada: kursor orang yang menggeser pointer keluar kanvas
baru hilang ketika ia menutup tab, atau ketika ia pergi.

Bentuk pesan untuk menyunting — `element.update`, `page.add`, dan sejenisnya —
**belum diputuskan**. Jangan menulis pengirimnya berdasarkan tebakan; bentuknya
akan ditulis di dokumen ini bersamaan dengan implementasinya, bukan sebelumnya.

Perubahan isi untuk sekarang dilakukan langsung ke kolom `documents.content` di
database.

---

## 3. Pesan dari server

Ringkasannya:

| Pesan | Dipicu oleh | Diterima | Kewajiban penerima |
|---|---|---|---|
| `snapshot` | klien mengirim `document.get` | hanya peminta | ganti seluruh keadaan kanvas |
| `presence` | daftar **orang** berubah, atau `document.get` | semua, atau hanya peminta — lihat 3.2 | perbarui daftar penyunting |
| `cursor` | denyut 50 ms bila ada yang berubah, atau `document.get` | semua bila penghuni ≥ 2; hanya peminta bila baru bergabung | ganti seluruh keadaan kursor |
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
| Denyut **50 ms**, hanya bila ada yang berubah | siaran ke semua penghuni |
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

Terukur: 521 `cursor.move` dalam tiga detik menghasilkan **61 pesan keluar** —
tepat 20 per detik — dengan latensi p99 45 ms.

**Gambarlah dengan interpolasi.** Denyut 50 ms terlihat melompat bila digambar
mentah, tetapi mulus dengan `transition` CSS atau lerp per frame — dan itu membuat
tunda rata-rata 25 ms tidak terasa.

### 3.4 `error`

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

Bila `document_unavailable` diabaikan, antarmuka akan menggantung dengan kanvas
kosong tanpa tanda apa pun — koneksinya hidup, hanya isinya yang tidak pernah
datang.

### 3.5 Versi

`version` adalah nomor revisi **dokumen** — satu dokumen, satu penghitung, naik
setiap kali ada perubahan yang berhasil diterapkan.

Rancangannya: **permintaan digerakkan frontend, penyelarasan digerakkan
backend.** Frontend menyimpan versi terakhir yang diterimanya, dan setiap kali
backend mengirim versi yang berbeda, frontend mengganti keadaannya dengan yang
baru. Tidak ada nomor urut yang perlu dikirim frontend.

Hari ini `version` hanya bergerak ketika dokumen kosong diisi panduan bawaan,
dan tidak ada siaran perubahan sama sekali. **Simpan nilainya, jangan bangun
logika rekonsiliasi apa pun di atasnya sekarang** — aturannya akan ditulis di
sini bersamaan dengan pesan penyuntingan.

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

Yang **tidak** dijamin:

- Urutan antar koneksi. Dua orang dapat menerima `presence` yang sama pada saat
  yang berbeda.
- Bahwa setiap perubahan menghasilkan tepat satu pesan. Server sengaja menahan
  siaran yang tidak mengubah apa pun — lihat tabel pemicu di 3.2.
- Bahwa setiap `cursor.move` menghasilkan satu `cursor`. Gerakan dipadatkan:
  yang keluar adalah posisi terakhir per denyut, bukan setiap gerakan.
- Bahwa setiap `cursor` sampai. Ia satu-satunya pesan yang boleh hilang, dan yang
  baru menggantikan yang masih mengantre.

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
        // dan gambar dengan interpolasi supaya denyut 50 ms tidak terlihat melompat.
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
