package dto

import (
	"encoding/json"

	"github.com/mohfakhria/api-widia-kencana/internal/domain/design"
)

// Jenis pesan yang dikirim klien.
const (
	DesignMessageDocumentGet    = "document.get"
	DesignMessageCursorMove     = "cursor.move"
	DesignMessageElementCreate  = "element.create"
	DesignMessageElementUpdate  = "element.update"
	DesignMessageElementDelete  = "element.delete"
	DesignMessageElementReorder = "element.reorder"
	DesignMessagePageCreate     = "page.create"
	DesignMessagePageUpdate     = "page.update"
	DesignMessagePageDelete     = "page.delete"
	DesignMessagePageReorder    = "page.reorder"
)

// Jenis pesan yang dikirim server.
//
// Perubahan disiarkan dalam bentuk lampau — created, bukan create — supaya
// perintah dan pemberitahuan tidak pernah tertukar. Keduanya membawa muatan yang
// mirip, dan tanpa pembeda pada field type, catatan log maupun union tipe di
// frontend akan menyatukan dua hal yang arahnya berlawanan.
const (
	DesignMessageSnapshot         = "snapshot"
	DesignMessagePresence         = "presence"
	DesignMessageCursor           = "cursor"
	DesignMessageError            = "error"
	DesignMessageElementCreated   = "element.created"
	DesignMessageElementUpdated   = "element.updated"
	DesignMessageElementDeleted   = "element.deleted"
	DesignMessageElementReordered = "element.reordered"
	DesignMessagePageCreated      = "page.created"
	DesignMessagePageUpdated      = "page.updated"
	DesignMessagePageDeleted      = "page.deleted"
	DesignMessagePageReordered    = "page.reordered"
)

type DocumentDesignTicketResponse struct {
	DesignTicket DesignTicketPayload `json:"design_ticket"`
}

type DesignTicketPayload struct {
	Ticket    string `json:"ticket"`
	ExpiresIn int64  `json:"expires_in"`
}

func NewDocumentDesignTicketResponse(ticket string, expiresIn int64) DocumentDesignTicketResponse {
	return DocumentDesignTicketResponse{
		DesignTicket: DesignTicketPayload{
			Ticket:    ticket,
			ExpiresIn: expiresIn,
		},
	}
}

// DesignInbound hanya membaca amplop pesan. Isi spesifik tiap jenis perintah
// baru diurai ketika penerapan perubahan dibangun.
//
// Tidak ada nomor urut. Korelasi permintaan-balasan digantikan version: klien
// membandingkan versi yang diterima dengan versi lokalnya untuk tahu apakah ia
// sudah selaras, tertinggal, atau melewatkan sesuatu.
type DesignInbound struct {
	Type string `json:"type"`
}

// DesignCursorMove adalah muatan pesan cursor.move.
//
// Koordinat memakai titik, sama seperti seluruh isi dokumen, sehingga letaknya
// dapat dipakai langsung tanpa konversi. Page disertakan karena satu dokumen
// dapat berisi banyak halaman — tanpanya penerima tidak tahu di halaman mana
// kursor itu harus digambar.
type DesignCursorMove struct {
	Page string  `json:"page"`
	X    float64 `json:"x"`
	Y    float64 `json:"y"`
}

// DesignCursorMessage adalah letak kursor semua orang sekaligus.
//
// Sengaja berisi seluruh daftar, bukan satu kursor yang baru bergerak. Satu
// muatan dapat dipakai bersama oleh semua penerima, dan penerima cukup mengganti
// seluruh keadaan kursornya alih-alih menggabungkan perubahan satu per satu.
//
// Klien mengabaikan id-nya sendiri. Menyaringnya di server berarti menyusun
// muatan berbeda untuk tiap penerima, dan itu mengalikan pekerjaan penyandian
// sebanyak jumlah penghuni.
type DesignCursorMessage struct {
	Type    string              `json:"type"`
	Cursors []DesignCursorEntry `json:"cursors"`
}

type DesignCursorEntry struct {
	ID   string  `json:"id"`
	Page string  `json:"page"`
	X    float64 `json:"x"`
	Y    float64 `json:"y"`
}

func NewDesignCursorMessage(cursors []DesignCursorEntry) ([]byte, error) {
	if cursors == nil {
		// Array kosong, bukan null — supaya frontend tidak perlu memeriksanya
		// sebelum memetakan.
		cursors = []DesignCursorEntry{}
	}

	return json.Marshal(DesignCursorMessage{
		Type:    DesignMessageCursor,
		Cursors: cursors,
	})
}

// DesignElementCreate adalah muatan pesan element.create.
//
// Page wajib: elemen baru harus tahu ke halaman mana ia dipasang, dan tidak ada
// tempat lain yang menyimpan keterangan itu.
//
// Element dibiarkan mentah dan tidak diurai di sini. Bentuk elemen beserta
// aturannya dimiliki domain/design, yang menolak field asing lewat
// DisallowUnknownFields — mengurainya di lapisan ini akan menduplikasi aturan itu
// di tempat yang tidak berwenang, dan duplikatnya akan melenceng.
type DesignElementCreate struct {
	Page    string          `json:"page"`
	Element json.RawMessage `json:"element"`
}

// DesignElementUpdate adalah muatan pesan element.update.
//
// Elemen dikirim UTUH, bukan hanya field yang berubah. Menang-terakhir jadi
// harfiah — elemen berid sama diganti apa adanya — dan patch per field akan
// memaksa seluruh field menjadi pointer hanya untuk membedakan "tidak dikirim"
// dari "dikirim bernilai nol".
//
// Tidak ada field page. Id elemen unik se-dokumen termasuk lintas halaman, jadi
// halaman dapat dicari dari id-nya. Menyertakan page justru membuka satu keadaan
// yang mustahil ditangani dengan benar: klien menyebut halaman yang bukan tempat
// elemen itu berada.
type DesignElementUpdate struct {
	Element json.RawMessage `json:"element"`
}

// DesignElementDelete adalah muatan pesan element.delete.
//
// Cukup id, dengan alasan yang sama seperti update.
type DesignElementDelete struct {
	ID string `json:"id"`
}

// DesignElementReorder adalah muatan pesan element.reorder.
//
// Urutan elemen di dalam sebuah halaman ADALAH urutan gambar: yang belakangan
// menutupi yang terdahulu. Karena element.update hanya mengganti elemen di
// tempatnya dan tidak memindahkannya, tanpa pesan ini elemen yang tertutup tidak
// punya cara apa pun untuk dinaikkan.
//
// Index adalah letak tujuan di dalam halaman elemen itu sendiri, dihitung dari
// nol. Nol berarti paling bawah. Halaman tidak perlu disebut, sama seperti update
// dan delete — id sudah menentukannya.
type DesignElementReorder struct {
	ID    string `json:"id"`
	Index int    `json:"index"`
}

// DesignElementReorderMessage adalah siaran satu pemindahan urutan.
//
// Sengaja TIDAK memakai DesignElementMessage. Index bertipe int, dan `omitempty`
// pada int membuang nilai nol — sehingga "pindahkan ke paling bawah" akan
// terkirim tanpa field index sama sekali dan terbaca frontend sebagai nilai
// bawaan yang kebetulan juga nol. Kebetulan itu bekerja sampai suatu hari tidak,
// dan struct tersendiri menutupnya tanpa perlu pointer.
//
// Index yang disiarkan adalah letak SESUNGGUHNYA setelah diterapkan, bukan yang
// diminta. Permintaan yang melewati batas dijepit ke ujung terdekat, dan klien
// perlu tahu ke mana elemennya benar-benar mendarat.
type DesignElementReorderMessage struct {
	Type    string `json:"type"`
	Version int64  `json:"version"`
	ID      string `json:"id"`
	Index   int    `json:"index"`
}

func NewDesignElementReorderedMessage(version int64, id string, index int) ([]byte, error) {
	return json.Marshal(DesignElementReorderMessage{
		Type:    DesignMessageElementReordered,
		Version: version,
		ID:      id,
		Index:   index,
	})
}

// DesignElementMessage adalah siaran satu perubahan elemen.
//
// Satu bentuk untuk ketiga jenis siaran, karena yang membedakannya hanya field
// mana yang terisi:
//
//	element.created  → Page dan Element
//	element.updated  → Element
//	element.deleted  → ID
//
// Version SELALU terisi, dan itu bagian terpenting dari pesan ini. Tiap perubahan
// yang diterapkan menaikkannya satu, sehingga klien yang menerima nomor bukan
// versi+1 tahu ada siaran yang terlewat dan cukup meminta document.get. Ini bukan
// rekonsiliasi — hanya cara klien mengetahui bahwa ia perlu memuat ulang.
// Element bertipe domain di sini, bukan json.RawMessage seperti pada muatan
// masuk. Asimetri itu disengaja: yang masuk berasal dari klien dan tidak boleh
// diurai oleh lapisan ini, sedangkan yang keluar sudah tervalidasi dan dimiliki
// orchestrator. Menyandikannya lebih dulu menjadi byte hanya untuk disandikan
// ulang ke dalam amplop adalah pekerjaan ganda tanpa jaminan tambahan.
//
// Pointer supaya omitempty bekerja: elemen kosong pada siaran penghapusan harus
// benar-benar hilang dari JSON, bukan muncul sebagai objek berisi nilai nol.
type DesignElementMessage struct {
	Type    string          `json:"type"`
	Version int64           `json:"version"`
	Page    string          `json:"page,omitempty"`
	ID      string          `json:"id,omitempty"`
	Element *design.Element `json:"element,omitempty"`
}

func NewDesignElementCreatedMessage(version int64, page string, element design.Element) ([]byte, error) {
	return json.Marshal(DesignElementMessage{
		Type:    DesignMessageElementCreated,
		Version: version,
		Page:    page,
		Element: &element,
	})
}

func NewDesignElementUpdatedMessage(version int64, element design.Element) ([]byte, error) {
	return json.Marshal(DesignElementMessage{
		Type:    DesignMessageElementUpdated,
		Version: version,
		Element: &element,
	})
}

func NewDesignElementDeletedMessage(version int64, id string) ([]byte, error) {
	return json.Marshal(DesignElementMessage{
		Type:    DesignMessageElementDeleted,
		Version: version,
		ID:      id,
	})
}

// DesignPageCreate adalah muatan pesan page.create.
//
// Index BERTIPE POINTER, dan itu bukan kerapian. Tanpa pointer, pesan yang tidak
// menyebut index diurai Go menjadi nol — yaitu "sisipkan paling depan", persis
// kebalikan dari yang dimaksud pengirimnya, yang hampir selalu "di akhir".
// Keduanya harus dapat dibedakan, dan hanya pointer yang membedakannya.
//
// Halaman baru selalu KOSONG. Tidak ada penyalinan halaman untuk sekarang.
type DesignPageCreate struct {
	ID    string `json:"id"`
	Index *int   `json:"index"`
}

// DesignPageUpdate adalah muatan pesan page.update — properti halaman saja.
//
// TIDAK ADA field elements, dan itu perbedaan pokok dari element.update. Elemen
// adalah daun sehingga dikirim utuh; halaman memuat elemen, dan mengirim halaman
// utuh berarti tiap perubahan hidden ikut menimpa seluruh isinya — dua orang yang
// menyunting elemen di halaman itu akan saling menghapus pekerjaan.
//
// Kedua boolean BERTIPE POINTER supaya "tidak dikirim" dapat dibedakan dari
// "dikirim bernilai false", lalu ditolak. Tanpa itu, klien yang lupa menyertakan
// locked akan MEMBUKA KUNCI halaman diam-diam dan menyiarkannya ke semua orang
// sebagai perubahan yang sah. Dua field, dua kesempatan untuk lupa; lebih baik
// gagal berisik daripada mengubah sesuatu yang tidak diminta siapa pun.
type DesignPageUpdate struct {
	ID     string `json:"id"`
	Hidden *bool  `json:"hidden"`
	Locked *bool  `json:"locked"`
}

// DesignPageUpdatedMessage adalah siarannya.
//
// Tanpa omitempty pada kedua boolean: siaran yang mengatakan "sekarang TIDAK
// tersembunyi" wajib membawa "hidden": false. Dengan omitempty ia terkirim tanpa
// field itu sama sekali, dan penerima tidak dapat membedakannya dari pesan yang
// memang tidak menyebut apa-apa.
type DesignPageUpdatedMessage struct {
	Type    string `json:"type"`
	Version int64  `json:"version"`
	ID      string `json:"id"`
	Hidden  bool   `json:"hidden"`
	Locked  bool   `json:"locked"`
}

func NewDesignPageUpdatedMessage(version int64, id string, hidden, locked bool) ([]byte, error) {
	return json.Marshal(DesignPageUpdatedMessage{
		Type:    DesignMessagePageUpdated,
		Version: version,
		ID:      id,
		Hidden:  hidden,
		Locked:  locked,
	})
}

// DesignPageDelete adalah muatan pesan page.delete.
//
// Halaman terakhir tidak dapat dihapus; permintaannya dibalas error. Dokumen
// tanpa halaman akan dianggap kosong saat dimuat berikutnya lalu ditimpa panduan
// bawaan, dan itu terjadi bukan saat penghapusannya melainkan ketika orang
// berikutnya membuka dokumen.
type DesignPageDelete struct {
	ID string `json:"id"`
}

// DesignPageReorder adalah muatan pesan page.reorder. Index di luar batas
// dijepit, sama seperti element.reorder.
type DesignPageReorder struct {
	ID    string `json:"id"`
	Index int    `json:"index"`
}

// Siaran halaman: satu struct per pesan, walau dua di antaranya sebentuk.
//
// Tidak digabung menjadi satu bentuk seperti DesignElementMessage. Bentuk yang
// dipakai bersama menuntut omitempty pada field yang tidak selalu terpakai, dan
// omitempty pada int membuang nilai nol — sedangkan halaman di posisi nol adalah
// keadaan yang sah dan sering. Nama yang menyebut satu pesan juga menghapus
// pertanyaan "yang mana saja yang dicakupnya".
//
// Index pada kedua siaran di bawah adalah letak SESUNGGUHNYA setelah penjepitan,
// bukan yang diminta.

type DesignPageCreatedMessage struct {
	Type    string `json:"type"`
	Version int64  `json:"version"`
	ID      string `json:"id"`
	Index   int    `json:"index"`
}

type DesignPageReorderedMessage struct {
	Type    string `json:"type"`
	Version int64  `json:"version"`
	ID      string `json:"id"`
	Index   int    `json:"index"`
}

// DesignPageDeletedMessage tidak membawa letak — halaman yang dibuang tidak
// punya letak lagi.
//
// Elemen di atasnya tidak disebutkan satu per satu: klien sudah tahu isi halaman
// itu dari snapshot dan siaran sebelumnya, jadi menyebutkannya ulang hanya
// menggemukkan pesan tanpa menambah keterangan. Buang halamannya, dan seluruh
// elemennya ikut.
type DesignPageDeletedMessage struct {
	Type    string `json:"type"`
	Version int64  `json:"version"`
	ID      string `json:"id"`
}

func NewDesignPageCreatedMessage(version int64, id string, index int) ([]byte, error) {
	return json.Marshal(DesignPageCreatedMessage{
		Type:    DesignMessagePageCreated,
		Version: version,
		ID:      id,
		Index:   index,
	})
}

func NewDesignPageReorderedMessage(version int64, id string, index int) ([]byte, error) {
	return json.Marshal(DesignPageReorderedMessage{
		Type:    DesignMessagePageReordered,
		Version: version,
		ID:      id,
		Index:   index,
	})
}

func NewDesignPageDeletedMessage(version int64, id string) ([]byte, error) {
	return json.Marshal(DesignPageDeletedMessage{
		Type:    DesignMessagePageDeleted,
		Version: version,
		ID:      id,
	})
}

type DesignSnapshotMessage struct {
	Type    string          `json:"type"`
	Version int64           `json:"version"`
	Page    DesignPageSize  `json:"page"`
	Content json.RawMessage `json:"content"`
}

// DesignPageSize adalah ukuran satu halaman dalam titik.
//
// Ikut dikirim bersama snapshot supaya frontend punya semua yang dibutuhkan untuk
// menggambar dalam satu pesan. Isi dokumen sengaja tidak memuat ukuran halaman,
// dan endpoint detail dokumen mengembalikannya dalam satuan asli kertas —
// milimeter, inci — sehingga tanpa ini frontend harus mengonversi sendiri.
type DesignPageSize struct {
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

// DesignPresenceMessage menyebut siapa saja yang sedang membuka dokumen ini.
//
// Tidak ada field jumlah tersendiri: panjang Users sudah menjawabnya. Field yang
// dapat diturunkan dari field lain cepat atau lambat akan melenceng dari
// sumbernya.
//
// Yang didaftar orang, bukan koneksi — satu orang dengan beberapa tab tetap satu
// entri. Warna avatar sengaja tidak ikut; frontend menurunkannya sendiri dari id
// supaya orang yang sama berwarna sama di semua layar tanpa backend perlu
// menyimpan apa pun.
type DesignPresenceMessage struct {
	Type  string               `json:"type"`
	Users []DesignPresenceUser `json:"users"`
}

type DesignPresenceUser struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func NewDesignPresenceMessage(users []DesignPresenceUser) ([]byte, error) {
	if users == nil {
		// Array kosong, bukan null. Frontend memetakannya langsung ke daftar
		// avatar, dan null memaksa setiap pemakainya memeriksa lebih dulu.
		users = []DesignPresenceUser{}
	}

	return json.Marshal(DesignPresenceMessage{
		Type:  DesignMessagePresence,
		Users: users,
	})
}

type DesignErrorMessage struct {
	Type    string `json:"type"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

func NewDesignSnapshotMessage(content json.RawMessage, version int64, width, height float64) ([]byte, error) {
	return json.Marshal(DesignSnapshotMessage{
		Type:    DesignMessageSnapshot,
		Version: version,
		Page:    DesignPageSize{Width: width, Height: height},
		Content: content,
	})
}

// DocumentDesignFontsResponse menyebut font yang benar-benar dapat dipakai.
//
// Editor membutuhkannya untuk mengisi pilihan font. Tanpa daftar ini frontend
// hanya dapat menebak, dan tebakan yang salah baru ketahuan saat ekspor gagal.
type DocumentDesignFontsResponse struct {
	Fonts []DesignFontFamily `json:"fonts"`
}

type DesignFontFamily struct {
	Name  string           `json:"name"`
	Faces []DesignFontFace `json:"faces"`
}

type DesignFontFace struct {
	Weight int    `json:"weight"`
	Style  string `json:"style"`
}

func NewDocumentDesignFontsResponse(families []design.FontFamily) DocumentDesignFontsResponse {
	response := DocumentDesignFontsResponse{Fonts: make([]DesignFontFamily, 0, len(families))}

	for _, family := range families {
		faces := make([]DesignFontFace, 0, len(family.Faces))
		for _, face := range family.Faces {
			faces = append(faces, DesignFontFace{Weight: face.Weight, Style: face.Style})
		}
		response.Fonts = append(response.Fonts, DesignFontFamily{Name: family.Name, Faces: faces})
	}

	return response
}

func NewDesignErrorMessage(code, message string) ([]byte, error) {
	return json.Marshal(DesignErrorMessage{
		Type:    DesignMessageError,
		Code:    code,
		Message: message,
	})
}
