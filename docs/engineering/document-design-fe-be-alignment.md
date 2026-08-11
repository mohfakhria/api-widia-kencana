# Document Design — Penyelarasan Frontend ↔ Backend

Dokumen bersama, dan **satu-satunya** dokumen document-design yang ada.
Kedua sisi menulis di sini.

> **Berkas ini hidup di dua repo dan disalin manual.** Yang paling akhir
> disunting adalah yang benar. Setiap perubahan **wajib** menambah satu baris
> di [§8 Riwayat keputusan](#8-riwayat-keputusan) dengan tanggal dan pemilik —
> itulah satu-satunya cara membedakan salinan yang tertinggal dari salinan
> yang memang tidak berubah.

> **Ini satu-satunya tempat, dan tidak menunjuk ke mana pun.** Tidak ada
> panduan pendamping, tidak ada deklarasi tipe untuk disalin, tidak ada
> lampiran di repo sebelah. Apa pun yang perlu disepakati dua pihak ada di
> sini atau tidak ada sama sekali.
>
> Harganya perlu diketahui berdua, dan tercatat di
> [§8](#8-riwayat-keputusan): dulu ada deklarasi tipe terbitan backend, dan
> frontend mengunci tipe wire-nya pada deklarasi itu sehingga kontrak yang
> berubah menggagalkan kompilasi sebelum ada yang sempat salah paham. Itulah
> yang menangkap pencabutan `text.measure`. Sekarang **tidak ada lagi
> pemeriksaan otomatis apa pun**: perubahan kontrak yang tidak ditulis di
> [§8](#8-riwayat-keputusan) akan muncul sebagai bug di produksi, bukan
> sebagai galat build.

---

## Cara pakai dokumen ini

Isinya adalah hal yang **butuh kesepakatan dua pihak** atau di mana **satu
sisi butuh sesuatu dari sisi lain**: pembagian tanggung jawab, harapan atas
permintaan dan balasan, style yang dipakai, behavior yang harus ditiru,
perbedaan yang disengaja, dan riwayat perubahan.

Bentuk setiap pesan dan setiap field **tidak** ada di sini, dan itu keputusan
yang perlu ditinjau ulang bila mulai terasa: bentuknya sekarang hanya hidup
sebagai kode di kedua sisi — `document-wire.ts` dan `document-edits.ts` di
frontend, penanganan pesan di backend. Menuliskannya di sini akan membuat
dokumen ini jauh lebih panjang dan menambah satu salinan lagi yang bisa
menyimpang; tidak menuliskannya berarti tidak ada satu tempat pun yang bisa
dibaca kedua pihak untuk menjawab "field ini bentuknya apa".

**Kepemilikan per section:**

| Section | Siapa yang menambah baris |
|---|---|
| §1 Pembagian tanggung jawab | Keduanya, hanya lewat kesepakatan |
| §2 Harapan permintaan & balasan | Keduanya, masing-masing pada sub-bagiannya |
| §3 Style yang dipakai frontend | **Frontend** |
| §4 Behavior tampil | **Frontend** mendeklarasikan, **backend** menandai bila tidak bisa ditiru |
| §5 Perbedaan yang disengaja | Keduanya |
| §6 Permintaan terbuka | Keduanya |
| §7 Status implementasi | Masing-masing mengisi kolomnya sendiri |
| §8 Riwayat keputusan | Keduanya |
| §9 Perilaku socket | **Backend** — frontend hanya menyusunnya ulang dari implementasinya sendiri, lihat catatan di sana |

Tidak ada bagian yang boleh menunjuk berkas di luar dokumen ini. Rujukan ke
kode boleh, sebagai petunjuk letak — bukan sebagai tempat menyimpan
kesepakatan.

---

## 1. Pembagian tanggung jawab

**Frontend memegang style dan behavior tampil.** Kanvas adalah tempat style
itu hidup — ia yang menentukan bagaimana sebuah dokumen terlihat, dan ia yang
menambah kemampuan baru saat kebutuhan desain muncul.

**Backend memegang penyimpanan dan reproduksi.** Yang dibutuhkan darinya ada
tiga, dan hanya tiga:

1. **Persistence** — menyimpan dokumen apa adanya.
2. **Sinkronisasi** — menyiarkan perubahan ke penyunting lain, menyimpan
   keadaan sesi di memori, dan mengelompokkan langkah undo/redo.
3. **Reproduksi** — menggambar ulang style yang frontend deklarasikan, untuk
   **ekspor PDF dan preview**, sehingga hasil cetak sama dengan kanvas.

Konsekuensi yang mengikat, dan inilah alasan dokumen ini ada:

- **Style baru dimulai dari frontend.** Backend tidak perlu menyetujuinya;
  backend perlu *bisa menggambarnya*. Jadi kewajiban frontend adalah
  **mendeklarasikan style apa saja yang dipakai** — itulah [§3](#3-style-yang-dipakai-frontend) — dan
  **behavior tampilnya saja**, yang ada di [§4](#4-behavior-tampil-yang-harus-ditiru).
- **Behavior interaksi bukan urusan backend.** Cara handle ditarik, kapan
  toolbar muncul, bagaimana teks disunting — tidak ada satu pun yang perlu
  diketahui backend, dan tidak boleh masuk dokumen ini. Yang masuk hanyalah
  behavior yang **mengubah piksel yang tergambar**.
- **Backend tidak menghitung apa pun tentang tampilan.** Ia tidak mengukur
  teks, tidak menentukan tinggi kotak, tidak memilih patahan baris untuk
  frontend. Ia menerima angka jadi dan menggambarnya. Ini pernah berbeda —
  lihat [§8](#8-riwayat-keputusan), 2026-08-11.

---

## 2. Harapan permintaan & balasan

Yang di bawah ini adalah **apa yang masing-masing pihak janjikan tentang isi
pesan**, dan apa yang berhak diharapkan sebagai balasan — bukan bentuknya,
melainkan hal-hal yang memang tidak bisa dinyatakan oleh sebuah bentuk.

### 2.1 Frontend → backend

**Yang frontend janjikan pada setiap kiriman:**

| Janji | Alasan |
|---|---|
| Setiap `element.update` membawa elemen **seutuhnya** | `element.update` mengganti, bukan menambal. Field yang tidak disertakan terhapus untuk semua orang |
| Semua panjang dalam **point** | Kanvas bekerja dalam paper px pada 96 dpi; konversi terjadi di satu tempat, `document-edits.ts` |
| `lineHeight` dikirim **tanpa konversi** | Ia pengali ukuran huruf, bukan panjang. Satu-satunya angka yang tidak dikonversi |
| Warna selalu heksadesimal `#rgb` atau `#rrggbb` | Nama warna CSS, `rgba()` dan `hsl()` ditolak. Warna tak terbaca dibuang di sisi frontend, bukan diteruskan |
| `fontWeight` hanya 400 atau 700 | Toolbar membatasinya. Nilai lain akan dibulatkan diam-diam saat ekspor |
| `rotation` dan `opacity` selalu disertakan | Keduanya konkret di model frontend. `opacity: 0` sah dan berbeda dari tidak menyebutkannya |
| Frontend tidak mengirim field yang tidak ada di kontrak | Field lokal berhenti di batas wire — lihat [§6](#6-permintaan-terbuka) bila sebuah field lokal perlu naik |

**Yang frontend harapkan sebagai balasan:**

- Disimpan **apa adanya** dan dikembalikan apa adanya. Pembulatan yang
  dilakukan backend harus tercatat di [§5](#5-perbedaan-yang-disengaja) —
  pembulatan diam-diam akan dikirim balik oleh frontend pada update
  berikutnya dan menetap selamanya.
- Perubahan disiarkan ke **seluruh** penghuni — **termasuk pengirimnya.**
  Backend sengaja tidak melewati pengirim: kalau ia dilewati, nomor `version`
  miliknya tertinggal setiap kali ia menyunting, lalu siaran pertama dari orang
  lain terlihat melompat dan ia memuat ulang seluruh dokumen tanpa sebab.
  Mengirimkannya kembali jauh lebih murah daripada itu.

  Jadi frontend akan menerima suntingannya sendiri kembali, dan harus
  mengabaikannya alih-alih memperlakukannya sebagai perubahan orang lain.
  **Kursor sama saja** — satu muatan berisi seluruh kursor dikirim ke semua
  penghuni, jadi kursor sendiri ikut kembali. Yang membedakan keduanya bukan
  siapa yang menerima, melainkan jaminannya: perubahan wajib tiba dan klien
  yang tertinggal diputus, sedangkan kursor boleh dibuang dan yang terbaru
  menimpa yang tertunda.
- Ekspor PDF menggambar tepat seperti [§3](#3-style-yang-dipakai-frontend) dan
  [§4](#4-behavior-tampil-yang-harus-ditiru) menyebutkan.
- Penolakan menyebutkan **field mana** yang salah, bukan hanya bahwa pesannya
  salah.

### 2.2 Backend → frontend

**Yang backend janjikan:**

| Janji | Alasan |
|---|---|
| Snapshot memuat keadaan lengkap, termasuk ukuran kertas | Frontend tidak menebak A4; kertas adalah properti dokumen |
| Perubahan kontrak diumumkan di [§8](#8-riwayat-keputusan) **sebelum** dikirimkan | Satu-satunya peringatan yang tersisa. Tidak ada lagi pemeriksaan yang menggagalkan build; pesan baru yang tidak diumumkan akan jatuh ke `default` router frontend dan **dibuang tanpa suara**, persis seperti empat pesan `page.*` dulu |
| Fitur yang **dicabut** diumumkan di [§6](#6-permintaan-terbuka) sebelum dicabut | Ini pernah tidak terjadi — lihat [§8](#8-riwayat-keputusan), 2026-08-11 |
| Nilai bawaan untuk field yang tidak dikirim tidak berubah tanpa pengumuman | Frontend membaca dengan bawaan; bawaan yang bergeser mengubah dokumen lama |

**Yang backend harapkan:**

- Style baru **dideklarasikan di [§3](#3-style-yang-dipakai-frontend)** sebelum
  dipakai di produksi, supaya ekspor tidak tertinggal.
- Perbedaan yang frontend pilih dengan sengaja dicatat di
  [§5](#5-perbedaan-yang-disengaja), bukan hanya sebagai komentar di repo
  frontend.
- Frontend tidak mengandalkan galat sebagai validasi. Yang divalidasi backend
  ada di kontrak; sisanya divalidasi frontend sebelum dikirim.

### 2.3 Yang tidak boleh diharapkan

| Bukan tanggung jawab | Siapa yang menanggung |
|---|---|
| Backend mengukur teks atau menghitung tinggi kotak | Frontend, dengan pengukuran browser |
| Backend menentukan patahan baris untuk frontend | Frontend, mengikuti [§4](#4-behavior-tampil-yang-harus-ditiru) |
| Backend menegakkan `locked` | Penanda editor saja; backend tetap menerima update pada elemen terkunci |
| Backend menjaga keutuhan grup | `groupId` adalah tanda datar; lima elemen segrup adalah lima pesan |
| Frontend menyimpan apa pun secara lokal yang harus selamat dari reload | Backend, lewat wire |

---

## 3. Style yang dipakai frontend

Deklarasi. Setiap baris adalah sesuatu yang harus muncul di PDF dan preview.
Kolom terakhir adalah yang backend butuhkan.

### 3.1 Berlaku untuk semua elemen

| Style | Field wire | Nilai | Yang harus digambar |
|---|---|---|---|
| Posisi & lebar | `x`, `y`, `w` | Point, relatif sudut kiri-atas halaman | Kotak elemen |
| Tinggi | `h` | Point | Semua tipe kecuali `line` |
| Putaran | `rotation` | Derajat searah jarum jam, terhadap **titik tengah kotak**, tidak dinormalkan | Setara `transform: rotate(Ndeg)` dengan `transform-origin: center`. Tidak menggeser `x`/`y`/`w`/`h` |
| Transparansi | `opacity` | 0..1 | Diterapkan pada elemen **seutuhnya** — isi dan garis tepi sekaligus, seperti `opacity` CSS, bukan `fill-opacity` |
| Pemotongan | — | — | Isi yang melebihi `w`/`h` dipotong, setara `overflow: hidden` |

### 3.2 Teks

| Style | Field wire | Nilai yang dipakai frontend | Yang harus digambar |
|---|---|---|---|
| Keluarga huruf | `fontFamily` | `helvetica` saja | Helvetica inti PDF |
| Ukuran huruf | `fontSize` | Point, 6–288 | — |
| Ketebalan | `fontWeight` | 400 atau 700 saja | — |
| Miring | `fontStyle` | `normal` \| `italic` | — |
| Warna | `color` | Heksadesimal | — |
| Perataan | `align` | `left` \| `center` \| `right` \| `justify` | — |
| Tinggi baris | `lineHeight` | **Pengali** ukuran huruf, mis. 1.2 | Tinggi kotak baris = `fontSize × lineHeight` |
| Jarak antar huruf | `letterSpacing` | Point, boleh negatif | Ditambahkan **setelah** setiap glyph |

CSS yang dipakai kanvas untuk kedelapan hal di atas ada di satu tempat,
`textLayoutStyle` — dan tempat yang sama itulah yang dipakai renderer,
penyunting, dan pengukur. Bila ada yang tidak tergambar sama, di situlah
mencarinya.

### 3.3 Bentuk dan garis

| Style | Field wire | Nilai | Yang harus digambar |
|---|---|---|---|
| Isi | `fill` | Heksadesimal, `""` berarti tanpa isi | Kosong ≠ putih |
| Garis tepi | `stroke` | Heksadesimal, `""` berarti tanpa garis | Digambar hanya bila `strokeWidth > 0` |
| Tebal garis | `strokeWidth` | Point | **Terpusat pada jalurnya**, seperti SVG — bukan seperti `border` CSS yang menaruh seluruhnya di satu sisi |
| Gaya garis | `strokeStyle` | `solid` \| `longdash` \| `dash` \| `dot` | Kelipatan tebal garis: `longdash` = 4/2, `dash` = 2/2, `dot` = 1/2. Ujung segmen **dipotong rata**, bukan dibulatkan |
| Sudut membulat | `radius` | Point | Dibatasi separuh sisi terpendek. Berlaku pada `rect` **dan gambar** — pada gambar ia memotong, seperti `border-radius` pada `<img>` |

Garis (`line`) adalah kasus khusus: `h` selalu 0, dan `y` yang dikirim
frontend sudah digeser separuh tebalnya, supaya kotak elemen mengapit
jalurnya persis seperti stroke.

### 3.4 Gambar

| Style | Field wire | Nilai | Yang harus digambar |
|---|---|---|---|
| Berkas | `assetToken` | Token aset | — |
| Cara mengisi | `fit` | `contain` \| `cover` | `contain` memuat seluruh gambar di dalam kotak; `cover` mengisi kotak dan memotong kelebihannya |

### 3.5 Halaman

| Style | Field wire | Nilai | Yang harus digambar |
|---|---|---|---|
| Latar | `background` | Heksadesimal, `""` berarti tanpa latar | Digambar sebelum elemen mana pun. Tanpa latar ≠ putih — di atas kertas berwarna keduanya berbeda |
| Ukuran kertas | dari snapshot | Point | Bukan A4 secara otomatis |

---

## 4. Behavior tampil yang harus ditiru

Hanya yang **mengubah piksel**. Behavior interaksi tidak ada di sini.

| Aturan | Yang dilakukan kedua sisi |
|---|---|
| Pemenggalan baris | Rakus: kata ditambahkan selama masih muat, pindah baris begitu tidak. Tanpa tanda hubung otomatis |
| Spasi | Deretan spasi dan tab menjadi satu pemisah; `\n` dihormati; baris kosong tetap memakan satu tinggi baris. Setara `white-space: pre-line` |
| Kerning | **Dimatikan.** Browser merapatkan pasangan huruf tertentu atas inisiatifnya sendiri; backend tidak |
| Ligatur | **Dimatikan.** `fi` tidak menyatu jadi satu glyph |
| Baseline | Half-leading: teks mengalir dari tepi **atas** kotak. Tidak ada penengahan vertikal |
| Margin, padding, border | Frontend memakai nol. Backend **punya** `paddingTop`/`Right`/`Bottom`/`Left` dan `verticalAlign`, dan rumus baseline-nya berangkat dari kotak isi — bukan kotak elemen. Selama frontend tidak mengirim keduanya, hasilnya identik |
| Pemotongan | Di `w` dan `h`, per elemen. Halaman juga memotong elemen yang keluar dari kertas |
| Satuan | Wire dalam point; kanvas dalam paper px pada 96 dpi. Keduanya menggambarkan ukuran fisik yang sama, jadi hubungannya 1:1 dengan PDF — bukan pendekatan |

**Pembulatan.** Panjang menyeberang sebagai point dan kembali sebagai paper
px. Perjalanan bolak-balik itu membulatkan, jadi angka yang dikirim dan angka
yang diterima **tidak pernah persis sama**. Jangan ada sisi yang menyimpulkan
apa pun dari kesamaan dua panjang.

---

## 5. Perbedaan yang disengaja

Bukan bug. Setiap baris punya pemilik yang bertanggung jawab menutupnya, atau
alasan mengapa ia dibiarkan terbuka.

| Aturan | Frontend | Backend | Alasan | Pemilik | Status |
|---|---|---|---|---|---|
| Kata lebih lebar dari kotaknya | **Dipatahkan** (`overflow-wrap: anywhere`) | Dibiarkan meluber, tidak dipatahkan | Menyamakannya tidak menyelamatkan apa pun: deretan tak terpatahkan sudah hilang dari PDF dengan cara apa pun. Yang didapat dari kesetiaan cuma kotak teks yang gagal memuat kata-katanya sendiri saat diketik — terbaca sebagai editor rusak, bukan sebagai peringatan cetak | Backend | Terbuka — lihat [§6](#6-permintaan-terbuka) |

---

## 6. Permintaan terbuka

| Permintaan | Dari | Untuk | Alasan | Status |
|---|---|---|---|---|
| Patahkan kata yang lebih lebar dari kotaknya | Frontend | Backend | Menutup satu-satunya perbedaan di [§5](#5-perbedaan-yang-disengaja). Kasus nyatanya bukan teks uji melainkan **URL panjang**, yang pasti muncul di dokumen sungguhan dan saat ini hilang dari PDF tanpa tanda apa pun | Diajukan 2026-08-11 |

---

## 7. Status implementasi

Menjawab **"siapa sudah mengerjakan apa, sekarang"** — berbeda dari riwayat
perubahan kontrak, yang menjawab "apakah kontraknya berubah". `text.measure`
gagal bukan karena perubahannya tak tercatat, melainkan karena statusnya tidak
pernah sama-sama terlihat.

| Fitur | Backend | Frontend | Terakhir diperiksa |
|---|---|---|---|
| Putaran, transparansi, elipsis, latar halaman | Selesai | Selesai | 2026-08-10 |
| `text.measure` / `text.measured` | **Dicabut** | **Dicabut** | 2026-08-11 |
| Pemenggalan kata panjang | Belum | Selesai (kanvas) | 2026-08-11 |
| Daftar font dari backend | **Dihapus** | Menyesuaikan — satu keluarga lewat CSS | 2026-08-10 |
| `underline` dan `strikethrough` pada teks | Selesai | Belum dipakai | 2026-08-11 |
| `verticalAlign` dan empat sisi `padding` pada teks | Selesai | Belum dipakai | 2026-08-11 |
| `radius` pada gambar | Selesai | Belum dipakai | 2026-08-11 |
| `fit: "fill"` pada gambar | Selesai | Belum dipakai — frontend memakai `contain` dan `cover` | 2026-08-11 |
| Penanganan `element_rejected` dan `page_rejected` | Dikirim sejak 2026-08-08 | Belum ditangani | 2026-08-11 |

---

## 8. Riwayat keputusan

Keputusan bersama **dan** perubahan kontrak. Keduanya di sini, karena keduanya
menjawab pertanyaan yang sama: apa yang berubah, sejak kapan, dan siapa yang
harus menindaklanjuti.

| Tanggal | Keputusan | Pemilik |
|---|---|---|
| 2026-08-11 | Dokumen ini dibuat. Pembagian tanggung jawab di [§1](#1-pembagian-tanggung-jawab) disepakati: frontend memegang style dan behavior tampil, backend memegang persistence, sinkronisasi, dan reproduksi untuk ekspor serta preview | Keduanya |
| 2026-08-11 | Kanvas mematahkan kata panjang, berbeda dari backend, dan perbedaannya dicatat alih-alih disembunyikan | Frontend |
| 2026-08-11 | `text.measure` dicabut di kedua sisi. Konsekuensinya perlu dicatat: **tidak ada lagi cara memverifikasi bahwa kanvas dan PDF sepakat** selain membandingkan ekspor dengan mata | Backend, diikuti frontend |
| 2026-08-11 | Tinggi kotak teks diturunkan dari isinya di sisi frontend; lebar tetap milik pengguna. Backend menerima keduanya sebagai angka jadi | Frontend |
| 2026-08-11 | §9 ditambahkan, disusun ulang dari implementasi frontend karena sumber aslinya sudah tidak disimpan. **Menunggu pemeriksaan backend** — angka yang tidak cocok akan muncul sebagai koneksi terputus di produksi, bukan sebagai galat | Frontend menyusun, backend memeriksa |
| 2026-08-11 | §9 diperiksa backend terhadap kode. §9.1 dan §9.4 cocok seluruhnya. Tiga koreksi ditulis: siaran perubahan **memantul ke pengirimnya** dan §2.1 sebelumnya menyatakan sebaliknya; `1006` punya arti ketiga — server membuang klien yang tertinggal; dan §9.5 kekurangan empat dari enam kode galat yang dikirim backend | Backend |
| 2026-08-11 | Kontrak disederhanakan jadi **satu** dokumen: yang ini. Deklarasi tipe terbitan backend dihentikan, dan bersamanya pin konformansi di frontend ikut dibuang — sebuah pin yang dipegang terhadap salinan beku selalu hijau apa pun yang berubah di hulu, yang lebih buruk daripada tidak ada pin. **Konsekuensinya: tidak ada lagi pemeriksaan otomatis atas perubahan kontrak.** Yang tersisa hanyalah §8 ini | Keduanya |

---

## 9. Perilaku socket yang tidak terlihat dari tipe

Deklarasi tipe menyebutkan bentuk setiap pesan, tetapi tidak bisa menyebutkan
kapan pesan itu boleh dikirim, apa artinya koneksi yang tertutup, atau berapa
lama sebuah tiket hidup. Semua itu ada di sini.

> **Bagian ini disusun ulang dari implementasi frontend yang berjalan**, karena
> sumber aslinya sudah tidak disimpan. Setiap angka di bawah diambil dari
> `design-socket.ts` dan `use-document-state.ts`, bukan dari ingatan.
>
> **Sudah diperiksa backend, 2026-08-11.** §9.1 dan §9.4 cocok seluruhnya
> dengan kode — termasuk kelima angka pengelompokan undo. Kelima close code di
> §9.2 memang yang dikirim server. Dua hal yang kurang sudah ditambahkan pada
> tempatnya: arti ketiga `1006`, dan empat kode galat yang belum tercantum di
> §9.5.

### 9.1 Tiket dan penyambungan

| Aturan | Nilai | Konsekuensi bila dilanggar |
|---|---|---|
| Umur tiket | 30 detik | Tiket yang dicetak lebih awal tiba sebagai `1008` |
| Pemakaian | Sekali pakai | Menyambung ulang dengan tiket yang sama selalu `1008` |
| Kuota per pengguna | 5; yang keenam menggusur yang tertua | Mencetak tiket saat mount atau saat hover menghabiskan kuota untuk koneksi yang tidak pernah terjadi |
| Waktu pencetakan | Tepat sebelum setiap percobaan sambung, tidak lebih awal | Sama seperti di atas |

Frontend menyambung ulang dengan backoff eksponensial: mulai 1 detik, batas
30 detik, jitter 25%. Percobaan langsung tanpa jeda dibatasi 2 kali berturut,
dan kegagalan handshake berturut dibatasi 3 kali sebelum berhenti — sebuah
alamat yang tidak menjawab akan tetap tidak menjawab tiga puluh detik lagi.

### 9.2 Close code

| Code | Arti | Yang dilakukan frontend |
|---|---|---|
| `1008` | Tiket basi atau sudah terpakai | Sambung ulang **segera** dengan tiket baru, maksimum 2 kali berturut |
| `1013` | Sepuluh koneksi hidup untuk pengguna ini | Backoff, tidak sebelum 5 detik. Menyambung lebih cepat tidak bisa menolong — sesuatu harus tertutup dulu |
| `1011` | Ruangnya berhenti bisa melayani kita di tengah sesi, misal dokumennya dihapus orang lain | **Berhenti.** Membiarkan orang menyunting apa yang tidak akan pernah tersimpan lebih buruk daripada berhenti |
| `1003` | Frontend mengirim frame biner | **Berhenti.** Itu bug frontend, dan menyambung ulang hanya mengulanginya |
| `1006` | **Tiga arti**, dan browser tidak bisa membedakannya | Yang membedakan hanyalah apakah socket-nya pernah terbuka. Handshake yang ditolak sebelum upgrade — Origin ditolak, URL salah, backend mati — tidak menghasilkan close frame sama sekali; begitu pula jaringan yang putus di tengah sesi. Yang pertama tidak layak disambung ulang, yang kedua layak |
| `1006` (lanjutan) | **Arti ketiga: server membuang klien ini karena tertinggal** | Antrean keluar penuh membuat backend membatalkan koneksi **tanpa** close frame, jadi ia tiba sebagai `1006` juga — lihat §9.3. Sambung ulang memang jawaban yang benar di sini, dan koneksi baru datang dengan antrean kosong. Yang perlu diketahui hanya diagnosisnya: `1006` di tengah sesi belum tentu jaringan |
| `1000` dari sisi server | Penutupan normal | Diperlakukan sebagai putus biasa dan disambung ulang |

### 9.3 Laju dan tekanan balik

| Aturan | Nilai |
|---|---|
| Jarak minimum antar `cursor.move` | 50 ms |
| Jarak minimum antar pesan sunting | 40 ms |
| Jarak minimum antar pemeriksaan saat perangkat bangun dari tidur | 3 detik |

Pesan sunting punya dua kelas, dan bedanya penting bagi kedua sisi. Pesan
**transient** — bingkai di tengah gestur — saling menimpa alih-alih mengantre,
karena pada satu drag hanya yang terbaru berarti. Pesan **settled** — yang
menutup gestur — tidak pernah ditunda dan tidak pernah dibuang.

Alasannya ada di sisi backend: **server memutus klien yang tertinggal, bukan
melewati pesannya.** Jadi sunting yang hilang bukan cuma piksel yang salah,
melainkan pekerjaan yang hilang, dan frontend membedakan kedua kelas itu
justru untuk tidak pernah tertinggal.

### 9.4 Pengelompokan langkah undo

Milik server, dan inilah yang menentukan apa yang dibatalkan satu Cmd+Z.

| Aturan | Nilai |
|---|---|
| `element.update` dan `page.update` berturut digabung jadi satu langkah | Ya |
| Batas jeda | 400 ms |
| Batas atas satu kelompok | 2 detik |
| Semua pesan lain — create, delete, reorder, dan `page.*` selain update | Selalu membuka langkahnya sendiri |

Konsekuensi bagi frontend, dan alasan ia menutup gestur secara eksplisit:
penggabungan itu dibatasi **jam, bukan oleh kita**. Bersandar padanya akan
membuat Cmd+Z bergantung pada secepat apa seseorang menarik pointer.

### 9.5 Galat yang datang saat koneksi tetap hidup

| Kode | Arti | Yang dilakukan frontend |
|---|---|---|
| `malformed_message` | Muatan pesan tidak sah atau kekurangan field wajib | Minta ulang seluruh dokumen — pesan yang ditolak berarti keadaan lokal mungkin sudah tidak sejalan |
| `document_unavailable` | Dokumen sementara tidak bisa dilayani | Coba lagi setiap 15 detik, maksimum 4 kali |
| `element_rejected` | **Pembuatan elemen tidak jadi** — muatannya tidak sah, id sudah dipakai, atau halamannya tidak ada. Hanya `element.create` yang membalas; `element.update`, `delete`, dan `reorder` yang sasarannya sudah lenyap **didiamkan**, karena itu lomba yang wajar pada menang-terakhir dan siaran penghapusannya toh sedang menuju ke sana | Belum ditangani. Ini yang paling perlu, karena artinya pekerjaan pengguna tidak tersimpan sementara layarnya menunjukkan sudah |
| `page_rejected` | **Perubahan halaman tidak jadi** — id sudah dipakai, batas 200 halaman tercapai, atau menghapus halaman terakhir. Hanya `page.create` dan `page.delete` yang membalas | Belum ditangani |
| `missing_message_type` | Pesan tanpa field `type` | Belum ditangani; ini bug frontend bila muncul |
| `unsupported_message_type` | Jenis pesan tidak dikenal server | Belum ditangani; muncul bila frontend berjalan lebih dulu dari backend |

Keenamnya dikirim backend hari ini. Empat yang terakhir belum punya penanganan
di frontend — ditambahkan backend 2026-08-11 supaya keberadaannya diketahui,
bukan sebagai keluhan. Yang mendesak hanya `element_rejected` dan
`page_rejected`: keduanya berarti suntingan tidak terjadi.
