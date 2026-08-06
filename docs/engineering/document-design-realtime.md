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
| Ekspor PDF | |

Isi dokumen **belum dapat diubah lewat API mana pun**. Semua jenis pesan selain
`document.get` dibalas `unsupported_message_type`. Bentuk pesannya sudah final,
jadi pengirimnya dapat ditulis sekarang meski balasannya masih penolakan.

> **Perubahan yang memutus kompatibilitas.** Model isi dokumen sekarang tertutup
> dan bersatuan titik (pt). Dokumen yang isinya masih memakai bentuk lama —
> dengan `props` — tidak akan terbaca dan koneksinya ditutup `1011`. Cara
> menyetel ulangnya ada di [bagian 6](#6-bentuk-isi-dokumen).

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

Model isi dokumen **tertutup**. Hanya properti yang benar-benar digambar backend
yang boleh ada; properti lain menghasilkan penolakan, bukan diabaikan diam-diam.

Ketertutupan itu bukan kerewelan. Backend sekarang ikut menggambar — untuk ekspor
PDF — sehingga properti yang tidak dipahaminya akan tampil di layar tetapi hilang
di hasil cetak, tanpa satu pun error maupun log. Menolaknya di pintu masuk membuat
perbedaan itu mustahil.

### Satuan: titik (pt)

Seluruh koordinat, ukuran, dan ukuran huruf memakai **titik**, yaitu 1/72 inci.
Tidak ada satuan yang ditulis di dalam data — angkanya saja.

Titik dipilih karena ia satuan asli PDF sekaligus satuan sah di CSS. Frontend
cukup menempelkan `pt`:

```js
const style = {
  left:      `${el.x}pt`,
  top:       `${el.y}pt`,
  width:     `${el.w}pt`,
  height:    `${el.h}pt`,
  fontSize:  `${el.fontSize}pt`,
};
```

Browser dan PDF karenanya membaca ukuran yang persis sama. Ukuran kertas ikut
dikonversi: A4 yang di database tersimpan `210 × 297 mm` menjadi
`595.276 × 841.89 pt`.

### Rangka

| Aturan | |
|---|---|
| `content.pages` | array; boleh kosong |
| Setiap halaman | objek dengan `id` tidak kosong dan `elements` (opsional) |
| Setiap elemen | objek dengan `id` dan `type` yang dikenal |
| Seluruh `id` | unik dalam satu dokumen, termasuk lintas halaman |

Ukuran halaman **tidak** disimpan di dalam isi dokumen. Ia diambil dari kertas
dokumen, sehingga seluruh halaman dijamin seukuran dan data tidak dapat
bertentangan dengan kertas yang dipilih pengguna.

### Properti bersama

| Properti | Tipe | Keterangan |
|---|---|---|
| `id` | string | tidak kosong, unik se-dokumen |
| `type` | string | `text`, `rect`, `line`, `image` |
| `x`, `y` | number | sudut kiri atas, relatif terhadap sudut kiri atas halaman |
| `w`, `h` | number | lebar dan tinggi; tidak boleh negatif kecuali pada `line` |

Nilai di luar ±100000 ditolak.

### `text`

| Properti | Tipe | Nilai yang sah |
|---|---|---|
| `text` | string | boleh kosong |
| `fontFamily` | string | nama keluarga yang terdaftar di backend |
| `fontSize` | number | > 0 |
| `fontWeight` | number | kelipatan 100, dari 100 sampai 900 |
| `fontStyle` | string | `normal`, `italic` |
| `color` | string | `#rgb` atau `#rrggbb` |
| `align` | string | `left`, `center`, `right`, `justify` |
| `lineHeight` | number | pengali ukuran huruf, misal `1.5` |
| `letterSpacing` | number | titik; boleh negatif |

Warna **hanya** menerima heksadesimal. Nama warna CSS, `rgba()`, dan `hsl()`
ditolak — menerimanya berarti backend harus menafsirkan sebagian CSS, dan
penafsiran sebagian itulah yang membuat layar dan cetak berbeda.

### `rect`

| Properti | Tipe | Keterangan |
|---|---|---|
| `fill` | string | warna isi; kosong berarti tanpa isi |
| `stroke` | string | warna garis tepi; kosong berarti tanpa garis |
| `strokeWidth` | number | ≥ 0; garis tepi hanya digambar bila > 0 |
| `radius` | number | ≥ 0; dibatasi separuh sisi terpendek |

### `line`

`w` dan `h` di sini bukan ukuran melainkan **simpangan ujung terhadap pangkal**,
sehingga keduanya boleh negatif. Garis mendatar berarti `h: 0`.

| Properti | Tipe | Keterangan |
|---|---|---|
| `stroke` | string | wajib diisi agar garis terlihat |
| `strokeWidth` | number | > 0 agar garis terlihat |

Ketebalan terbagi rata di kedua sisi jalurnya, sama seperti `stroke` pada SVG.

### `image`

| Properti | Tipe | Keterangan |
|---|---|---|
| `assetToken` | string | wajib; token aset yang sudah terunggah |
| `fit` | string | `contain`, `cover`, `fill` — sama artinya dengan `object-fit` |

Gambar menunjuk aset, bukan URL. Backend tidak pernah mengambil alamat yang
ditentukan lewat isi dokumen. Aset yang sudah terhapus dilewati saat ekspor —
sama seperti frontend yang juga tidak dapat menampilkannya.

Format yang dapat disematkan: JPEG, PNG, GIF.

### Nilai bawaan

Properti yang tidak disebutkan memakai nilai di bawah ini. **Frontend wajib
memakai nilai yang sama persis** — bila berbeda, elemen yang tidak menyebutkan
properti itu akan tampil berbeda antara layar dan cetak.

| Properti | Bawaan |
|---|---|
| `fontFamily` | `helvetica` |
| `fontSize` | `12` |
| `fontWeight` | `400` |
| `fontStyle` | `normal` |
| `color` | `#000000` |
| `align` | `left` |
| `lineHeight` | `1.2` |
| `letterSpacing` | `0` |
| `fit` | `contain` |

Cara paling aman adalah tidak bergantung pada nilai bawaan sama sekali: sebutkan
properti yang menentukan tata letak secara tegas pada setiap elemen teks.

### Contoh satu elemen

```json
{
  "id": "el_7f3a",
  "type": "text",
  "x": 59.53,
  "y": 68.03,
  "w": 476.22,
  "h": 50.51,
  "text": "Kaidah Dokumen yang Baik",
  "fontFamily": "helvetica",
  "fontSize": 24,
  "fontWeight": 700,
  "fontStyle": "normal",
  "color": "#111827",
  "align": "left",
  "lineHeight": 1.3
}
```

### Isi yang ditolak

Isi yang melanggar aturan di atas membuat koneksi ditutup `1011` dengan alasan
yang menyebutkan persis apa yang salah — misalnya
`json: unknown field "textShadow"` atau `duplicate element id "dup"`.

### Menyetel ulang dokumen berbentuk lama

Dokumen yang isinya masih memakai bentuk `props` tidak akan terbaca. Kosongkan
isinya agar panduan bawaan disusun ulang dalam bentuk baru:

```sql
UPDATE documents
SET content = '{"pages": []}',
    content_version = content_version + 1
WHERE content::text LIKE '%"props"%';
```

Jalankan saat tidak ada yang sedang membuka dokumennya.

---

## 7. Agar layar dan cetak sama

Ekspor PDF digambar mesin yang berbeda dari browser. Kesamaan hasilnya tidak
datang sendiri — ia ditegakkan oleh tiga hal, dan ketiganya **kewajiban
frontend**.

### Berkas font yang sama

Bukan sekadar nama keluarga yang sama. Helvetica di macOS dan Arial di Windows
punya lebar glif yang berbeda, dan selisih sekecil apa pun menumpuk menjadi
pemenggalan baris yang berbeda.

Backend memuat berkas font dari direktori `DESIGN_FONT_DIR` menurut manifes
`fonts.json`:

```json
{
  "families": [
    {
      "name": "inter",
      "faces": [
        { "weight": 400, "style": "normal", "file": "Inter-Regular.ttf" },
        { "weight": 700, "style": "normal", "file": "Inter-Bold.ttf" }
      ]
    }
  ]
}
```

Berkas **yang sama** harus disajikan ke frontend lewat `@font-face`, bukan diambil
dari Google Fonts maupun font sistem.

Keluarga `helvetica` selalu tersedia tanpa berkas apa pun karena metriknya melekat
pada spesifikasi PDF, tetapi ia punya dua batas:

- **Kesamaan tidak dijamin.** Browser akan memakai Helvetica atau Arial milik
  sistem, yang metriknya berbeda antar sistem operasi.
- **Hanya huruf Eropa Barat.** Keluarga inti memakai pengkodean Windows-1252.
  Tanda pisah `—`, kutip tipografis `“ ”`, dan huruf beraksen tercetak benar,
  tetapi aksara di luar itu — Tionghoa, Arab, Jepang — menjadi tanda tanya.

Ia ada supaya ekspor dapat dicoba sebelum font sungguhan disiapkan, bukan untuk
dipakai pada dokumen yang sungguh dicetak.

Ketebalan yang tidak terdaftar **menggagalkan ekspor**, tidak dibulatkan ke yang
terdekat. Penebalan buatan oleh browser sementara backend memakai penggantinya
adalah cara paling halus untuk menghasilkan dua tampilan yang berbeda.

### Kerning dan ligatur dimatikan

```css
font-kerning: none;
font-feature-settings: "liga" 0;
```

Browser secara bawaan merapatkan pasangan huruf tertentu dan menggabungkan `fi`
menjadi satu glif. Backend menjumlahkan lebar glif apa adanya. Tanpa kedua baris
di atas, lebar baris di layar dan di cetak tidak akan pernah sama.

### Perilaku kotak teks

```css
white-space: pre-line;
overflow: hidden;
overflow-wrap: normal;
```

| Aturan backend | Padanan CSS |
|---|---|
| `\n` menjadi pergantian baris, deretan spasi dipadatkan jadi satu | `white-space: pre-line` |
| Teks yang melebihi kotak dipotong | `overflow: hidden` |
| Kata yang lebih lebar dari kotak dibiarkan meluber, tidak dipatahkan | `overflow-wrap: normal` |
| Baris kosong tetap memakan satu tinggi baris | — |

### Aturan tata letak yang dipakai backend

**Pemenggalan baris** rakus: kata ditambahkan selama masih muat, dan pindah ke
baris berikutnya begitu tidak. Tanpa tanda hubung otomatis.

**Lebar teks** adalah jumlah lebar glif, ditambah `letterSpacing` setelah
**setiap** huruf termasuk yang terakhir — sama seperti CSS.

**Garis dasar baris pertama** memakai aturan half-leading milik CSS:

```
tinggiBaris  = fontSize × lineHeight
sisa         = tinggiBaris − (ascent + descent)
garisDasar   = y + sisa / 2 + ascent
```

`ascent` dan `descent` diambil dari FontDescriptor berkas font. Perlu dicatat
bahwa browser tidak selalu memakai sumber yang sama — sebagian mengambil metrik
`hhea`, sebagian `OS/2`. **Bila teks tampak bergeser vertikal secara konsisten
pada satu keluarga font, di sinilah tempat memeriksanya.**

**Rata penuh** hanya meregangkan jarak antar kata, dan **tidak** diterapkan pada
baris terakhir sebuah paragraf — sama seperti `text-align: justify` bawaan.

---

## 8. Panduan bawaan

Dokumen yang `pages`-nya masih kosong **diisi otomatis** satu halaman berisi
delapan blok teks berisi kaidah menyusun dokumen. Benih ini disimpan, sehingga
id elemennya tetap sama pada pembukaan berikutnya.

Dokumen yang sudah punya halaman tidak pernah disentuh — sekalipun halaman itu
belum berisi elemen.

Tata letaknya proporsional terhadap kertas: margin sisi 10% lebar, margin atas 8%
tinggi, dan seperlima halaman bawah sengaja dibiarkan kosong. Ukuran hurufnya
tidak proporsional melainkan mutlak — 24, 13, 11, dan 10.5 pt — karena yang
menentukan keterbacaan adalah jarak baca mata, bukan lebar kertas.

Setiap blok menyebut `fontFamily`, `fontSize`, `fontWeight`, `fontStyle`, `color`,
`align`, dan `lineHeight` secara tegas, tanpa mengandalkan satu pun nilai bawaan.
Benih ini karenanya dapat dipakai sebagai rujukan pertama saat menyelaraskan
tampilan frontend.

---

## 9. Contoh utuh

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

## 10. Ekspor PDF

```
POST /api/document-export/{documentToken}
Authorization: Bearer <access token>
```

Balasannya **bukan** amplop JSON seperti endpoint lain, melainkan berkas PDF
mentah:

```
200 OK
Content-Type: application/pdf
Content-Disposition: attachment; filename="Surat Jalan.pdf"; filename*=UTF-8''Surat%20Jalan.pdf
Cache-Control: no-store
```

Kegagalan tetap memakai amplop JSON yang sama dengan endpoint lain, karena saat
itu tidak ada berkas yang dikirim.

```js
async function exportPdf(documentToken, accessToken) {
  const res = await fetch(`${API}/api/document-export/${documentToken}`, {
    method: 'POST',
    headers: { Authorization: `Bearer ${accessToken}` },
  });

  if (!res.ok) {
    const { message } = await res.json();
    throw new Error(message);
  }

  const blob = await res.blob();
  const url = URL.createObjectURL(blob);
  const link = document.createElement('a');
  link.href = url;
  link.download = filenameFrom(res.headers.get('Content-Disposition')) ?? 'document.pdf';
  link.click();
  URL.revokeObjectURL(url);
}
```

### Yang dicetak adalah keadaan terkini, bukan yang tersimpan

Penyimpanan ke database bersifat tunda sampai dua detik. Karena itu ekspor
mengambil isi dari room bila dokumennya sedang dibuka seseorang, dan dari database
bila tidak.

Artinya **tidak perlu menunggu atau memaksa simpan sebelum mengekspor**. Menggeser
sebuah elemen lalu langsung menekan ekspor menghasilkan PDF yang sudah memuat
geseran itu.

### Kegagalan yang mungkin

| Status | Sebab |
|---|---|
| `400` | isi dokumen tidak sah, kertas memakai satuan yang tidak dikenal, atau font yang diminta tidak tersedia |
| `404` | dokumen tidak ada atau sudah dihapus |
| `503` | dokumen sedang dibuka tetapi room-nya bermasalah |

Pesan pada `400` menyebut persis apa yang kurang — misalnya
`font inter 700 normal is not available`. Itu jenis kegagalan yang diperbaiki
dengan mendaftarkan berkas fontnya, bukan dengan mencoba lagi.

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
