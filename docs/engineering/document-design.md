# Document Design — Panduan Frontend

Cara membangun editor dokumen: bentuk isi dokumen, cara menggambarnya supaya
sama persis dengan hasil cetak, dan cara mengekspornya.

> **Dokumen ini menjelaskan isinya. Amplopnya dijelaskan di
> [`websocket-contract.md`](websocket-contract.md).**
>
> Semua yang melintas di socket — setiap pesan, apa yang memicunya, close code,
> dan penyambungan ulang — ada di sana. Termasuk **riwayat perubahan kontrak**,
> yang menjawab apakah Anda perlu menyesuaikan sesuatu sejak terakhir membaca.

> **Baru pertama kali menyesuaikan editor yang sudah ada?** Lompat ke
> [daftar periksa penyesuaian](#daftar-periksa-penyesuaian-frontend), lalu
> kembali ke sini untuk perinciannya.

## Peta singkat

| | |
|---|---|
| [1. Model isi dokumen](#1-model-isi-dokumen) | Bentuk halaman dan elemen, satuan, nilai bawaan, dan apa yang ditolak |
| [2. Menggambar di kanvas](#2-menggambar-di-kanvas) | Zoom dan koordinat, font, teks, kotak dan garis, gambar |
| [3. Panduan bawaan](#3-panduan-bawaan-untuk-dokumen-kosong) | Isi yang disusun backend untuk dokumen yang masih kosong |
| [4. Ekspor PDF](#4-ekspor-pdf) | `POST /api/document-export/:token` |
| [Daftar periksa](#daftar-periksa-penyesuaian-frontend) | Apa yang harus diubah dari editor yang sudah ada |

Isi dokumen **belum dapat diubah lewat API mana pun** — untuk sekarang
perubahannya dilakukan langsung ke kolom `documents.content` di database. Bentuk
pesan penyuntingan belum diputuskan; lihat
[kontrak WebSocket](websocket-contract.md#22-yang-belum-ada).

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
field `page` pada pesan `snapshot`, sudah dalam titik — lihat
[kontrak WebSocket](websocket-contract.md#31-snapshot).

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

Isi yang melanggar aturan di atas membuat koneksi ditutup
[`1011`](websocket-contract.md#51-close-code) dengan alasan
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

## 3. Panduan bawaan untuk dokumen kosong

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

## 4. Ekspor PDF

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

## Lampiran: menyetel ulang dokumen berbentuk lama

Sebelum model ini ditutup, isi elemen disimpan di dalam objek `props` dan
koordinatnya memakai satuan kertas. Dokumen berbentuk itu **tidak akan terbaca**;
room menutup koneksi dengan [`1011`](websocket-contract.md#51-close-code), dan
ekspor menolaknya dengan `400`.

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
