# Document Design — Panduan Integrasi Frontend

Kontrak antara frontend dan backend untuk editor dokumen, **sesuai implementasi
yang berjalan hari ini**. Apa pun yang belum ada disebutkan terang-terangan
sebagai belum ada, bukan dijanjikan bentuknya.

> **Baru pertama kali menyesuaikan editor yang sudah ada?** Lompat ke
> [daftar periksa penyesuaian](#daftar-periksa-penyesuaian-frontend) di bagian
> akhir, lalu kembali ke sini untuk perinciannya.

## Peta singkat

```
GET  /api/document-design-fonts           →  font yang tersedia
POST /api/document-design-ticket/:token   →  tiket sekali pakai, 30 detik
WS   /document-design/:token?ticket=      →  buka sesi
     kirim { "type": "document.get" }     →  terima { "type": "snapshot", … }
                                          →  lalu { "type": "presence", … }
POST /api/document-export/:token          →  berkas PDF
```

| Sudah bisa dipakai | Belum ada |
|---|---|
| Tiket handshake dan koneksi WebSocket | Penyuntingan lewat API mana pun |
| Minta isi dokumen dan terima snapshot | Siaran perubahan antar klien |
| Daftar siapa yang sedang membuka | Riwayat dan pembatalan perubahan |
| Panduan bawaan untuk dokumen kosong | Kursor dan pilihan orang lain |
| Ekspor PDF | |

**Isi dokumen belum dapat diubah lewat API.** Setiap jenis pesan selain
`document.get` dibalas `unsupported_message_type`. Bentuk pesan untuk penyuntingan
**belum diputuskan** — jangan menulis pengirimnya berdasarkan tebakan.

Perubahan isi untuk sekarang dilakukan langsung ke kolom `documents.content` di
database.

---

## 1. Model isi dokumen

Ini bagian yang paling menentukan, jadi ia lebih dulu: seluruh sisa panduan ini
hanya cara memindahkan dan menggambar bentuk di bawah.

Modelnya **tertutup**. Hanya properti yang benar-benar digambar backend yang boleh
ada; properti lain ditolak, bukan diabaikan diam-diam.

Ketertutupan itu bukan kerewelan. Backend ikut menggambar sekarang — untuk ekspor
PDF — sehingga properti yang tidak dipahaminya akan tampil di layar lalu hilang di
hasil cetak, tanpa satu pun error maupun log. Menolaknya di pintu masuk membuat
perbedaan itu mustahil terjadi.

### 1.1 Satuan: titik (pt)

Seluruh koordinat, ukuran, dan ukuran huruf memakai **titik**, yaitu 1/72 inci.
Tidak ada satuan yang ditulis di dalam data — angkanya saja.

Titik dipilih karena ia satuan asli PDF **sekaligus** satuan sah di CSS. Frontend
cukup menempelkan `pt`:

```js
const style = {
  left:     `${el.x}pt`,
  top:      `${el.y}pt`,
  width:    `${el.w}pt`,
  height:   `${el.h}pt`,
  fontSize: `${el.fontSize}pt`,
};
```

Browser dan PDF karenanya membaca ukuran yang persis sama. Cara mengubah titik
menjadi piksel layar, mengatur zoom, dan mengembalikan koordinat pointer ke titik
ada di [2.1](#21-ukuran-halaman-zoom-dan-koordinat).

Ukuran kertas ikut dikonversi dari satuan aslinya di database — A4 yang tersimpan
`210 × 297 mm` menjadi `595.276 × 841.89 pt` — dan hasilnya dikirim bersama
snapshot, jadi frontend tidak perlu mengonversi apa pun.

### 1.2 Rangka

```jsonc
{
  "pages": [
    { "id": "…", "elements": [ /* … */ ] }
  ]
}
```

| Aturan | |
|---|---|
| `pages` | array; boleh kosong |
| Setiap halaman | `id` tidak kosong, `elements` opsional |
| Setiap elemen | `id` tidak kosong dan `type` yang dikenal |
| Seluruh `id` | unik dalam satu dokumen, **termasuk lintas halaman** |

Id unik lintas halaman supaya elemen dapat berpindah halaman tanpa risiko bentrok.

**Ukuran halaman tidak disimpan di dalam isi dokumen.** Ia diambil dari kertas
dokumen, sehingga seluruh halaman dijamin seukuran dan data tidak dapat
bertentangan dengan kertas yang dipilih pengguna. Frontend menerimanya lewat
field `page` pada pesan `snapshot`, sudah dalam titik.

Urutan elemen adalah urutan gambar: yang belakangan menutupi yang terdahulu, sama
seperti urutan DOM.

### 1.3 Properti bersama

| Properti | Tipe | Keterangan |
|---|---|---|
| `id` | string | tidak kosong, unik se-dokumen |
| `type` | string | `text`, `rect`, `line`, `image` |
| `x`, `y` | number | sudut kiri atas kotak, relatif terhadap sudut kiri atas halaman |
| `w`, `h` | number | lebar dan tinggi; tidak boleh negatif kecuali pada `line` |

Nilai di luar ±100000 ditolak.

### 1.4 `text`

| Properti | Tipe | Nilai yang sah |
|---|---|---|
| `text` | string | boleh kosong; `\n` menjadi pergantian baris |
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

### 1.5 `rect`

| Properti | Tipe | Keterangan |
|---|---|---|
| `fill` | string | warna isi; kosong berarti tanpa isi |
| `stroke` | string | warna garis tepi; kosong berarti tanpa garis |
| `strokeWidth` | number | ≥ 0; garis tepi hanya digambar bila > 0 |
| `radius` | number | ≥ 0; dibatasi separuh sisi terpendek |

Kotak tanpa `fill` maupun `stroke` tidak digambar sama sekali.

### 1.6 `line`

`w` dan `h` di sini **bukan ukuran** melainkan simpangan ujung terhadap pangkal,
sehingga keduanya boleh negatif. Garis mendatar berarti `"h": 0`.

| Properti | Tipe | Keterangan |
|---|---|---|
| `stroke` | string | wajib diisi agar garis terlihat |
| `strokeWidth` | number | > 0 agar garis terlihat |

Ketebalan terbagi rata di kedua sisi jalurnya, sama seperti `stroke` pada SVG —
dan **tidak** seperti `border` pada CSS. Cara menggambarnya ada di
[2.4](#24-kotak-dan-garis-pakai-svg-bukan-div).

### 1.7 `image`

| Properti | Tipe | Keterangan |
|---|---|---|
| `assetToken` | string | wajib; token aset yang sudah terunggah |
| `fit` | string | `contain`, `cover`, `fill` — sama artinya dengan `object-fit` |

Gambar menunjuk **aset, bukan URL**. Backend tidak pernah mengambil alamat yang
ditentukan lewat isi dokumen. Untuk menampilkannya, frontend memakai
`GET /api/asset-presign/:token` seperti biasa.

Aset yang sudah terhapus dilewati saat ekspor — sama seperti frontend yang juga
tidak dapat menampilkannya, sehingga layar dan cetak tetap sama.

Format yang dapat disematkan ke PDF: **JPEG, PNG, GIF**. Format lain dilewati.

### 1.8 Nilai bawaan

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
setiap properti yang menentukan tata letak pada setiap elemen teks. Panduan bawaan
yang disusun backend melakukan persis itu, dan dapat dipakai sebagai rujukan
pertama saat menyelaraskan tampilan.

### 1.9 Isi yang ditolak

Isi yang melanggar aturan di atas membuat koneksi ditutup `1011` dengan alasan
yang menyebutkan persis apa yang salah:

```
json: unknown field "textShadow"
duplicate element id "dup"
element "el_7f3a" has color "red", expected #rgb or #rrggbb
element "el_7f3a" has align "middle", expected one of left, center, right, justify
```

Ekspor menolaknya dengan `400` dan pesan yang sama.

---

## 2. Menggambar di kanvas

Bagian ini menjawab pertanyaan yang paling sering membuat frontend tertahan:
bagaimana mengubah angka di atas menjadi sesuatu di layar yang **sama persis**
dengan hasil cetaknya.

### 2.1 Ukuran halaman, zoom, dan koordinat

Ukuran halaman datang bersama `snapshot`, sudah dalam titik:

```jsonc
{ "type": "snapshot", "version": 3, "page": { "width": 595.276, "height": 841.89 }, "content": { … } }
```

Ambil dari sana, **jangan** menghitung sendiri dari `GET /api/document-detail`.
Endpoint itu mengembalikan ukuran dalam satuan asli kertas — `210 / 297 / "mm"`
untuk A4, `9.5 / 11 / "in"` untuk Continuous Form — dan mengonversinya sendiri
hanya menambah satu tempat yang bisa meleset.

**Titik ke piksel layar.** CSS memakai 96 piksel per inci sedangkan titik adalah
1/72 inci, jadi satu titik selalu bernilai 4/3 piksel:

```js
const PX_PER_PT = 96 / 72;   // 1.333…
```

Artinya halaman A4 pada zoom 100% berukuran 793.7 × 1122.5 piksel CSS. "Zoom
100%" **bukan** `scale(1)` dalam arti satu titik satu piksel — ia sudah 1.333×
sejak awal, dan itu memang benar: begitulah CSS mengartikan satuan mutlak.

**Zoom** cukup dengan `transform` pada pembungkus halaman. Angka di dalam data
tidak pernah diubah:

```jsx
<div style={{
  width:  `${page.width * PX_PER_PT * zoom}px`,   // transform tidak memesan ruang,
  height: `${page.height * PX_PER_PT * zoom}px`,  // jadi ukurannya disebut di sini
}}>
  <div style={{
    width:  `${page.width}pt`,
    height: `${page.height}pt`,
    position: 'relative',
    overflow: 'hidden',          // elemen di luar halaman ikut terpotong, sama seperti cetak
    transform: `scale(${zoom})`,
    transformOrigin: 'top left',
  }}>
    {elements.map(renderElement)}
  </div>
</div>
```

Pembungkus luar wajib menyebut ukurannya sendiri: `transform` tidak memesan ruang
di tata letak, sehingga tanpa itu halaman akan saling tumpang tindih saat
digulir.

**Koordinat pointer kembali ke titik** — inilah yang dibutuhkan seret-dan-lepas:

```js
function toPoints(event, pageElement, zoom) {
  const box = pageElement.getBoundingClientRect();
  const pxPerPt = PX_PER_PT * zoom;

  return {
    x: (event.clientX - box.left) / pxPerPt,
    y: (event.clientY - box.top) / pxPerPt,
  };
}
```

`getBoundingClientRect()` sudah memperhitungkan `scale`, jadi cukup dibagi sekali.

**Pembulatan.** Backend membulatkan koordinat benih ke tiga desimal. Frontend
sebaiknya melakukan hal yang sama saat menyimpan hasil seretan, supaya nilai
seperti `23.760000000000002` tidak menumpuk di dalam data.

### 2.2 Font: berkas yang sama di kedua sisi

Ini kewajiban yang paling mudah terlewat dan paling mahal akibatnya. Helvetica di
macOS dan Arial di Windows punya lebar glif yang berbeda, dan selisih sekecil apa
pun menumpuk menjadi pemenggalan baris yang berbeda antara layar dan cetak.

**Daftar font diambil dari backend**, bukan dipatok di frontend:

```http
GET /api/document-design-fonts
Authorization: Bearer <access_token>
```

```jsonc
{
  "status": "ok",
  "message": "Success",
  "data": {
    "fonts": [
      {
        "name": "helvetica",
        "faces": [
          { "weight": 400, "style": "italic" },
          { "weight": 400, "style": "normal" },
          { "weight": 700, "style": "italic" },
          { "weight": 700, "style": "normal" }
        ]
      }
    ]
  }
}
```

Isinya persis yang benar-benar terdaftar di backend. Pilihan font di editor harus
dibangun dari daftar ini — daftar yang dipatok akan melenceng begitu ada yang
menambah berkas font, dan melencengnya baru ketahuan ketika ekspor gagal `400`.

**Berkas yang sama disajikan lewat `@font-face`**, bukan Google Fonts, bukan font
sistem:

```css
@font-face {
  font-family: 'inter';
  src: url('/fonts/Inter-Regular.ttf') format('truetype');
  font-weight: 400;
  font-style: normal;
  font-display: block;   /* bukan swap — lihat di bawah */
}
```

`font-display: block` disengaja. Dengan `swap`, browser menggambar dengan font
cadangan lebih dulu; bila pengguna menyeret elemen atau kanvas diukur pada jendela
waktu itu, hasilnya memakai metrik yang salah. Dan sebelum mengukur apa pun,
tunggu:

```js
await document.fonts.ready;
```

**Nama keluarga tidak peka huruf besar-kecil** di backend — `Inter` dan `inter`
sama-sama diterima. Tetapi nilai `fontFamily` di data harus cocok dengan
`font-family` di `@font-face`, jadi paling aman menyimpannya dalam huruf kecil
seperti yang disebut endpoint daftar font.

Backend memuat berkasnya dari `DESIGN_FONT_DIR` menurut manifes `fonts.json`:

```json
{
  "families": [
    {
      "name": "inter",
      "faces": [
        { "weight": 400, "style": "normal", "file": "Inter-Regular.ttf" },
        { "weight": 700, "style": "normal", "file": "Inter-Bold.ttf" },
        { "weight": 400, "style": "italic", "file": "Inter-Italic.ttf" }
      ]
    }
  ]
}
```

Ketebalan yang tidak terdaftar **menggagalkan ekspor**, tidak dibulatkan ke yang
terdekat. Penebalan buatan oleh browser sementara backend memakai penggantinya
adalah cara paling halus untuk menghasilkan dua tampilan yang berbeda; lebih baik
gagal dengan keterangan jelas.

> **Keluarga `helvetica` selalu tersedia tanpa berkas apa pun**, karena metriknya
> melekat pada spesifikasi PDF. Tetapi ia punya dua batas: browser akan memakai
> Helvetica atau Arial milik sistem yang metriknya berbeda antar sistem operasi,
> dan pengkodeannya Windows-1252 sehingga aksara di luar Eropa Barat — Tionghoa,
> Arab, Jepang — menjadi tanda tanya. Ia ada supaya ekspor dapat dicoba sebelum
> font sungguhan disiapkan, bukan untuk dokumen yang sungguh dicetak.

### 2.3 Elemen teks

```jsx
function TextElement({ el }) {
  return (
    <div
      style={{
        position: 'absolute',
        left: `${el.x}pt`, top: `${el.y}pt`,
        width: `${el.w}pt`, height: `${el.h}pt`,

        fontFamily: el.fontFamily ?? 'helvetica',
        fontSize: `${el.fontSize ?? 12}pt`,
        fontWeight: el.fontWeight ?? 400,
        fontStyle: el.fontStyle ?? 'normal',
        color: el.color ?? '#000000',
        textAlign: el.align ?? 'left',
        lineHeight: el.lineHeight ?? 1.2,
        letterSpacing: `${el.letterSpacing ?? 0}pt`,

        // Wajib. Tanpa lima baris ini layar dan cetak tidak akan pernah sama.
        fontKerning: 'none',
        fontFeatureSettings: '"liga" 0',
        whiteSpace: 'pre-line',
        overflow: 'hidden',
        overflowWrap: 'normal',

        // Wajib nol. Lihat penjelasan di bawah.
        margin: 0, padding: 0, border: 0,
      }}
    >
      {el.text}
    </div>
  );
}
```

Empat jebakan pada elemen teks, semuanya menghasilkan perbedaan yang tidak
menimbulkan error apa pun:

**Kerning dan ligatur harus dimatikan.** Browser secara bawaan merapatkan pasangan
huruf tertentu dan menggabungkan `fi` menjadi satu glif. Backend menjumlahkan
lebar glif apa adanya.

**Padding, border, dan margin harus nol.** Rumus garis dasar di
[2.6](#26-aturan-tata-letak-yang-dipakai-backend) mengandaikan teks mulai persis
di tepi atas kotak. Padding sekecil apa pun menggesernya ke bawah, dan pergeseran
itu tidak pernah muncul di hasil cetak.

**Jangan meratakan dengan flex atau grid.** `display: flex` dengan
`alignItems: 'center'` membuang aturan half-leading sama sekali. Teks harus
mengalir biasa dari tepi atas.

**Tinggi tidak boleh mengikuti isinya.** `height: auto` — atau kotak yang tumbuh
otomatis saat pengguna mengetik — akan menampilkan seluruh teks di layar
sementara backend **memotongnya** pada `h`. Kalau editor perlu menumbuhkan kotak,
tumbuhkan `h` di dalam data, bukan hanya tampilannya.

### 2.4 Kotak dan garis: pakai SVG, bukan `div`

PDF menggambar garis **terpusat pada jalurnya**: `strokeWidth` 1 pt berarti 0.5 pt
di dalam dan 0.5 pt di luar batas kotak. CSS `border` tidak pernah begitu — ia
seluruhnya di dalam atau seluruhnya di luar, tergantung `box-sizing`. Kotak dengan
garis tepi akan selalu meleset setengah ketebalan, di keempat sisinya.

SVG memakai aturan yang sama persis dengan PDF, jadi memakainya menghapus seluruh
kelas ketidakcocokan ini sekaligus:

```jsx
function ShapeElement({ el, page }) {
  return (
    <svg
      viewBox={`0 0 ${page.width} ${page.height}`}
      style={{
        position: 'absolute', left: 0, top: 0,
        width: '100%', height: '100%',
        overflow: 'visible', pointerEvents: 'none',
      }}
    >
      {el.type === 'rect' ? (
        <rect
          x={el.x} y={el.y} width={el.w} height={el.h} rx={el.radius || 0}
          fill={el.fill || 'none'}
          stroke={el.stroke || 'none'}
          strokeWidth={el.strokeWidth || 0}
        />
      ) : (
        <line
          x1={el.x} y1={el.y} x2={el.x + el.w} y2={el.y + el.h}
          stroke={el.stroke || 'none'}
          strokeWidth={el.strokeWidth || 0}
        />
      )}
    </svg>
  );
}
```

`viewBox` seukuran halaman membuat satuan SVG menjadi titik, sehingga koordinat
dipakai apa adanya. Satu `<svg>` **per elemen**, bukan satu untuk semua, supaya
urutan tumpuknya tetap mengikuti urutan elemen — kotak yang datang setelah teks
harus menutupi teks itu.

`rx` pada SVG dibatasi separuh sisi terpendek secara otomatis, sama seperti
`radius` di backend. `fill` atau `stroke` yang kosong dipetakan ke `'none'`,
bukan dibiarkan kosong.

### 2.5 Gambar

```jsx
<img
  src={presignedUrl}
  style={{
    position: 'absolute',
    left: `${el.x}pt`, top: `${el.y}pt`,
    width: `${el.w}pt`, height: `${el.h}pt`,
    objectFit: el.fit ?? 'contain',
    objectPosition: 'center',   // backend selalu menengahkan
  }}
/>
```

`presignedUrl` diambil dari `GET /api/asset-presign/{assetToken}` seperti aset
lain di aplikasi ini. Isi dokumen menyimpan token, bukan URL — backend tidak
pernah mengambil alamat yang ditentukan lewat isi dokumen.

`object-fit` di CSS dan `fit` di backend punya arti yang sama untuk ketiga
nilainya, dan `object-position: center` cocok dengan backend yang selalu
menengahkan. Aset yang sudah terhapus tidak tergambar di kedua sisi, jadi layar
dan cetak tetap sama.

### 2.6 Aturan tata letak yang dipakai backend

Bagian rujukan. Diperlukan saat ada yang tidak cocok dan Anda perlu tahu apa yang
sebenarnya dilakukan backend.

**Pemenggalan baris** rakus: kata ditambahkan selama masih muat, dan pindah ke
baris berikutnya begitu tidak. Tanpa tanda hubung otomatis. Kata yang lebih lebar
daripada kotaknya dibiarkan meluber, tidak dipatahkan — itulah `overflow-wrap:
normal`.

**Spasi** dipadatkan: deretan spasi dan tab menjadi satu pemisah, sedangkan `\n`
dihormati sebagai pergantian baris. Itu persis `white-space: pre-line`. Baris
kosong tetap memakan satu tinggi baris.

**Lebar teks** adalah jumlah lebar glif, ditambah `letterSpacing` setelah
**setiap** huruf termasuk yang terakhir — sama seperti CSS.

**Garis dasar baris pertama** memakai aturan half-leading milik CSS:

```
tinggiBaris = fontSize × lineHeight
sisa        = tinggiBaris − (ascent + descent)
garisDasar  = y + sisa / 2 + ascent
```

`ascent` dan `descent` diambil dari FontDescriptor berkas font. Browser tidak
selalu memakai sumber yang sama — sebagian mengambil metrik `hhea`, sebagian
`OS/2`. **Bila teks tampak bergeser vertikal secara konsisten pada satu keluarga
font, di sinilah tempat memeriksanya.**

**Rata penuh** hanya meregangkan jarak antar kata, dan **tidak** diterapkan pada
baris terakhir sebuah paragraf — sama seperti `text-align: justify` bawaan.

### 2.7 Halaman jamak

Seluruh halaman satu dokumen berukuran sama, jadi `page` dari snapshot berlaku
untuk semuanya. Tampilkan bertumpuk ke bawah; jarak antar halaman sepenuhnya
urusan tampilan dan tidak ada di dalam data.

Setiap halaman perlu `overflow: hidden`. Elemen yang koordinatnya di luar batas
kertas **sah** menurut validasi backend — hanya nilai di luar ±100000 yang
ditolak — tetapi tidak akan tergambar di PDF karena berada di luar area cetak.
Tanpa kliping, layar akan menampilkan sesuatu yang tidak pernah ikut tercetak.

---

## 3. Membuka sesi realtime

### 3.1 Terbitkan tiket

`WebSocket` di browser tidak dapat mengirim header `Authorization`, sehingga access
token tidak bisa ikut saat handshake. Tiket ini penggantinya.

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

### 3.2 Buka koneksi

```
ws://localhost:8081/document-design/{documentToken}?ticket={ticket}
```

Header `Origin` diperiksa terhadap `FRONTEND_URL`. Bila tidak cocok, handshake
ditolak **HTTP 403 sebelum upgrade** — dan browser tidak dapat membaca status itu.

Saat `APP_ENV=local`, seluruh host loopback ikut diizinkan tanpa perlu menyetel
`FRONTEND_URL` setiap kali port atau subdomain berganti:

```
localhost      localhost:<port>
*.localhost    *.localhost:<port>     ← portal.localhost:3000, dan seterusnya
127.0.0.1      127.0.0.1:<port>
```

Pelonggaran ini **hanya** berlaku pada `APP_ENV=local`; staging dan produksi tetap
terkurung pada `FRONTEND_URL`. Ia juga tetap terbatas pada loopback: `.localhost`
adalah TLD yang dicadangkan RFC 6761 dan tidak dapat didaftarkan publik, sehingga
`localhost.evil.com` tetap ditolak.

| Batas koneksi | |
|---|---|
| Ukuran satu pesan | **1 MB** |
| Koneksi bersamaan per user | **10** |
| Ping dari server | tiap 30 detik, tenggang pong 10 detik |
| Antrean keluar | 64 pesan; klien yang tertinggal lebih jauh **diputus** |

Ping dan pong ditangani browser otomatis — tidak ada yang perlu ditulis. Koneksi
yang tidak responsif diputus paling lambat 40 detik setelah benar-benar mati.

### 3.3 Minta isi dokumen

**Server diam sampai diminta.** Membuka koneksi tidak mengirim apa pun.

```jsonc
{ "type": "document.get" }
```

Boleh dipanggil **berulang** pada koneksi yang sama. Itulah jalur pemulihan:
frontend yang menyadari keadaannya tertinggal cukup meminta ulang tanpa memutus
koneksi.

Klien baru menjadi anggota room — dan karenanya penerima siaran nanti — pada saat
permintaan ini diproses, bukan saat socket terbuka.

Semua pesan wajib berupa **JSON dalam text frame**.

### 3.4 Terima snapshot

```jsonc
{
  "type": "snapshot",
  "version": 3,
  "page": { "width": 595.276, "height": 841.89 },
  "content": {
    "pages": [
      {
        "id": "18f10d20-42f7-4a4c-80df-63dc46ab0022",
        "elements": [
          {
            "id": "4f1a421c-5f0c-49b4-af21-07c13902d2c1",
            "type": "text",
            "x": 59.528,
            "y": 67.351,
            "w": 476.22,
            "h": 50.513,
            "text": "Kaidah Dokumen yang Baik",
            "fontFamily": "helvetica",
            "fontSize": 24,
            "fontWeight": 700,
            "fontStyle": "normal",
            "color": "#111827",
            "align": "left",
            "lineHeight": 1.3
          }
        ]
      }
    ]
  }
}
```

`page` adalah ukuran satu halaman dalam titik, berlaku untuk seluruh halaman
dokumen ini. Ia satu-satunya sumber ukuran kanvas yang perlu dipakai — lihat
[2.1](#21-ukuran-halaman-zoom-dan-koordinat).

### 3.5 Siapa yang sedang membuka

Setiap kali daftar orang berubah, server mengirim:

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
untuk menampilkan "+6" pada tumpukan avatar. Warna avatar juga tidak dikirim;
turunkan sendiri dari `id` supaya orang yang sama berwarna sama di semua layar.

**Yang didaftar orang, bukan koneksi.** Satu orang dengan tiga tab tetap satu
entri, dan ia baru hilang ketika tab terakhirnya tertutup. Ini bukan kehalusan
teoretis: frontend membuka lebih dari satu koneksi tiap kali halaman dimuat, jadi
menghitung koneksi akan menampilkan satu orang sebagai beberapa penyunting.

Urutannya menurut nama, jadi susunan avatar tidak berubah-ubah sendiri.

**Kapan pesan ini datang:**

| Kejadian | Siapa yang menerima |
|---|---|
| Seseorang bergabung, dan ia belum hadir sebelumnya | **semua**, termasuk yang baru bergabung |
| Seseorang membuka tab kedua | hanya tab baru itu — bagi yang lain daftarnya tidak berubah |
| `document.get` diulang sebagai pemulihan | hanya yang meminta |
| Seseorang menutup satu dari beberapa tabnya | **tidak ada** — ia masih hadir |
| Seseorang menutup tab terakhirnya | semua yang tersisa |

Pada klien yang baru bergabung, `snapshot` **selalu** datang lebih dulu, baru
`presence`. Keduanya dimasukkan ke antrean pada langkah yang sama oleh server,
jadi urutan itu terjamin.

Seseorang baru terhitung hadir setelah ia mengirim `document.get` dan menerima
isinya — koneksi yang terbuka tetapi belum meminta dokumen belum masuk daftar.
Itu memang benar: ia belum melihat apa pun.

Nama diambil dari tiket, yang diterbitkan lewat endpoint terautentikasi. Tidak
ada yang perlu dikirim frontend untuk ini.

### 3.6 Versi

`version` adalah nomor revisi **dokumen** — satu dokumen, satu penghitung, naik
setiap kali ada perubahan yang berhasil diterapkan.

Rancangannya: **permintaan digerakkan frontend, penyelarasan digerakkan backend.**
Frontend menyimpan versi terakhir yang diterimanya, dan setiap kali backend
mengirim versi yang berbeda, frontend mengganti keadaannya dengan yang baru. Tidak
ada nomor urut yang perlu dikirim frontend.

Hari ini `version` hanya bergerak ketika dokumen kosong diisi panduan bawaan, dan
tidak ada siaran perubahan sama sekali. **Simpan nilainya, jangan bangun logika
rekonsiliasi apa pun di atasnya sekarang** — aturannya akan ditulis di sini
bersamaan dengan pesan penyuntingan, bukan sebelumnya.

### 3.7 Panduan bawaan untuk dokumen kosong

Dokumen yang `pages`-nya masih kosong **diisi otomatis** satu halaman berisi
delapan blok teks tentang kaidah menyusun dokumen. Benih ini disimpan, sehingga id
elemennya tetap sama pada pembukaan berikutnya.

Dokumen yang sudah punya halaman tidak pernah disentuh — sekalipun halaman itu
belum berisi elemen.

Tata letaknya proporsional terhadap kertas: margin sisi 10% lebar, margin atas 8%
tinggi, dan seperlima halaman bawah sengaja dibiarkan kosong. Ukuran hurufnya
mutlak, bukan proporsional — 24, 13, 11, dan 10.5 pt — karena yang menentukan
keterbacaan adalah jarak baca mata, bukan lebar kertas.

Benih ini hanya disusun saat room memuat dokumen, yaitu ketika ada yang menyambung
lewat WebSocket. Mengekspor dokumen kosong yang belum pernah dibuka menghasilkan
halaman kosong, dan itu memang isinya.

---

## 4. Menangani kegagalan

Ada **dua jalur yang terpisah sama sekali**, dan menyamakannya adalah kekeliruan
yang paling mahal:

- **Close code** — koneksi berakhir.
- **Pesan `error`** — koneksi tetap hidup; satu permintaan ditolak.

Menyambung ulang bukan tindakan yang tepat untuk jalur kedua.

### 4.1 Close code

| Code | Reason | Kapan | Tindakan frontend |
|---|---|---|---|
| `1008` | `invalid or expired design ticket` | saat handshake | Terbitkan tiket baru, sambung ulang |
| `1013` | `too many concurrent design connections` | saat handshake | Sudah 10 koneksi untuk user ini; tunggu, lalu coba lagi |
| `1011` | alasan spesifik dari server | **di tengah sesi** | Berhenti; tampilkan pesannya, jangan sambung ulang otomatis |
| `1003` | `only JSON text frames are supported` | kapan saja | Bug frontend: ada frame biner terkirim |
| `1000` | — | — | Penutupan normal |

`1011` **tidak** terjadi saat membuka koneksi. Ia muncul ketika room berhenti dapat
melayani selagi frontend sudah menjadi anggota — misalnya isinya tidak lagi dapat
disimpan karena dokumennya dihapus orang lain. Server memutus koneksi beserta
alasannya, karena membiarkan orang menyunting sesuatu yang tidak akan pernah
tersimpan lebih buruk daripada menghentikannya.

Room yang bermasalah **sebelum** frontend menjadi anggota tidak menutup koneksi; ia
dibalas pesan `error` berkode `document_unavailable`.

### 4.2 Pesan error

```jsonc
{ "type": "error", "code": "unsupported_message_type", "message": "…" }
```

| Code | Arti | Tindakan |
|---|---|---|
| `document_unavailable` | Room tidak dapat melayani `document.get`; pesannya menyebut sebabnya | Tunggu belasan detik, lalu kirim `document.get` lagi. Room bermasalah dibuang setelah masa tenggangnya dan dimuat ulang, sehingga percobaan berikutnya sering berhasil. **Jangan** sambung ulang koneksinya |
| `malformed_message` | JSON tidak dapat diurai | Buang antrean optimistik, kirim `document.get` untuk memulai bersih |
| `missing_message_type` | Field `type` kosong atau tidak ada | Bug frontend |
| `unsupported_message_type` | Jenis pesan belum didukung backend | Bug frontend, atau fitur yang memang belum ada |

Bila `document_unavailable` diabaikan, antarmuka akan menggantung dengan kanvas
kosong tanpa tanda apa pun — koneksinya hidup, hanya isinya yang tidak pernah
datang.

### 4.3 `1006` punya dua sebab yang berlawanan

Handshake yang ditolak — `Origin` tidak diizinkan, atau backend tidak terjangkau —
gagal **sebelum** upgrade, sehingga tidak ada close frame sama sekali. Browser
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

---

## 5. Ekspor PDF

```http
POST /api/document-export/{documentToken}
Authorization: Bearer <access_token>
```

Balasannya **bukan** amplop JSON seperti endpoint lain, melainkan berkas PDF
mentah:

```
200 OK
Content-Type: application/pdf
Content-Disposition: attachment; filename="Surat Jalan.pdf"; filename*=UTF-8''Surat%20Jalan.pdf
Cache-Control: no-store
```

Kegagalan tetap memakai amplop JSON yang sama dengan endpoint lain, karena saat itu
tidak ada berkas yang dikirim.

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

Penyimpanan ke database tertunda sampai dua detik. Karena itu ekspor mengambil isi
dari room bila dokumennya sedang dibuka seseorang, dan dari database bila tidak.

Artinya **tidak perlu menunggu atau memaksa simpan sebelum mengekspor**. Menggeser
sebuah elemen lalu langsung menekan ekspor menghasilkan PDF yang sudah memuat
geseran itu.

### Kegagalan yang mungkin

| Status | Sebab |
|---|---|
| `400` | isi dokumen tidak sah, kertas memakai satuan yang tidak dikenal, atau font yang diminta tidak terdaftar |
| `404` | dokumen tidak ada atau sudah dihapus |
| `503` | dokumen sedang dibuka tetapi room-nya bermasalah |

Pesan pada `400` menyebut persis apa yang kurang — misalnya
`font inter 700 normal is not available`. Itu jenis kegagalan yang diperbaiki
dengan mendaftarkan berkas fontnya, bukan dengan mencoba lagi.

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

    if (message.type === 'snapshot') {
      // page berisi ukuran halaman dalam titik; dibutuhkan untuk menyiapkan kanvas.
      renderCanvas(message.content, message.page);
      localVersion = message.version;
      return;
    }

    if (message.type === 'presence') {
      // Daftar orang, bukan koneksi. Selalu datang setelah snapshot pertama.
      renderAvatars(message.users);
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

Setiap jalur penyambungan ulang — termasuk backoff — **wajib menerbitkan tiket
baru**. Tiket lama sudah hangus atau kedaluwarsa, dan memakainya kembali hanya
menghasilkan `1008` yang menyesatkan.

Cara menggambar tiap jenis elemen ada di [bagian 2](#2-menggambar-di-kanvas).

---

## Daftar periksa penyesuaian frontend

Bagian ini menjawab pertanyaan yang paling praktis: **apa yang harus diubah dari
editor yang sudah ada?**

### Yang berubah

| Kemungkinan sekarang | Menjadi | Perinciannya |
|---|---|---|
| Koordinat dalam piksel, atau dalam satuan kertas | Titik (pt) di dalam data; piksel hanya saat menggambar | [1.1](#11-satuan-titik-pt), [2.1](#21-ukuran-halaman-zoom-dan-koordinat) |
| Properti visual bersarang di `props` | Datar, langsung di elemen | [1.3](#13-properti-bersama) |
| `fontWeight: "bold"` | `fontWeight: 700` | [1.4](#14-text) |
| `color: "red"`, `rgba(…)`, `hsl(…)` | `#rrggbb` atau `#rgb` | [1.4](#14-text) |
| Ukuran halaman ditebak atau dihitung sendiri | Dari `page` pada snapshot | [2.1](#21-ukuran-halaman-zoom-dan-koordinat) |
| Daftar font dipatok di frontend | Dari `GET /api/document-design-fonts` | [2.2](#22-font-berkas-yang-sama-di-kedua-sisi) |
| Font dari Google Fonts atau font sistem | Berkas yang sama dengan backend, lewat `@font-face` | [2.2](#22-font-berkas-yang-sama-di-kedua-sisi) |
| Kotak teks tumbuh mengikuti isinya | Tinggi tetap dari `h`; isi yang melebihi terpotong | [2.3](#23-elemen-teks) |
| Kotak digambar dengan `div` + `border` | `<rect>` di dalam SVG | [2.4](#24-kotak-dan-garis-pakai-svg-bukan-div) |
| Garis digambar dengan `div` tipis | `<line>` di dalam SVG | [2.4](#24-kotak-dan-garis-pakai-svg-bukan-div) |
| Gambar memakai URL langsung | `assetToken` + `GET /api/asset-presign/:token` | [2.5](#25-gambar) |

### Yang perlu ditambahkan

Empat hal ini kemungkinan besar belum ada sama sekali, dan tanpanya layar akan
berbeda dari hasil cetak **tanpa satu pun pesan kesalahan**:

- `font-kerning: none` dan `font-feature-settings: "liga" 0` pada setiap elemen teks
- `white-space: pre-line`, `overflow: hidden`, `overflow-wrap: normal` pada setiap elemen teks
- `margin`, `padding`, dan `border` bernilai nol pada setiap elemen teks, dan tanpa perataan flex atau grid
- `overflow: hidden` pada setiap halaman, supaya elemen di luar kertas ikut terpotong seperti saat dicetak

### Urutan yang disarankan

1. **Ganti satuan lebih dulu.** Seluruh koordinat dan ukuran huruf menjadi titik,
   dan ukuran kanvas diambil dari `page` pada snapshot. Selama ini belum benar,
   perbandingan apa pun dengan hasil cetak tidak ada artinya.
2. **Ratakan `props`.** Properti visual naik satu tingkat ke elemen, dan nama
   yang berbeda disesuaikan menurut [1.4](#14-text) sampai [1.7](#17-image).
   Properti yang tidak ada di daftar itu **ditolak backend**, bukan diabaikan.
3. **Pasang font.** Taruh berkasnya di `DESIGN_FONT_DIR` beserta `fonts.json`,
   sajikan berkas yang sama lewat `@font-face`, dan isi pilihan font dari
   endpoint daftar font.
4. **Terapkan aturan CSS wajib** pada elemen teks, lalu ganti kotak dan garis ke
   SVG.
5. **Bandingkan.** Buka satu dokumen di editor, ekspor PDF-nya, tumpuk keduanya.
   Mulai dari dokumen yang teksnya panjang dan membungkus — di situlah selisih
   metrik paling terlihat.

### Cara memeriksa bahwa penyesuaiannya sudah benar

Ambil satu dokumen kosong dan biarkan backend mengisinya dengan panduan bawaan.
Benih itu menyebut **setiap** properti tata letak secara tegas, tanpa mengandalkan
satu pun nilai bawaan, sehingga ia rujukan yang paling bersih untuk dibandingkan.

Yang perlu diperiksa, berurutan dari yang paling sering meleset:

| Gejala | Tersangka pertama |
|---|---|
| Baris pecah di kata yang berbeda | Berkas font tidak sama, atau kerning/ligatur belum dimatikan |
| Seluruh teks bergeser turun sedikit | Ada `padding` atau `border` pada kotak teks |
| Teks bergeser vertikal hanya pada satu keluarga font | Perbedaan metrik `hhea` versus `OS/2` — lihat [2.6](#26-aturan-tata-letak-yang-dipakai-backend) |
| Teks yang panjang terlihat penuh di layar tapi terpotong di PDF | Kotak teks tumbuh otomatis; `h` di data tidak ikut berubah |
| Garis tepi kotak meleset tipis | Masih memakai `border`, bukan SVG |
| Seluruh tata letak berskala salah | Satuan belum titik, atau `PX_PER_PT` belum dipakai |

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

---

## Lampiran: menyetel ulang dokumen berbentuk lama

Sebelum model ini ditutup, isi elemen disimpan di dalam objek `props` dan
koordinatnya memakai satuan kertas. Dokumen berbentuk itu **tidak akan terbaca**;
room menutup koneksi dengan `1011`, dan ekspor menolaknya dengan `400`.

Kosongkan isinya agar panduan bawaan disusun ulang dalam bentuk baru:

```sql
UPDATE documents
SET content = '{"pages": []}',
    content_version = content_version + 1
WHERE content::text LIKE '%"props"%';
```

`content_version` dinaikkan, bukan disetel ke angka tetap, supaya nilainya tidak
pernah mundur dari yang pernah dipakai room. Jalankan saat tidak ada yang sedang
membuka dokumennya: room menyimpan dengan compare-and-set pada kolom itu, sehingga
penulisan dari luar akan membuat penyimpanannya bentrok dan koneksi orang tersebut
diputus.
