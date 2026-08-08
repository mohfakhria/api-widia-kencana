/**
 * Kontrak Document Design dalam bentuk yang dapat dieksekusi.
 *
 * Prosa tidak dapat menahan penyimpangan; tipe dapat. Salin atau impor berkas
 * ini di frontend supaya ketidakcocokan menjadi galat kompilasi, bukan kejutan
 * saat berjalan.
 *
 * Penjelasan naratifnya:
 *   - amplop pesan  → docs/engineering/websocket-contract.md
 *   - isi dokumen   → docs/engineering/document-design.md
 *
 * PENTING: berkas ini wajib berubah pada commit yang sama dengan struct Go yang
 * diwakilinya. Aturan lengkapnya ada di CLAUDE.md.
 */

// ---------------------------------------------------------------------------
// Isi dokumen
// ---------------------------------------------------------------------------

/**
 * Seluruh koordinat, ukuran, dan ukuran huruf memakai TITIK (pt), yaitu 1/72
 * inci. Tidak ada satuan yang ditulis di dalam data — angkanya saja.
 *
 * Titik adalah satuan asli PDF sekaligus satuan sah di CSS, jadi frontend cukup
 * menempelkan "pt". Untuk piksel layar: 1pt = 96/72 = 1.333px.
 */
export type Points = number;

/** Heksadesimal saja: `#rgb` atau `#rrggbb`. Nama warna CSS, rgba(), dan hsl() ditolak backend. */
export type DesignColor = `#${string}`;

/** Kelipatan 100, dari 100 sampai 900. Ketebalan yang tidak terdaftar menggagalkan ekspor. */
export type DesignFontWeight = 100 | 200 | 300 | 400 | 500 | 600 | 700 | 800 | 900;

export type DesignFontStyle = 'normal' | 'italic';
export type DesignAlign = 'left' | 'center' | 'right' | 'justify';
/** Sama artinya dengan `object-fit` pada CSS. */
export type DesignImageFit = 'contain' | 'cover' | 'fill';

export interface DesignContent {
  pages: DesignPage[];
}

export interface DesignPage {
  /** Tidak kosong, unik se-dokumen. */
  id: string;
  /** Urutan elemen adalah urutan gambar: yang belakangan menutupi yang terdahulu. */
  elements?: DesignElement[];
}

interface DesignElementBase {
  /** Tidak kosong, unik se-dokumen — termasuk lintas halaman. */
  id: string;
  /** Sudut kiri atas kotak, relatif terhadap sudut kiri atas halaman. */
  x: Points;
  y: Points;
  w: Points;
  h: Points;
}

export interface DesignTextElement extends DesignElementBase {
  type: 'text';
  /** `\n` menjadi pergantian baris; deretan spasi dipadatkan jadi satu. */
  text?: string;
  /** Nama keluarga dari GET /api/document-design-fonts. Bawaan: "helvetica". */
  fontFamily?: string;
  /** Bawaan: 12. */
  fontSize?: Points;
  /** Bawaan: 400. */
  fontWeight?: DesignFontWeight;
  /** Bawaan: "normal". */
  fontStyle?: DesignFontStyle;
  /** Bawaan: "#000000". */
  color?: DesignColor;
  /** Bawaan: "left". */
  align?: DesignAlign;
  /** Pengali ukuran huruf, misal 1.5. Bawaan: 1.2. */
  lineHeight?: number;
  /** Titik; boleh negatif. Bawaan: 0. */
  letterSpacing?: Points;
}

export interface DesignRectElement extends DesignElementBase {
  type: 'rect';
  /** Kosong berarti tanpa isi. */
  fill?: DesignColor;
  /** Kosong berarti tanpa garis tepi. */
  stroke?: DesignColor;
  /** Garis tepi hanya digambar bila > 0. Terpusat pada jalurnya, seperti SVG — bukan seperti `border`. */
  strokeWidth?: Points;
  /** Dibatasi separuh sisi terpendek. */
  radius?: Points;
}

export interface DesignLineElement extends DesignElementBase {
  type: 'line';
  /**
   * Pada garis, `w` dan `h` BUKAN ukuran melainkan simpangan ujung terhadap
   * pangkal, sehingga keduanya boleh negatif. Garis mendatar berarti h: 0.
   */
  stroke?: DesignColor;
  strokeWidth?: Points;
}

export interface DesignImageElement extends DesignElementBase {
  type: 'image';
  /** Token aset yang sudah terunggah — bukan URL. Tampilkan lewat GET /api/asset-presign/:token. */
  assetToken: string;
  /** Bawaan: "contain". */
  fit?: DesignImageFit;
}

export type DesignElement =
  | DesignTextElement
  | DesignRectElement
  | DesignLineElement
  | DesignImageElement;

// ---------------------------------------------------------------------------
// Pesan WebSocket
// ---------------------------------------------------------------------------

export interface DesignDocumentGetMessage {
  type: 'document.get';
}

/**
 * Letak pointer di atas kanvas.
 *
 * Tidak pernah dibalas, bahkan saat ditolak. Batasi lajunya dengan throttle —
 * requestAnimationFrame paling sederhana — BUKAN debounce: debounce menunggu
 * jeda, sehingga kursor baru bergerak ketika mouse berhenti.
 *
 * Pengirim harus sudah menjadi anggota, yaitu sudah mengirim document.get.
 */
export interface DesignCursorMoveMessage {
  type: 'cursor.move';
  /** Id halaman tempat pointer berada. Wajib, tidak boleh kosong. */
  page: string;
  x: Points;
  y: Points;
}

/**
 * Menambahkan elemen ke sebuah halaman.
 *
 * id dibuat frontend dan wajib unik se-dokumen, termasuk lintas halaman — pakai
 * UUID. Elemen baru masuk di akhir daftar halaman itu, yaitu paling atas.
 *
 * Satu-satunya pesan penyuntingan yang membawa page, dan satu-satunya yang dapat
 * dibalas error (element_rejected).
 */
export interface DesignElementCreateMessage {
  type: 'element.create';
  page: string;
  element: DesignElement;
}

/**
 * Mengganti sebuah elemen SELURUHNYA.
 *
 * Bukan patch: field yang tidak disertakan kembali ke nilai bawaannya. Letaknya
 * dalam urutan gambar tidak berubah — yang memindahkan hanya element.reorder.
 *
 * Elemen yang sudah tidak ada didiamkan: tanpa error, tanpa siaran, tanpa
 * kenaikan version. Orang lain yang menghapusnya lebih dulu adalah kejadian
 * biasa, dan element.deleted untuknya sedang menuju Anda.
 *
 * Saat menggeser, throttle ke ~20–30 per detik lalu kirim satu update terakhir
 * yang pasti ketika tombol dilepas. Siaran perubahan tidak boleh hilang, jadi
 * klien yang tertinggal terlalu jauh diputus — bukan dilewati seperti kursor.
 */
export interface DesignElementUpdateMessage {
  type: 'element.update';
  element: DesignElement;
}

/** Membuang elemen. Halaman dicari backend dari id. */
export interface DesignElementDeleteMessage {
  type: 'element.delete';
  id: string;
}

/**
 * Memindahkan elemen di dalam halamannya, karena urutan elemen adalah urutan
 * gambar.
 *
 * index dihitung dari nol; nol berarti paling bawah. Index di luar batas TIDAK
 * ditolak melainkan dijepit ke ujung terdekat — siaran element.reordered membawa
 * letak sesungguhnya, dan angka itulah yang dipakai.
 */
export interface DesignElementReorderMessage {
  type: 'element.reorder';
  id: string;
  index: number;
}

export type DesignClientMessage =
  | DesignDocumentGetMessage
  | DesignCursorMoveMessage
  | DesignElementCreateMessage
  | DesignElementUpdateMessage
  | DesignElementDeleteMessage
  | DesignElementReorderMessage;

export interface DesignSnapshotMessage {
  type: 'snapshot';
  /** Nomor revisi dokumen. Simpan nilainya; jangan bangun rekonsiliasi di atasnya sekarang. */
  version: number;
  /** Ukuran satu halaman dalam titik, berlaku untuk seluruh halaman dokumen ini. */
  page: { width: Points; height: Points };
  content: DesignContent;
}

/**
 * Siapa yang sedang membuka dokumen.
 *
 * Yang didaftar ORANG, bukan koneksi: satu orang dengan beberapa tab tetap satu
 * entri. Tidak ada field jumlah — users.length sudah menjawabnya. Warna avatar
 * diturunkan frontend dari id.
 */
export interface DesignPresenceMessage {
  type: 'presence';
  users: DesignPresenceUser[];
}

export interface DesignPresenceUser {
  id: string;
  name: string;
}

/**
 * Kursor semua orang, satu pesan per denyut 70 ms dan hanya bila ada yang
 * berubah. Juga dikirim langsung kepada orang yang baru bergabung.
 *
 * GANTI seluruh keadaan kursor dengan isi array ini, jangan digabungkan satu per
 * satu. Termasuk kursor penerima sendiri — saring id sendiri saat menggambar.
 *
 * Dikunci per ORANG, bukan per koneksi. Setiap id dijamin ada juga di presence,
 * jadi nama dan warnanya selalu tersedia; warna diturunkan frontend dari id.
 *
 * Satu-satunya pesan yang boleh hilang: bila antrean klien penuh ia dilewati, dan
 * yang baru menggantikan yang masih mengantre.
 */
export interface DesignCursorMessage {
  type: 'cursor';
  cursors: DesignCursor[];
}

export interface DesignCursor {
  id: string;
  /** Id halaman; sembunyikan kursor orang yang sedang melihat halaman lain. */
  page: string;
  x: Points;
  y: Points;
}

/**
 * Siaran satu perubahan yang sudah diterapkan.
 *
 * DITERIMA JUGA OLEH PENGIRIMNYA — berbeda dari cursor, dan disengaja. Tanpa itu
 * nomor version pengirim tertinggal tiap kali ia menyunting, lalu siaran pertama
 * dari orang lain terlihat melompat dan ia memuat ulang tanpa sebab.
 *
 * Terapkan optimistis lebih dulu; siaran yang kembali hanya menaikkan version
 * Anda, sekaligus menjadi tanda bahwa perubahannya sungguh diterima.
 *
 * version adalah nomor SETELAH perubahan ini, dan selalu naik tepat satu. Bila
 * yang datang bukan versi+1, ada yang terlewat — kirim document.get.
 *
 * Tidak pernah dijatuhkan. Klien yang antrean keluarnya penuh diputus tanpa close
 * frame, sehingga terlihat sebagai 1006 dan tidak dapat dibedakan dari jaringan
 * mati; sambung ulang lalu document.get.
 */
export interface DesignElementCreatedMessage {
  type: 'element.created';
  version: number;
  page: string;
  element: DesignElement;
}

export interface DesignElementUpdatedMessage {
  type: 'element.updated';
  version: number;
  /** Elemen utuh. Ganti yang berid sama, jangan pindahkan urutannya. */
  element: DesignElement;
}

export interface DesignElementDeletedMessage {
  type: 'element.deleted';
  version: number;
  id: string;
}

export interface DesignElementReorderedMessage {
  type: 'element.reordered';
  version: number;
  id: string;
  /** Letak SESUNGGUHNYA setelah dijepit, bukan yang diminta. */
  index: number;
}

/** Pesan error TIDAK menutup koneksi. Menyambung ulang bukan tindakan yang tepat. */
export interface DesignErrorMessage {
  type: 'error';
  code: DesignErrorCode;
  message: string;
}

export type DesignErrorCode =
  /** Room tidak dapat melayani document.get. Tunggu belasan detik, minta lagi. */
  | 'document_unavailable'
  /** JSON tidak dapat diurai. Mulai bersih dengan document.get. */
  | 'malformed_message'
  /** Field type kosong atau tidak ada. Bug frontend. */
  | 'missing_message_type'
  /** Jenis pesan belum didukung backend. */
  | 'unsupported_message_type'
  /**
   * Muatan penyuntingan ditolak: elemen tidak sah, properti tak dikenal, halaman
   * tidak ada, atau id sudah dipakai. Batalkan perubahan optimistik Anda —
   * hampir selalu bug frontend, bukan keadaan yang dapat dicoba lagi.
   */
  | 'element_rejected';

export type DesignServerMessage =
  | DesignSnapshotMessage
  | DesignPresenceMessage
  | DesignCursorMessage
  | DesignElementCreatedMessage
  | DesignElementUpdatedMessage
  | DesignElementDeletedMessage
  | DesignElementReorderedMessage
  | DesignErrorMessage;

/**
 * Close code yang dikirim server.
 *
 * Sengaja tipe, bukan objek konstanta: berkas ini hanya mendeklarasikan tipe dan
 * tidak menghasilkan JavaScript apa pun, sehingga nilai yang diimpor darinya
 * tidak akan ada saat berjalan. Bila frontend ingin nama untuk angka-angka ini,
 * deklarasikan konstantanya sendiri lalu ketikkan dengan tipe di bawah.
 *
 * 1006 tidak ada di sini karena ia tidak pernah dikirim siapa pun — browser
 * melaporkannya ketika socket tertutup tanpa close frame, baik karena handshake
 * ditolak maupun karena jaringan putus. Bedakan keduanya dengan mencatat apakah
 * `onopen` pernah menyala.
 */
export type DesignCloseCode =
  /** Penutupan normal. */
  | 1000
  /** Ada frame biner terkirim. Bug frontend. */
  | 1003
  /** Tiket tidak sah atau kedaluwarsa. Terbitkan tiket baru, sambung ulang. */
  | 1008
  /** Room berhenti melayani di tengah sesi. Jangan sambung ulang otomatis. */
  | 1011
  /** Sudah 10 koneksi untuk user ini. Tunggu, lalu coba lagi. */
  | 1013;

// ---------------------------------------------------------------------------
// Endpoint HTTP
// ---------------------------------------------------------------------------

/** POST /api/document-design-ticket/:token */
export interface DesignTicketResponse {
  status: 'ok';
  message: string;
  data: {
    design_ticket: {
      ticket: string;
      /** Detik. Tiket sekali pakai dan berumur 30 detik. */
      expires_in: number;
    };
  };
}

/** GET /api/document-design-fonts */
export interface DesignFontsResponse {
  status: 'ok';
  message: string;
  data: {
    fonts: DesignFontFamily[];
  };
}

export interface DesignFontFamily {
  name: string;
  faces: DesignFontFace[];
}

export interface DesignFontFace {
  weight: DesignFontWeight;
  style: DesignFontStyle;
}

/**
 * POST /api/document-export/:token
 *
 * Balasan berhasil BUKAN JSON melainkan berkas PDF mentah
 * (Content-Type: application/pdf). Hanya kegagalan yang memakai amplop JSON.
 */
export interface DesignErrorResponse {
  status: 'error';
  message: string;
}
