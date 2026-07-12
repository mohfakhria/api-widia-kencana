# Wireframe Document Builder — Phase 1

## 1. Struktur Utama

Document Builder terdiri dari tiga area:

- **Elements** untuk menambahkan elemen.
- **Live Preview** untuk melihat dan berinteraksi langsung dengan halaman aktif.
- **Properties** untuk mengatur elemen yang sedang dipilih.

Section **Layer** berada di bawah Elements dan hanya menampilkan struktur dari halaman yang sedang aktif.

---

## 2. Final Wireframe — Page 1

```text
┌──────────────────────────────┬──────────────────────────────────────────────────────┬──────────────────────────┐
│ ELEMENTS                     │ LIVE PREVIEW                                         │ PROPERTIES               │
├──────────────────────────────┼──────────────────────────────────────────────────────┼──────────────────────────┤
│                              │                                                      │                          │
│ LAYOUT                       │   ┌───────────────────────────────┐                  │ Selected Element         │
│   + Grid                     │   │  −    72%    +    ▣    ↗     │                  │ Grid                     │
│   + Divider                  │   └───────────────────────────────┘                  │                          │
│   + Spacer                   │                                                      │ LAYOUT                   │
│   + Page Break               │          ┌──────────────────────────────────┐        │ Columns    : 2           │
│                              │          │                                  │        │ Gap        : 12 px       │
│ CONTENT                      │          │   ⠿ ┌────────────────────────┐   │        │ Width      : Auto        │
│   + Text                     │          │     │ GRID                   │   │        │ Height     : Auto        │
│   + Image                    │          │     │                        │   │        │                          │
│   + List                     │          │     │ ⠿ ┌────────┬────────┐  │   │        │ SPACING                  │
│   + Table                    │          │     │   │ Text   │ Text   │  │   │        │ Margin     : 0 px        │
│                              │          │     │   │        │        │  │   │        │ Padding    : 0 px        │
│ BUSINESS                     │          │     │   └────────┴────────┘  │   │        │                          │
│   + Signature                │          │     └────────────────────────┘   │        │ ALIGNMENT                │
│   + QR Code                  │          │                                  │        │ Horizontal : Start       │
│   + Barcode                  │          │   ⠿ Text                         │        │ Vertical   : Start       │
│                              │          │                                  │        │                          │
├──────────────────────────────┤          │                                  │        │                          │
│ LAYER                        │          │                                  │        │                          │
│                              │          │                                  │        │                          │
│ Page 1                 [+]   │          │                                  │        │                          │
│                              │          │                                  │        │                          │
│ ├─ Header                    │          │                                  │        │                          │
│ │  └─ ⠿ Grid ◀    [⧉] [🗑]  │          │                                  │        │                          │
│ │     ├─ ⠿ Image  [⧉] [🗑]  │          │                                  │        │                          │
│ │     └─ ⠿ Text   [⧉] [🗑]  │          │                                  │        │                          │
│ ├─ Body                      │          │                                  │        │                          │
│ │  └─ ⠿ Text      [⧉] [🗑]  │          │                                  │        │                          │
│ └─ Footer                    │          │                                  │        │                          │
│    └─ ⠿ Text      [⧉] [🗑]  │          │                                  │        │                          │
│                              │          └──────────────────────────────────┘        │                          │
│                              │                                                      │                          │
│                              │                 [‹]   1 / 3   [›]                    │                          │
│                              │                                                      │                          │
└──────────────────────────────┴──────────────────────────────────────────────────────┴──────────────────────────┘
```

---

## 3. Final Wireframe — Page 2

```text
┌──────────────────────────────┬──────────────────────────────────────────────────────┬──────────────────────────┐
│ ELEMENTS                     │ LIVE PREVIEW                                         │ PROPERTIES               │
├──────────────────────────────┼──────────────────────────────────────────────────────┼──────────────────────────┤
│                              │                                                      │                          │
│ LAYOUT                       │   ┌───────────────────────────────┐                  │ Selected Element         │
│   + Grid                     │   │  −    72%    +    ▣    ↗     │                  │ Table                    │
│   + Divider                  │   └───────────────────────────────┘                  │                          │
│   + Spacer                   │                                                      │ TABLE                    │
│   + Page Break               │          ┌──────────────────────────────────┐        │ Columns    : 4           │
│                              │          │                                  │        │ Rows       : 3           │
│ CONTENT                      │          │   ⠿ Text                         │        │ Width      : Auto        │
│   + Text                     │          │   “Payment Details”              │        │ Height     : Auto        │
│   + Image                    │          │                                  │        │                          │
│   + List                     │          │   ⠿ ──────────────────────────   │        │ SPACING                  │
│   + Table                    │          │                                  │        │ Margin     : 0 px        │
│                              │          │   ⠿ ┌────────────────────────┐   │        │ Padding    : 0 px        │
│ BUSINESS                     │          │     │ TABLE                  │   │        │                          │
│   + Signature                │          │     │                        │   │        │ ALIGNMENT                │
│   + QR Code                  │          │     │ Item  Qty  Price Total │   │        │ Horizontal : Start       │
│   + Barcode                  │          │     │                        │   │        │ Vertical   : Start       │
│                              │          │     └────────────────────────┘   │        │                          │
├──────────────────────────────┤          │                                  │        │                          │
│ LAYER                        │          │                                  │        │                          │
│                              │          │                                  │        │                          │
│ Page 2                 [+]   │          │                                  │        │                          │
│                              │          │                                  │        │                          │
│ ├─ Header                    │          │                                  │        │                          │
│ ├─ Body                      │          │                                  │        │                          │
│ │  ├─ ⠿ Text      [⧉] [🗑]  │          │                                  │        │                          │
│ │  ├─ ⠿ Divider   [⧉] [🗑]  │          │                                  │        │                          │
│ │  └─ ⠿ Table ◀   [⧉] [🗑]  │          │                                  │        │                          │
│ └─ Footer                    │          │                                  │        │                          │
│                              │          └──────────────────────────────────┘        │                          │
│                              │                                                      │                          │
│                              │                 [‹]   2 / 3   [›]                    │                          │
│                              │                                                      │                          │
└──────────────────────────────┴──────────────────────────────────────────────────────┴──────────────────────────┘
```

---

## 4. UX Final

### Navigasi Halaman

- Tombol `[‹]` dan `[›]` memindahkan halaman aktif.
- Angka `1 / 3` atau `2 / 3` menunjukkan halaman aktif.
- Ketika halaman berubah, Live Preview dan Layer ikut berubah.
- Layer hanya menampilkan elemen milik halaman aktif.
- `[+]` menambahkan halaman baru setelah halaman aktif.

### Aksi pada Layer

```text
⠿   Drag untuk sorting
⧉   Duplicate
🗑   Delete
◀   Element aktif
```

Header, Body, dan Footer ditampilkan sebagai region halaman. Grid ditampilkan sebagai parent. Elemen di dalam Grid ditampilkan sebagai child.

```text
Page 1
├─ Header
│  └─ Grid
│     ├─ Image
│     └─ Text
├─ Body
│  ├─ Text
│  └─ Table
└─ Footer
   └─ Text
```

### Hover Grid

```text
Hover Grid
→ Grid highlight
→ item Grid di Layer ikut highlight
```

Saat pointer berada pada Grid di Live Preview:

- Outline Grid ditampilkan.
- Drag handle Grid dapat muncul.
- Item Grid terkait pada Layer ikut diberi hover state.
- Properties tidak berubah karena hover bukan selection.

### Hover Text di dalam Grid

```text
Hover Text di dalam Grid
→ hanya Text tersebut yang highlight
→ child Text terkait di Layer ikut highlight
```

Saat pointer berada pada Text di dalam Grid:

- Hanya Text terdalam yang diberi highlight.
- Grid tidak dianggap sebagai elemen hover utama.
- Child Text yang sesuai di Layer ikut diberi hover state.
- Pengguna tetap dapat memilih Grid melalui area padding Grid atau melalui Layer.

### Klik Element

```text
Klik element
→ element menjadi selected
→ Layer item menjadi active
→ Properties menampilkan konfigurasi element
```

Selection berjalan dua arah:

- Klik elemen di Live Preview akan mengaktifkan item terkait di Layer.
- Klik item di Layer akan mengaktifkan elemen terkait di Live Preview.
- Properties selalu menampilkan konfigurasi elemen yang aktif.
- Selection tetap aktif sampai pengguna memilih elemen lain.

### Properties Panel

Properties ditampilkan sebagai inspector panel yang compact dan dikelompokkan berdasarkan intent, bukan sekadar daftar field mentah.

Data tetap berasal dari metadata backend:

- `document_element_properties` menentukan property apa saja yang tampil untuk element aktif.
- `document_properties` menentukan `code`, `name`, `data_type`, `input_type`, `default_value`, dan `unit`.
- `document_property_options` menyediakan preset seperti select option, color palette, dan grid column cards.

Contoh inspector ketika Grid dipilih:

```text
PROPERTIES
┌──────────────────────────────┐
│ Grid                         │
│ Type: Layout                 │
│ [Duplicate] [Delete]         │
├──────────────────────────────┤
│ Layout                       │
│ Columns                      │
│ ┌───────┐ ┌───────┐          │
│ │Single │ │Half ◀│          │
│ │100    │ │50/50 │          │
│ └───────┘ └───────┘          │
│ ┌───────┐ ┌───────┐          │
│ │Left   │ │Right  │          │
│ │30/70  │ │70/30  │          │
│ └───────┘ └───────┘          │
│ ┌───────┐ ┌───────┐          │
│ │Thirds │ │Custom │          │
│ │33/33/ │ │Manual │          │
│ │34     │ │%      │          │
│ └───────┘ └───────┘          │
│ Gap                          │
│ [ 12                    ] px │
├──────────────────────────────┤
│ Size                         │
│ Width          Height        │
│ [ auto     ]   [ auto     ]  │
├──────────────────────────────┤
│ Spacing                      │
│ Margin         Padding       │
│ [ 0       ] px [ 0       ] px│
├──────────────────────────────┤
│ Appearance                   │
│ Background                   │
│ [Primary] [Secondary]        │
│ [Accent]  [Custom Color]     │
│ Border         Radius        │
│ [ 0       ] px [ 0       ] px│
├──────────────────────────────┤
│ Alignment                    │
│ Horizontal                   │
│ [Start] [Center] [End]       │
│ Vertical                     │
│ [Start] [Center] [End]       │
└──────────────────────────────┘
```

Contoh inspector ketika Text dipilih:

```text
PROPERTIES
┌──────────────────────────────┐
│ Text                         │
│ Type: Content                │
│ [Duplicate] [Delete]         │
├──────────────────────────────┤
│ Content                      │
│ [ Payment Details          ] │
├──────────────────────────────┤
│ Typography                   │
│ Font                         │
│ [ Arial                  v ] │
│ Size           Weight        │
│ [ 16     ] px  [Regular  v ] │
│ Line Height                  │
│ [ 1.5                   ]    │
├──────────────────────────────┤
│ Color                        │
│ Text Color                   │
│ [Primary] [Secondary]        │
│ [Accent]  [Custom Color]     │
├──────────────────────────────┤
│ Alignment                    │
│ [Left] [Center] [Right]      │
│ [Justify]                    │
├──────────────────────────────┤
│ Spacing                      │
│ Margin                       │
│ [ 0                     ] px │
└──────────────────────────────┘
```

Rule rendering input:

```text
input_type = text         → text input
input_type = number       → number input + unit label
input_type = select       → dropdown / segmented button dari options
input_type = color        → color picker + palette options
input_type = textarea     → multiline input
input_type = switch       → boolean toggle
input_type = grid-columns → preset cards + custom percentage input
```

Untuk `grid-columns`, value disimpan sebagai array persen.

```text
Single  → [100]
Half    → [50,50]
Left    → [30,70]
Right   → [70,30]
Thirds  → [33,33,34]
Custom  → user input manual, total maksimal 100%
```

### Drag Element

```text
Drag element
→ element bisa diurutkan
→ posisi Layer ikut berubah
```

Setiap elemen dapat di-drag dari:

- Layer
- Live Preview

Setelah elemen dipindahkan:

- Urutan elemen pada Live Preview berubah.
- Posisi elemen pada Layer ikut berubah.
- Parent-child tetap mengikuti posisi terbaru.
- Elemen yang selesai dipindahkan tetap menjadi selected.

### Sinkronisasi Layer dan Live Preview

Layer dan Live Preview selalu merepresentasikan data yang sama.

```text
Hover Live Preview
→ Layer ikut highlight

Hover Layer
→ Live Preview ikut highlight

Click Live Preview
→ Layer active
→ Properties berubah

Click Layer
→ Live Preview active
→ Properties berubah

Drag Live Preview
→ Layer berubah

Drag Layer
→ Live Preview berubah
```

### Tampilan Drag Handle

Drag handle `⠿` tersedia pada setiap elemen.

Pada implementasi final, drag handle di Live Preview sebaiknya hanya tampil ketika elemen:

- Di-hover
- Sedang selected

Hal ini menjaga Live Preview tetap bersih ketika pengguna tidak sedang berinteraksi dengan elemen.
