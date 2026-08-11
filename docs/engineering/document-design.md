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

Isi dokumen diubah lewat pesan `element.*` di WebSocket — bentuk dan aturannya
ada di [kontrak WebSocket](websocket-contract.md#23-elementcreate). Dokumen ini
menjelaskan **isinya**: bentuk elemen yang Anda kirim di dalam pesan-pesan itu,
dan cara menggambarnya supaya layar dan hasil cetak sama.

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
| `pages` | array; **minimal satu halaman** — halaman terakhir tidak dapat dihapus |
| Setiap halaman | `id` tidak kosong, `elements` opsional; `title`, `background`, `hidden`, dan `locked` opsional |
| Setiap elemen | `locked`, `groupId`, `rotation`, dan `opacity` opsional, berlaku untuk semua jenis |
| `rect`, `ellipse`, dan `line` | `strokeStyle` opsional: `solid`, `longdash`, `dash`, `dot` |
| Setiap elemen | `id` tidak kosong dan `type` yang dikenal |
| Seluruh `id` | unik dalam satu dokumen, **termasuk lintas halaman** |

Id unik lintas halaman supaya elemen dapat berpindah halaman tanpa risiko bentrok.

**Ukuran halaman tidak disimpan di dalam isi dokumen.** Ia diambil dari kertas
dokumen, sehingga seluruh halaman dijamin seukuran dan data tidak dapat
bertentangan dengan kertas yang dipilih pengguna. Frontend menerimanya lewat
field `page` pada pesan `snapshot`, sudah dalam titik — lihat
[kontrak WebSocket](websocket-contract.md#31-snapshot).

Urutan elemen adalah urutan gambar: yang belakangan menutupi yang terdahulu, sama
seperti urutan DOM. Urutan **halaman** adalah urutan cetak, dan keduanya diubah
lewat pesan `element.reorder` dan `page.reorder`.

**`hidden` pada halaman tidak ikut tercetak.** Ekspor melewatinya seluruhnya —
termasuk tidak mengunduh aset di atasnya. Dokumen yang seluruh halamannya
tersembunyi menghasilkan PDF berisi satu halaman kosong, sama seperti dokumen
yang memang belum punya halaman.

### Gaya garis

`strokeStyle` berlaku pada `rect` dan `line`, dan hanya berarti bila
`strokeWidth` lebih dari nol. Kosong berarti `solid`.

**Panjang segmennya kelipatan `strokeWidth`**, bukan angka mutlak, supaya polanya
tetap sebanding ketika garis ditebalkan. Tulis `w` untuk `strokeWidth`:

| Nilai | `stroke-dasharray` | Terlihat seperti |
|---|---|---|
| `solid` | — | ———————— |
| `longdash` | `4w 2w` | —— —— —— |
| `dash` | `2w 2w` | – – – – |
| `dot` | `w 2w` | · · · · · |

Angka ini **wajib sama di kedua renderer.** Backend menggambarnya di PDF dari
`design.StrokeDashPattern`, satu-satunya tempat angkanya hidup di sisi ini; dua
renderer yang masing-masing menebak polanya pasti berbeda, dan perbedaannya baru
terlihat setelah dicetak.

**Ujung segmen dipotong rata**, tidak dibulatkan — itu bawaan SVG maupun PDF,
jadi keduanya sepakat tanpa siapa pun menyetel apa pun. Konsekuensinya `dot`
adalah kotak kecil, bukan lingkaran. Membulatkannya di satu sisi saja justru
membuat keduanya berbeda.

Nilai di luar keempatnya ditolak seperti properti asing lain.

**`title` pada halaman tidak digambar.** Ia sebutan halaman di editor — daftar
halaman, panel thumbnail — sedangkan judul yang tampil di atas kertas adalah
elemen teks biasa. Keduanya tidak berhubungan dan boleh berbeda.

**`locked` dan `groupId` tidak digambar sama sekali.** Renderer mengabaikan
keduanya; mereka ada supaya editor punya tempat menyimpannya, dan supaya ikut
tersalin ketika elemen berpindah. `locked` **tidak ditegakkan backend** — ia
mencegah kecelakaan, bukan mencegah klien yang memang mengirim perubahan.
`groupId` datar, tanpa penyusunan bersarang, dan backend tidak menjamin anggota
satu grup bersebelahan dalam urutan gambar.

> **`element.update` mengganti elemen seutuhnya.** Update yang tidak menyertakan
> `locked` dan `groupId` akan menghapus keduanya. Kirim balik seluruh field yang
> Anda terima, termasuk yang tidak Anda pedulikan.

Satu dokumen dibatasi **200 halaman**; jumlah elemen tidak dibatasi. Halaman
berbiaya jauh melampaui isinya karena tiap halaman menjadi satu halaman PDF yang
digambar dari nol saat ekspor.

### 1.3 Properti bersama

| Properti | Tipe | Keterangan |
|---|---|---|
| `id` | string | tidak kosong, unik se-dokumen |
| `type` | string | `text`, `rect`, `ellipse`, `line`, `image` |
| `x`, `y` | number | sudut kiri atas kotak, relatif terhadap sudut kiri atas halaman |
| `w`, `h` | number | lebar dan tinggi; tidak boleh negatif kecuali pada `line` |
| `rotation` | number | derajat, **searah jarum jam**, terhadap titik tengah kotak |
| `opacity` | number | `0` tembus pandang sampai `1` pekat; bawaan `1` |

Nilai di luar ±100000 ditolak.

`rotation` disamakan dengan `transform: rotate(Ndeg)` berpasangan dengan
`transform-origin: center`, jadi frontend tidak perlu menerjemahkan apa pun.
Nilainya tidak dinormalkan — `370` dikembalikan sebagai `370`. Pada `line`, titik
tengah kotak kebetulan juga titik tengah garisnya.

**`opacity` bernilai `0` adalah nilai yang sah**, dan berbeda dari tidak
menyebutkannya. Kirim `0` bila memang ingin tembus pandang, dan hilangkan
field-nya bila ingin bawaan; jangan mengirim `0` untuk mengatakan "bawaan". Di
luar `0..1` ditolak `element_rejected`.

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
| `underline` | boolean | seluruh isi elemen |
| `strikethrough` | boolean | seluruh isi elemen; boleh bersamaan dengan `underline` |
| `verticalAlign` | string | `top`, `middle`, `bottom` |
| `paddingTop` … `paddingLeft` | number | ≥ 0; empat sisi terpisah, tanpa bentuk ringkas |
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

### 1.5.1 `ellipse`

Properti yang sama persis dengan `rect`, tanpa `radius` — sudut membulat tidak
berarti apa-apa pada bidang yang tidak bersudut. `radius` yang telanjur terbawa
diabaikan, bukan ditolak, supaya mengubah `rect` menjadi `ellipse` tidak menuntut
pembersihan properti yang tidak terlihat pengaruhnya.

Bidangnya **pas di dalam kotak** `x`/`y`/`w`/`h`, sama seperti `<ellipse>` di SVG
yang `cx`, `cy`, `rx`, dan `ry`-nya diturunkan dari kotak itu:

```jsx
<ellipse
  cx={el.x + el.w / 2} cy={el.y + el.h / 2}
  rx={el.w / 2} ry={el.h / 2}
/>
```

Lingkaran adalah kotak yang `w` dan `h`-nya sama. Tidak ada jenis `circle`
tersendiri: dua cara menyatakan satu bentuk adalah dua cara untuk tidak sepakat.

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
ditentukan lewat isi dokumen.

Untuk menampilkannya, tunjuk saja endpoint isinya:

```html
<img src="/api/asset-content/{assetToken}">
```

URL itu **tetap dan tidak pernah kedaluwarsa** — yang kedaluwarsa adalah sasaran
pengalihannya, dan itu disusun ulang pada setiap permintaan. Tidak perlu memanggil
apa pun lebih dulu, tidak perlu menyimpan URL bertanda tangan, tidak perlu
menyegarkannya.

Endpoint itu **tidak menuntut `Authorization`**, karena tag `<img>` memang tidak
dapat mengirimnya; token asetnya yang menjadi kredensial. `GET /api/asset-presign/:token`
tetap ada dan tetap menuntut login — pakai itu bila Anda memang perlu URL-nya
sendiri, misalnya untuk mengunduh.

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

Yang menentukan hasil cetak sama dengan layar bukan nama font yang sama, melainkan
**lebar maju yang sama**. Selisih sekecil apa pun menumpuk menjadi pemenggalan
baris yang berbeda antara layar dan cetak.

#### Mode bawaan: tanpa berkas font sama sekali

Backend berjalan tanpa satu berkas font pun. Dalam keadaan itu ia memakai
Helvetica inti PDF, yang metriknya melekat pada spesifikasi PDF dan tidak perlu
disematkan. Ini keadaan yang didukung, bukan keadaan darurat.

CSS Anda harus menunjuk font yang lebarnya sepadan dengan Helvetica:

```css
font-family: Helvetica, Arial, "Liberation Sans", "Nimbus Sans", sans-serif;
font-kerning: none;
font-feature-settings: "liga" 0;
```

Bukan kebetulan ketiganya berderet: Arial memang diciptakan sebagai pengganti
Helvetica dengan lebar maju yang sama persis, dan Liberation Sans serta Nimbus
Sans dirancang selebar itu pula. Yang berbeda bentuk glifnya, bukan lebarnya.

**Jangan menulis `sans-serif` telanjang, dan jangan `system-ui`.** Keduanya
menyerahkan pilihan ke sistem: di banyak Linux jatuh ke DejaVu Sans yang lebih
lebar, di Android ke Roboto, di macOS `system-ui` berarti San Francisco. Ketiganya
bermetrik lain, dan hasilnya teks yang di layar muat satu baris menjadi dua baris
di PDF.

**Ketebalannya hanya 400 dan 700.** Helvetica inti tidak punya yang lain, dan
permintaan di luar keduanya dibulatkan saat cetak — lihat bagian pembulatan di
bawah. Karena itu yang benar di toolbar adalah tombol **B** dua keadaan, bukan
pemilih sembilan tingkat: tingkat yang tidak dapat dicetak hanya menjanjikan
sesuatu yang tidak akan muncul di PDF.

Miring tersedia penuh, tegak maupun tebal.

#### Bila suatu saat memakai font sendiri

Selama backend belum mendaftarkan berkas font, bagian ini tidak berlaku dan
tumpukan CSS di atas sudah cukup. Begitu ada berkas yang didaftarkan, **berkas
yang sama persis** wajib disajikan ke browser lewat `@font-face` — bukan Google
Fonts, bukan font sistem, karena keduanya dapat berbeda revisi:

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

#### Yang diminta tidak ada: dibulatkan, tidak menggagalkan

Ekspor **tidak pernah gagal** karena font. Permintaan yang tidak dapat dipenuhi
apa adanya diturunkan bertahap:

| Yang diminta | Yang dicetak |
|---|---|
| Potongan yang terdaftar | Itu juga |
| Ketebalan yang tidak terdaftar, keluarga dan gaya terdaftar | Ketebalan **terdekat** pada keluarga yang sama — rupa hurufnya tetap benar |
| Keluarga atau gaya yang tidak terdaftar | Helvetica inti, ketebalan dibulatkan ke 400 atau 700 |

Seri dibulatkan ke arah yang lebih ringan: `500` menjadi `400`, `600` menjadi
`700`.

**Ini terjadi diam-diam bagi pengguna.** Tidak ada pesan kesalahan, tidak ada
tanda di PDF-nya. Yang ada hanya satu baris log peringatan di server per pasangan
diminta-dipakai, beserta jumlah elemen yang terpengaruh.

Karena itu tanggung jawabnya berpindah ke editor: **jangan menawarkan ketebalan
yang tidak disebut `faces`.** Toolbar dengan sembilan tingkat ketebalan di atas
backend tanpa berkas font akan mencetak Medium 500 sebagai Regular dan Semibold
600 sebagai Bold — dan pengguna baru menyadarinya setelah dokumennya jadi. Dengan
font inti saja, yang benar adalah tombol **B** dua keadaan, bukan pemilih
ketebalan.

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

#### Sisi dalam dan perataan tegak

```css
padding: 8px 12px;      /* paddingTop/Bottom 8, paddingLeft/Right 12 */
```

Empat sisi terpisah, **tanpa bentuk ringkas `padding`** di dalam data. Model ini
datar, dan menyediakan keduanya berarti aturan presedensi yang harus disepakati
dua sisi.

Sisi dalam **mengecilkan ruang tempat teks dipenggal**, jadi ia ikut mengubah
jumlah baris — bukan sekadar menggeser. Kotak yang sudah pas isinya dapat
mendadak kekurangan satu baris begitu sisi dalamnya ditambah.

Kliping tetap memakai **kotak penuh**, sama seperti `overflow: hidden` di CSS
yang memotong pada kotak padding, bukan pada kotak isi.

Perataan tegak memakai flex, bukan `vertical-align` — yang di CSS hanya berlaku
pada sel tabel dan elemen sebaris:

```css
display: flex;
flex-direction: column;
justify-content: flex-start;  /* top    */
justify-content: center;      /* middle */
justify-content: flex-end;    /* bottom */
```

Blok yang lebih tinggi daripada kotaknya **tetap digeser**, sehingga pada
`middle` dan `bottom` bagian atasnya yang terpotong. Itu disengaja: menjepitnya ke
`top` berarti perataan berubah diam-diam tepat ketika isinya bertambah satu baris.

#### Garis bawah dan coret

```css
text-decoration: underline;              /* underline */
text-decoration: line-through;           /* strikethrough */
text-decoration: underline line-through; /* keduanya */
```

Keduanya berlaku pada **seluruh isi elemen**. Menggarisbawahi satu kata di tengah
paragraf menuntut teks kaya, dan itu belum ada.

**Jangan menyetel `text-underline-offset` maupun `text-decoration-thickness`.**
Backend menurunkan letak dan tebalnya dari ukuran huruf — 0,1 em di bawah garis
dasar setebal 0,05 em, dan coret 0,4 em di atasnya. Angka itu metrik Helvetica,
jadi untuk keluarga inti ia persis; menyetel nilai sendiri di CSS hanya membuat
layar dan cetak berbeda.

Garisnya **menerus mengikuti lebar teks**, termasuk pada perataan penuh yang
selanya melebar dan pada teks yang diberi jarak antar huruf. Pada baris terakhir
paragraf berperataan penuh — yang memang tidak diregangkan — garisnya berhenti di
ujung teks, bukan di tepi kotak.

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
      <g
        transform={
          el.rotation
            ? `rotate(${el.rotation} ${el.x + el.w / 2} ${el.y + el.h / 2})`
            : undefined
        }
        opacity={el.opacity ?? 1}
      >
        {el.type === 'rect' ? (
          <rect
            x={el.x} y={el.y} width={el.w} height={el.h} rx={el.radius || 0}
            fill={el.fill || 'none'}
            stroke={el.stroke || 'none'}
            strokeWidth={el.strokeWidth || 0}
          />
        ) : el.type === 'ellipse' ? (
          <ellipse
            cx={el.x + el.w / 2} cy={el.y + el.h / 2}
            rx={el.w / 2} ry={el.h / 2}
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
      </g>
    </svg>
  );
}
```

**`el.opacity ?? 1`, bukan `el.opacity || 1`.** Nol adalah nilai yang sah, dan
nol itu falsy — `||` akan mengubah elemen yang sengaja dibuat tembus pandang
menjadi pekat sepenuhnya. Jebakan yang sama berlaku di mana pun `opacity`
dibaca.

`rotate()` di SVG **wajib menyertakan titik pusatnya**. `rotate(45)` telanjang
berputar terhadap titik asal koordinat, bukan terhadap elemen, dan hasilnya
elemen terlempar jauh dari tempatnya.

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

Sumber gambarnya cukup `/api/asset-content/{assetToken}` — tetap, tanpa masa
berlaku, tanpa panggilan pendahuluan. Isi dokumen menyimpan token, bukan URL:
backend tidak pernah mengambil alamat yang ditentukan lewat isi dokumen.

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

### 2.7 Putaran, transparansi, dan latar halaman

Ketiganya ditambahkan belakangan dan mudah tergambar berbeda antara layar dan
cetak, karena masing-masing punya lebih dari satu cara yang "masuk akal".

**Putaran** searah jarum jam terhadap titik tengah kotak. Di CSS berarti:

```css
transform: rotate(45deg);
transform-origin: center;
```

Di SVG, `transform="rotate(45, cx, cy)"` dengan `cx`/`cy` titik tengah kotak —
bukan `rotate(45)` telanjang, yang berputar terhadap titik asal koordinat.

Putaran **tidak** mengubah `x`, `y`, `w`, dan `h`: keduanya tetap kotak sebelum
diputar. Kotak pemilihan dan pegangan ubah-ukuran di editor karena itu perlu ikut
diputar, bukan dihitung ulang dari kotak yang membungkus hasil putarannya.

**Transparansi** memakai `opacity` CSS pada elemen — bukan warna ber-alpha.
Backend menerapkannya pada seluruh elemen sekaligus, isi dan garis tepi bersama,
sama seperti `opacity` di CSS dan berbeda dari `fill-opacity`.

**Latar halaman** digambar sebelum elemen mana pun. Kosong berarti tidak digambar
sama sekali, dan itu **tidak sama dengan putih**: dicetak di atas kertas berwarna,
keduanya berbeda. Jangan menirunya dengan `rect` seukuran halaman — latar tidak
boleh ikut terpilih atau tergeser saat pengguna menekan pilih-semua.

Latar disetel lewat `page.update`, yang mewajibkan **keempat** field-nya dikirim
bersama. Lihat [kontrak WebSocket](websocket-contract.md).

### 2.8 Halaman jamak

Seluruh halaman satu dokumen berukuran sama, jadi `page` dari snapshot berlaku
untuk semuanya. Tampilkan bertumpuk ke bawah; jarak antar halaman sepenuhnya
urusan tampilan dan tidak ada di dalam data.

Setiap halaman perlu `overflow: hidden`. Elemen yang koordinatnya di luar batas
kertas **sah** menurut validasi backend — hanya nilai di luar ±100000 yang
ditolak — tetapi tidak akan tergambar di PDF karena berada di luar area cetak.
Tanpa kliping, layar akan menampilkan sesuatu yang tidak pernah ikut tercetak.

### 2.9 Mengukur teks sebelum menempatkannya

Lewat WebSocket, bukan HTTP — satu jalur dengan penyuntingan:

```jsonc
{ "type": "text.measure", "requestId": "m-1",
  "elements": [ { "id": "syarat", "type": "text", "x": 40, "y": 0, "w": 300, "h": 0,
                  "text": "Harga berlaku empat belas hari…", "fontSize": 10 } ] }
```

```jsonc
{ "type": "text.measured", "requestId": "m-1",
  "elements": [ { "id": "syarat", "lines": 4, "height": 48, "width": 297.3 } ] }
```

`height` adalah tinggi kotak yang dibutuhkan, dan itulah nilai yang dipasang ke
`h`. Bentuk lengkap pesannya beserta aturannya ada di
[kontrak 2.13](websocket-contract.md#213-textmeasure).

**Dua kegunaannya.**

Tombol **"sesuaikan tinggi kotak dengan isinya"**. Frontend bisa menghitung
sendiri, tetapi hasilnya hanya sebaik kecocokan pemenggalannya dengan backend —
dan itu justru kelas ketidakcocokan yang paling sering membuat cetakan berbeda
dari layar. Bertanya ke backend menghapusnya.

**Penempatan terprogram.** Penyusun yang bukan manusia tidak dapat melihat di
mana satu blok berakhir; ia bekerja menurun — taruh judul, tanyakan tingginya,
taruh berikutnya di bawahnya. Tanpa ini, panjang teks yang tidak diketahui di
muka membuat seluruh tata letak menjadi tebakan. Selisihnya nyata: teks yang sama
menjadi 2 baris pada `w: 300` dan 5 baris pada `w: 150`.

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
| `font-family: sans-serif` atau `system-ui` | Dipaku ke `Helvetica, Arial, "Liberation Sans", "Nimbus Sans", sans-serif` | [2.2](#22-font-berkas-yang-sama-di-kedua-sisi) |
| Pemilih ketebalan sembilan tingkat | Tombol B: 400 atau 700 | [2.2](#22-font-berkas-yang-sama-di-kedua-sisi) |
| Berkas font sendiri diambil dari Google Fonts | Berkas yang sama persis dengan backend, lewat `@font-face` | [2.2](#22-font-berkas-yang-sama-di-kedua-sisi) |
| Kotak teks tumbuh mengikuti isinya | Tinggi tetap dari `h`; isi yang melebihi terpotong | [2.3](#23-elemen-teks) |
| Kotak digambar dengan `div` + `border` | `<rect>` di dalam SVG | [2.4](#24-kotak-dan-garis-pakai-svg-bukan-div) |
| Garis digambar dengan `div` tipis | `<line>` di dalam SVG | [2.4](#24-kotak-dan-garis-pakai-svg-bukan-div) |
| `rotate()` tanpa `transform-origin: center` | Terhadap titik tengah kotak | [2.7](#27-putaran-transparansi-dan-latar-halaman) |
| `x`/`y`/`w`/`h` dihitung ulang setelah diputar | Tetap kotak sebelum diputar | [2.7](#27-putaran-transparansi-dan-latar-halaman) |
| Transparansi lewat warna ber-alpha | `opacity` pada elemen | [2.7](#27-putaran-transparansi-dan-latar-halaman) |
| `vertical-align` untuk meratakan tegak | `display: flex` + `justify-content` | [2.3](#23-elemen-teks) |
| Sisi dalam ditiru dengan menggeser `x` dan mengecilkan `w` | `paddingLeft` dan kawan-kawan | [2.3](#23-elemen-teks) |
| Latar halaman ditiru dengan `rect` seukuran halaman | `background` pada halaman | [2.7](#27-putaran-transparansi-dan-latar-halaman) |
| Gambar memakai URL langsung | `assetToken` + `<img src="/api/asset-content/:token">` | [2.5](#25-gambar) |

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
3. **Paku font.** Setel `font-family` ke daftar yang lebar majunya sepadan
   dengan Helvetica menurut [2.2](#22-font-berkas-yang-sama-di-kedua-sisi), dan
   batasi pilihan ketebalan pada 400 dan 700.
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
