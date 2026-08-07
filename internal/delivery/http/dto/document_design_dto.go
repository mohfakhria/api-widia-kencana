package dto

import (
	"encoding/json"

	"github.com/mohfakhria/api-widia-kencana/internal/domain/design"
)

// Jenis pesan yang dikirim klien.
const (
	DesignMessageDocumentGet = "document.get"
	DesignMessageCursorMove  = "cursor.move"
)

// Jenis pesan yang dikirim server.
const (
	DesignMessageSnapshot = "snapshot"
	DesignMessagePresence = "presence"
	DesignMessageCursor   = "cursor"
	DesignMessageError    = "error"
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
