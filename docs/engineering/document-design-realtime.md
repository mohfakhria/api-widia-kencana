# Document Design Realtime — Panduan Integrasi Frontend

Dokumen ini menjelaskan kontrak antara frontend dan backend untuk editor dokumen
realtime, **sesuai implementasi yang sudah berjalan hari ini**.

## Status implementasi

| Sudah bisa dipakai | Belum ada |
|---|---|
| Terbitkan tiket handshake | Penerapan perubahan (`element.update`, `page.add`, dan sejenisnya) |
| Buka koneksi WebSocket | Siaran perubahan antar klien |
| Minta isi dokumen (`document.get`) | Daftar siapa yang sedang membuka (`presence`) |
| Terima `snapshot` | |
| Panduan bawaan untuk dokumen kosong | |

Isi dokumen **belum dapat diubah lewat API mana pun**. Semua jenis pesan selain
`document.get` dibalas `unsupported_message_type`. Bentuk pesannya sudah final,
jadi pengirimnya dapat ditulis sekarang meski balasannya masih penolakan.

---

## 1. Terbitkan tiket

`WebSocket` di browser tidak dapat mengirim header `Authorization`, sehingga
access token tidak bisa ikut saat handshake. Tiket ini penggantinya.

```http
POST {{base_url}}/api/document-design-ticket/{documentToken}
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

`documentToken` adalah `documents.token`, diperoleh dari `GET /api/document-list`
atau `GET /api/document-detail/:token`.

### Sifat tiket

| | |
|---|---|
| Umur | **30 detik** |
| Pemakaian | **sekali**, hangus begitu handshake dilakukan |
| Terikat | satu dokumen; dipakai untuk dokumen lain ditolak |
| Kuota | maksimal **5 tiket hidup per user**; permintaan keenam menggusur yang tertua |

Karena itu: **ambil tiket tepat sebelum menyambung, dan ambil tiket baru pada
setiap penyambungan ulang.** Jangan menyimpannya di state.

### Kegagalan

| Status | Arti |
|---|---|
| `401` | Access token tidak ada atau tidak sah — jalankan refresh token dulu |
| `400` | `documentToken` bukan UUID |
| `404` | Dokumen tidak ada atau berstatus `deleted` |

---

## 2. Buka koneksi

```
ws://localhost:8081/document-design/{documentToken}?ticket={ticket}
```

Header `Origin` diperiksa terhadap `FRONTEND_URL` di backend. Bila tidak cocok,
handshake ditolak **HTTP 403 sebelum upgrade**.

Saat `APP_ENV=local`, seluruh host loopback ikut diizinkan tanpa perlu menyetel
`FRONTEND_URL` setiap kali port atau subdomain berganti:

```
localhost           localhost:<port>
*.localhost         *.localhost:<port>      ← portal.localhost:3000, dan seterusnya
127.0.0.1           127.0.0.1:<port>
```

Pelonggaran ini **hanya** berlaku pada `APP_ENV=local` — staging maupun produksi
tetap terkurung pada `FRONTEND_URL`. Ia juga tetap terbatas pada loopback:
`.localhost` adalah TLD yang dicadangkan RFC 6761 dan tidak dapat didaftarkan
publik, sehingga `localhost.evil.com` tetap ditolak. Backend mencatat peringatan
saat start bila pelonggaran ini aktif.

> **Browser tidak dapat membaca status HTTP dari handshake WebSocket yang gagal.**
> Penolakan `Origin` akan tampak sama persis dengan jaringan mati. Bila koneksi
> gagal tanpa `CloseEvent` yang berarti, tersangka pertama adalah `FRONTEND_URL`
> yang tidak cocok.

Kegagalan lain sengaja disampaikan **setelah** upgrade sebagai close frame,
justru supaya alasannya terbaca frontend lewat `CloseEvent.code` dan
`CloseEvent.reason`.

### Close code

| Code | Reason | Kapan | Yang harus frontend lakukan |
|---|---|---|---|
| `1008` | `invalid or expired design ticket` | saat handshake | Terbitkan tiket baru, sambung ulang |
| `1013` | `too many concurrent design connections` | saat handshake | Sudah 10 koneksi untuk user ini; tunggu, lalu coba lagi |
| `1011` | alasan spesifik dari server | **di tengah sesi** | Berhenti; tampilkan pesannya, jangan sambung ulang otomatis |
| `1003` | `only JSON text frames are supported` | kapan saja | Bug frontend: ada frame biner terkirim |
| `1000` | — | — | Penutupan normal |

`1011` **tidak** terjadi saat membuka koneksi. Ia muncul ketika room berhenti
dapat melayani selagi frontend sudah menjadi anggota — misalnya isinya tidak lagi
dapat disimpan karena dokumennya dihapus orang lain. Server memutus koneksi
beserta alasannya, karena membiarkan orang menyunting sesuatu yang tidak akan
pernah tersimpan lebih buruk daripada menghentikannya.

Room yang bermasalah **sebelum** frontend menjadi anggota — misalnya isinya cacat
di database — tidak menutup koneksi; ia dibalas pesan `error` berkode
`document_unavailable`. Lihat bagian berikutnya.

### `1006` punya dua sebab yang sangat berbeda

Handshake yang ditolak — `Origin` tidak diizinkan, atau backend tidak terjangkau
— gagal **sebelum** upgrade, sehingga tidak ada close frame sama sekali. Browser
melaporkannya sebagai `1006`, sama persis dengan jaringan yang putus di tengah
sesi. Keduanya butuh tindakan yang berlawanan.

Pembedanya murah: catat apakah `onopen` pernah menyala.

| | Sebab | Tindakan |
|---|---|---|
| `1006` setelah `onopen` menyala | Jaringan putus, server berhenti | Backoff seperti biasa |
| `1006` tanpa `onopen` pernah menyala | Handshake ditolak — `Origin`, URL, atau backend mati | **Berhenti setelah 2–3 percobaan** dan tampilkan pesan konfigurasi |

Yang kedua tidak akan sembuh dengan menunggu. Mencoba terus hanya menghasilkan
percobaan tanpa akhir tanpa petunjuk apa pun di sisi browser — sementara log
backend sebenarnya sudah menyebutkan `Origin` yang ditolak beserta `Host` yang
diharapkan.

Catatan: saat backend berhenti secara wajar, klien pun kemungkinan besar melihat
`1006`, bukan `1000` — socket sudah tertutup sebelum close frame sempat terkirim.
Tindakannya sama, jadi tidak ada yang perlu dibedakan.

### Batas koneksi

| | |
|---|---|
| Ukuran satu pesan | **1 MB** |
| Koneksi bersamaan per user | **10** |
| Ping dari server | tiap 30 detik, tenggang pong 10 detik |
| Antrean keluar | 64 pesan; klien yang tertinggal lebih jauh **diputus** |

Ping dan pong ditangani browser secara otomatis — tidak ada yang perlu ditulis.
Tetapi koneksi yang tidak responsif akan diputus paling lambat 40 detik setelah
benar-benar mati.

---

## 3. Minta isi dokumen

**Server diam sampai diminta.** Membuka koneksi tidak mengirim apa pun.

```jsonc
{ "type": "document.get" }
```

Boleh dipanggil **berulang** pada koneksi yang sama. Itulah jalur pemulihan:
frontend yang menyadari keadaannya tertinggal cukup meminta ulang tanpa memutus
koneksi.

Klien baru menjadi anggota room — dan karenanya penerima siaran — pada saat
permintaan ini diproses, bukan saat socket terbuka.

### Pesan lain

Semua pesan wajib berupa **JSON dalam text frame**. Tidak ada nomor urut; lihat
bagian [Versi](#5-versi) untuk penggantinya.

| Kiriman | Balasan |
|---|---|
| Jenis selain `document.get` | `error` `unsupported_message_type` |
| Tanpa field `type` | `error` `missing_message_type` |
| JSON rusak | `error` `malformed_message` |
| Frame biner | koneksi ditutup `1003` |

---

## 4. Terima snapshot

```jsonc
{
  "type": "snapshot",
  "version": 1,
  "content": {
    "pages": [
      {
        "id": "0b928a8a-…",
        "elements": [
          {
            "id": "650abe0f-…",
            "type": "text",
            "x": 21, "y": 23.76, "w": 168, "h": 17.82,
            "props": {
              "text": "Kaidah Dokumen yang Baik",
              "fontSize": 24,
              "fontWeight": "bold",
              "align": "left"
            }
          }
        ]
      }
    ]
  }
}
```

### Pesan error

```jsonc
{ "type": "error", "code": "unsupported_message_type", "message": "…" }
```

> **Pesan `error` tidak menutup koneksi.** Ia jalur yang terpisah sama sekali dari
> close code, dan menyambung ulang **bukan** tindakan yang tepat untuk salah satu
> pun di antaranya.

| Code | Arti | Tindakan |
|---|---|---|
| `document_unavailable` | Room tidak dapat melayani `document.get`; pesannya menyebut sebabnya, misalnya `document content is malformed` | Tunggu belasan detik, lalu kirim `document.get` lagi. Room yang bermasalah dibuang setelah masa tenggangnya dan dimuat ulang, sehingga percobaan berikutnya sering berhasil. **Jangan** sambung ulang koneksinya |
| `malformed_message` | JSON tidak dapat diurai, sehingga backend tidak tahu pesan mana yang dimaksud | Buang antrean optimistik, kirim `document.get` untuk memulai bersih |
| `missing_message_type` | Field `type` kosong atau tidak ada | Bug frontend |
| `unsupported_message_type` | Jenis pesan belum didukung backend | Bug frontend, atau fitur yang memang belum ada |

Bila `document_unavailable` diabaikan, antarmuka akan menggantung dengan kanvas
kosong tanpa tanda apa pun — koneksinya hidup, hanya isinya yang tidak pernah
datang.

---

## 5. Versi

`version` adalah nomor revisi **dokumen** — satu dokumen, satu penghitung. Setiap
perubahan yang berhasil diterapkan menaikkannya satu, apa pun jenisnya.

Frontend menyimpan versi lokalnya dan membandingkannya dengan setiap pesan yang
masuk:

```
diterima == lokal + 1   → terapkan, lokal = diterima
diterima <= lokal       → sudah diterapkan, abaikan
diterima  > lokal + 1   → ADA YANG TERLEWAT → kirim document.get
```

Kasus ketiga yang paling penting. Menerapkan delta di atas keadaan yang salah
membuat kanvas berbeda dari kebenaran di server, dan **tidak ada yang akan
memberi tahu** sampai pengguna menyadari gambarnya salah.

Kasus kedua yang menggantikan nomor urut: siaran atas perubahan frontend sendiri
akan kembali, dan karena sudah digambar optimistik, penerapannya tidak mengubah
apa pun.

> Ini menuntut satu hal dari frontend: **logika penerapan wajib idempoten.**
> Menyetel `x=120` dua kali harus sama dengan sekali. Yang butuh perhatian nanti
> hanya penambahan elemen — bila id-nya sudah ada, abaikan, jangan gandakan.

Sampai penerapan perubahan dibangun, `version` hanya bergerak saat dokumen kosong
diberi panduan bawaan.

---

## 6. Bentuk isi dokumen

Backend **tidak memaksakan** bentuk elemen. Struktur `props` sepenuhnya keputusan
frontend, dan apa pun yang dikirim akan kembali utuh — termasuk field yang backend
tidak kenal, dan termasuk literal angka apa adanya (`0.50` tetap `0.50`).

Yang **wajib** hanya rangkanya:

| Aturan | |
|---|---|
| `content.pages` | array |
| Setiap halaman | objek dengan `id` bertipe string dan tidak kosong |
| Setiap elemen | objek dengan `id` dan `type`, keduanya string dan tidak kosong |
| Seluruh `id` | unik dalam satu dokumen, termasuk lintas halaman |

Halaman tanpa `elements` sah. Field lain di tingkat mana pun lewat apa adanya.

Isi yang melanggar aturan di atas membuat koneksi ditutup `1011` dengan alasan
yang menyebutkan persis apa yang salah — misalnya `duplicate element id "dup"`.

---

## 7. Panduan bawaan

Dokumen yang `pages`-nya masih kosong **diisi otomatis** satu halaman berisi
delapan blok teks berisi kaidah menyusun dokumen. Benih ini disimpan, sehingga
id elemennya tetap sama pada pembukaan berikutnya.

Dokumen yang sudah punya halaman tidak pernah disentuh — sekalipun halaman itu
belum berisi elemen.

Tata letaknya proporsional terhadap kertas: margin sisi 10% lebar, margin atas 8%
tinggi, dan seperlima halaman bawah sengaja dibiarkan kosong.

> **Dua asumsi yang perlu dikonfirmasi frontend.**
>
> **Satuan.** Koordinat dan ukuran memakai satuan kertas itu sendiri — milimeter
> untuk A4, inci untuk Letter — sedangkan `fontSize` memakai angka mutlak. Bila
> kanvas bekerja dalam piksel, angkanya perlu dikonversi.
>
> **Nama properti.** `text`, `fontSize`, `fontWeight`, `align` disusun dari
> toolbar editor. Backend tidak memvalidasi isi `props`, jadi nama yang keliru
> tidak menimbulkan error — benihnya saja yang tidak terbaca.

---

## 8. Contoh utuh

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

    if (message.type === 'snapshot') {
      renderCanvas(message.content);
      localVersion = message.version;
      return;
    }

    if (message.type !== 'error') return;

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
  };

  ws.onclose = (event) => {
    switch (event.code) {
      case 1008:                                   // tiket basi — selalu ambil tiket baru
        return retryWithNewTicket();
      case 1013:                                   // batas koneksi
        return setTimeout(retryWithNewTicket, 5000);
      case 1011:                                   // room berhenti melayani
      case 1003:                                   // frame biner: bug frontend
        return showError(event.reason);
      default:
        if (!everOpened) {
          // Handshake ditolak — Origin, URL, atau backend mati. Menunggu tidak
          // akan menyembuhkannya; log backend menyebutkan sebabnya.
          return showConfigurationError();
        }
        return scheduleReconnect();   // jaringan putus: backoff biasa
    }
  };

  return ws;
}
```

Setiap jalur penyambungan ulang — termasuk backoff — **wajib menerbitkan tiket
baru**. Tiket lama sudah hangus atau kedaluwarsa, dan memakainya kembali hanya
menghasilkan `1008` yang menyesatkan.

```js
function retryWithNewTicket() {
  return openDesign(documentToken, accessToken);   // selalu mulai dari langkah 1
}
```

---

## Empat jebakan yang paling sering menggigit

**Tiket hanya berlaku 30 detik dan sekali pakai.** Setiap penyambungan ulang wajib
mengambil tiket baru.

**Kegagalan `Origin` tidak terlihat dari browser.** Tidak ada status, tidak ada
`reason`. Periksa `FRONTEND_URL` — atau pastikan `APP_ENV=local` bila frontend
dijalankan di host loopback selain itu. Log backend menyebutkan `Origin` yang
ditolak beserta `Host` yang diharapkan.

**Access token harus segar dulu.** `401` pada endpoint tiket berarti alur refresh
token yang bermasalah, bukan WebSocket.

**Kuota 5 tiket per user.** Dua alur yang sama-sama meminta tiket akan saling
menggusur, dan gejalanya muncul sebagai `1008` pada tiket yang baru saja diambil.
