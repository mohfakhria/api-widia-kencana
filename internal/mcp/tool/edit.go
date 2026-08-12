package tool

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/mohfakhria/api-widia-kencana/internal/domain/design"
	"github.com/mohfakhria/api-widia-kencana/internal/mcp/session"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Bentuk pesan sunting di kawat.
//
// Diketik, bukan disusun sebagai map. Elemennya memakai design.Element
// LANGSUNG — tipe yang sama dengan yang divalidasi server — sehingga field yang
// tidak ada di kontrak tidak dapat dikirim tanpa gagal kompilasi. Itu penting
// karena server memakai DisallowUnknownFields: satu nama field yang meleset
// menolak SELURUH pesan, bukan mengabaikan fieldnya.
type (
	createMessage struct {
		Type    string         `json:"type"`
		Origin  string         `json:"origin"`
		Page    string         `json:"page"`
		Element design.Element `json:"element"`
	}

	updateMessage struct {
		Type    string         `json:"type"`
		Origin  string         `json:"origin"`
		Element design.Element `json:"element"`
	}

	deleteMessage struct {
		Type   string `json:"type"`
		Origin string `json:"origin"`
		ID     string `json:"id"`
	}

	reorderMessage struct {
		Type   string `json:"type"`
		Origin string `json:"origin"`
		ID     string `json:"id"`
		Index  int    `json:"index"`
	}
)

// Tenggat dan selang menunggu konfirmasi suntingan.
//
// settleTimeout longgar dengan sengaja: yang ditunggu adalah perjalanan
// pulang-pergi ke server produksi, dan menyerah terlalu cepat menghasilkan
// laporan "tidak terkonfirmasi" untuk suntingan yang sebenarnya mendarat —
// kabar yang lebih menyesatkan daripada menunggu sebentar lebih lama.
const (
	settleTimeout = 3 * time.Second
	settleTick    = 25 * time.Millisecond
)

// maxBatch membatasi banyaknya suntingan dalam satu pemanggilan tool.
//
// Angkanya bukan selera. Setiap suntingan disiarkan ke SETIAP anggota room, dan
// antrean keluar tiap anggota berbatas designQueueLimit — 256 pada saat ini.
// Antrean yang melimpah TIDAK membuang pesannya melainkan MEMUTUS anggota itu,
// jadi satu pemanggilan yang mengirim ratusan elemen sekaligus dapat menendang
// penyunting manusia yang sedang tersendat keluar dari dokumennya sendiri —
// tanpa agent maupun modelnya pernah tahu itu terjadi.
//
// Seratus menyisakan ruang bagi lalu lintas lain di antrean yang sama. Ini
// bukan pengganti batas laju di sisi server, yang belum ada: ia hanya menutup
// jalur paling mudah menuju kejadian itu.
const maxBatch = 100

// batasi menolak permintaan yang terlalu besar untuk satu pemanggilan.
//
// Mengembalikan nil bila ukurannya wajar, supaya pemanggilnya terbaca sebagai
// satu penjagaan biasa.
func batasi(n int, satuan string) *mcp.CallToolResult {
	if n <= maxBatch {
		return nil
	}

	return fail(fmt.Sprintf(
		"%d %s dalam satu panggilan melampaui batas %d. Pecah menjadi beberapa "+
			"panggilan berurutan — kiriman sebesar itu dapat memutus penyunting "+
			"lain yang sedang membuka dokumen ini.", n, satuan, maxBatch))
}

// hasilKirim adalah apa yang benar-benar diketahui setelah pengiriman.
//
// Terkirim dan Terkonfirmasi dipisah karena keduanya memang beda pertanyaan:
// yang pertama tentang socket, yang kedua tentang dokumen. Menggabungkannya
// membuat "pesan keluar" terbaca sebagai "dokumen berubah", dan itu justru
// kekeliruan yang paling mahal di sini.
type hasilKirim struct {
	Terkirim      int
	Terkonfirmasi bool
	Galat         []string
	Version       int64
}

// kirim mengirim sekumpulan pesan lalu menunggu buktinya.
//
// Berhenti pada kegagalan PENGIRIMAN pertama — socket yang putus tidak akan
// membaik pada pesan berikutnya, dan meneruskannya hanya menghasilkan dokumen
// yang setengah tersunting tanpa ada yang tahu sampai mana.
//
// Buktinya adalah VERSION, bukan ketiadaan galat. Kontraknya menyebut hanya
// element.create yang membalas saat ditolak; update, delete, dan reorder yang
// sasarannya sudah lenyap DIDIAMKAN. Menunggu galat saja karenanya akan selalu
// menjawab "berhasil" untuk ketiganya. Yang naik tanpa kecuali dari kedelapan
// pesan sunting adalah version, jadi version yang bertambah sebanyak pesan yang
// dikirim adalah satu-satunya bukti positif yang tersedia di sisi ini.
func kirim(ctx context.Context, sesi *session.Document, pesan []any) (hasilKirim, error) {
	// Dibuang lebih dulu. Penampungnya bersama untuk seluruh sesi, dan penolakan
	// yang masih tersisa dari pemanggilan sebelumnya akan terbaca sebagai
	// kegagalan suntingan INI.
	sesi.TakeErrors()

	awal := sesi.Version()

	for i, p := range pesan {
		if err := sesi.Send(ctx, p); err != nil {
			return hasilKirim{Terkirim: i, Galat: sesi.TakeErrors(), Version: sesi.Version()}, err
		}
	}

	// Selisihnya, bukan nilai mutlaknya. Penyunting lain yang bekerja bersamaan
	// hanya membuat version naik LEBIH cepat, tidak pernah lebih lambat, jadi
	// ambang ini aman terhadap keramaian — sekadar bisa terlampaui lebih awal.
	target := awal + int64(len(pesan))
	tenggat := time.Now().Add(settleTimeout)

	for {
		sekarang := sesi.Version()
		galat := sesi.TakeErrors()

		// Galat menutup penantian. Setelah ada penolakan, version tidak akan
		// pernah mencapai target, dan menunggu sampai tenggat hanya menahan
		// jawaban tanpa menambah satu keterangan pun.
		if len(galat) > 0 {
			return hasilKirim{Terkirim: len(pesan), Galat: galat, Version: sekarang}, nil
		}

		if sekarang >= target {
			return hasilKirim{Terkirim: len(pesan), Terkonfirmasi: true, Version: sekarang}, nil
		}

		if time.Now().After(tenggat) {
			return hasilKirim{Terkirim: len(pesan), Version: sekarang}, nil
		}

		select {
		case <-time.After(settleTick):
		case <-ctx.Done():
			return hasilKirim{Terkirim: len(pesan), Version: sekarang}, nil
		}
	}
}

// laporkan menyusun jawaban tool dari hasil pengiriman.
//
// Penolakan dilaporkan sebagai KEGAGALAN tool walau pesannya terkirim. Bagi
// model, "terkirim" tanpa "diterapkan" adalah kabar yang menyesatkan: ia akan
// melanjutkan seolah dokumen sudah berubah.
func laporkan(hasil hasilKirim, kegiatan string) *mcp.CallToolResult {
	if len(hasil.Galat) > 0 {
		return fail(fmt.Sprintf(
			"%d %s terkirim, tetapi server menolak: %s. Dokumen sekarang di versi %d — "+
				"panggil read_document untuk melihat keadaan yang sebenarnya berlaku.",
			hasil.Terkirim, kegiatan, strings.Join(hasil.Galat, "; "), hasil.Version))
	}

	// Bukan kegagalan, dan bukan pula keberhasilan. Yang diketahui hanya bahwa
	// buktinya belum datang — bisa karena sasarannya sudah lenyap (yang
	// didiamkan server), bisa karena siarannya sekadar lambat.
	if !hasil.Terkonfirmasi {
		return fail(fmt.Sprintf(
			"%d %s terkirim, tetapi versi dokumen belum bertambah sebanyak itu dalam %s. "+
				"Untuk update, delete, dan reorder, server MENDIAMKAN sasaran yang sudah "+
				"tidak ada — jadi kemungkinan besar id-nya keliru. Dokumen di versi %d; "+
				"panggil read_document sebelum mencoba lagi, jangan diulang begitu saja.",
			hasil.Terkirim, kegiatan, settleTimeout, hasil.Version))
	}

	return text(fmt.Sprintf("%d %s diterapkan. Dokumen sekarang di versi %d.",
		hasil.Terkirim, kegiatan, hasil.Version))
}
