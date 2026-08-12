// Package tool memuat seluruh yang dapat dilakukan agent lewat MCP.
//
// Satu berkas per tool, dan Register di bawah adalah SATU-SATUNYA tempat
// daftarnya disusun. Itu disengaja: pertanyaan "apa saja yang boleh dikerjakan
// agent" harus punya satu jawaban yang dapat dibaca sekali lihat. Pendaftaran
// yang tersebar cepat atau lambat menumbuhkan tool yang tidak seorang pun ingat
// pernah membukanya.
//
// Paket ini terpisah dari induknya bukan demi kerapian melainkan karena arah
// impor: tool membutuhkan klien API dan pengelola sesi, sedangkan induknya
// membutuhkan tool untuk mendaftarkannya. Menaruh semuanya di satu paket
// menutup jalan itu; menurunkan yang dipakai bersama ke apiclient dan session
// membukanya tanpa siklus.
package tool

import (
	"fmt"

	"github.com/mohfakhria/api-widia-kencana/internal/mcp/apiclient"
	"github.com/mohfakhria/api-widia-kencana/internal/mcp/session"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Deps adalah segala yang dibutuhkan tool untuk bekerja.
//
// Dibungkus struct, bukan diteruskan sebagai deretan parameter, karena
// daftarnya akan bertambah: penolong tata letak, batas laju, apa pun yang
// menyusul. Menambah field tidak menyentuh satu pun tanda tangan yang sudah ada.
type Deps struct {
	API      *apiclient.Client
	Sessions *session.Manager
}

// Register memasang seluruh tool pada server MCP.
//
// Mengembalikan galat karena sebagian skema disusun, bukan sekadar diturunkan
// dari tipe, dan penyusunan itu dapat meleset bila bentuk masukannya berubah.
// Galatnya menjatuhkan server saat start — dan itu yang dikehendaki: tool
// dengan skema yang diam-diam tidak lengkap akan menyesatkan setiap model yang
// memakainya, jauh lebih lama sebelum ada yang menyadarinya.
func Register(server *mcp.Server, deps Deps) error {
	registerWhoAmI(server, deps)
	registerReadDocument(server, deps)

	if err := registerCreateElements(server, deps); err != nil {
		return fmt.Errorf("create_elements: %w", err)
	}

	if err := registerUpdateElements(server, deps); err != nil {
		return fmt.Errorf("update_elements: %w", err)
	}

	registerDeleteElements(server, deps)
	registerReorderElement(server, deps)

	return nil
}

// fail membungkus kegagalan sebagai HASIL tool, bukan galat protokol.
//
// Bedanya penting bagi klien: yang pertama sampai ke model sebagai sesuatu yang
// dapat ia tanggapi — memperbaiki argumen, mencoba dokumen lain — sedangkan yang
// kedua memutus sesi dan menyisakan model tanpa keterangan apa pun.
func fail(pesan string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		IsError: true,
		Content: []mcp.Content{&mcp.TextContent{Text: pesan}},
	}
}

func text(pesan string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: pesan}},
	}
}
