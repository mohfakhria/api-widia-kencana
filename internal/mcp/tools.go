package mcp

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// serverName dan serverVersion diumumkan ke klien saat initialize. Klien
// menampilkannya kepada penggunanya, jadi keduanya nama yang dibaca manusia.
const (
	serverName    = "widia-kencana"
	serverVersion = "0.1.0"
)

// WhoAmIInput sengaja kosong.
//
// SDK menuntut tipe masukan berupa struct atau map supaya skema JSON-nya
// bertipe "object", sesuai spesifikasi MCP. Struct kosong menghasilkan objek
// tanpa properti — bentuk yang benar untuk tool tanpa argumen.
type WhoAmIInput struct{}

// newMCPServer menyusun server MCP beserta seluruh tool-nya.
//
// Tool DIDAFTARKAN DI SINI, bukan tersebar, supaya satu berkas menjawab
// "apa saja yang dapat dilakukan agent". Daftar yang tersebar cepat atau lambat
// menumbuhkan tool yang tidak seorang pun ingat pernah membukanya.
func newMCPServer(api *APIClient, sessions *SessionManager) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    serverName,
		Version: serverVersion,
	}, nil)

	// whoami adalah tool paling sederhana yang masih membuktikan sesuatu: ia
	// menembus seluruh rantai — protokol MCP, penjaga token, sesi agent, lalu
	// API — dan mengembalikan identitas yang API sendiri akui.
	//
	// Nilainya bukan pada apa yang ia kerjakan, melainkan pada apa yang
	// kegagalannya beri tahu. Login yang berhasil sebagai akun KELIRU terlihat
	// sama persis dengan yang benar sampai jauh kemudian, ketika sesuatu ditolak
	// barier peran tanpa menyebut sebabnya.
	mcp.AddTool(server, &mcp.Tool{
		Name: "whoami",
		Description: "Menyebutkan identitas agent yang dipakai server ini saat " +
			"menyunting dokumen: id pengguna, nama, dan perannya menurut API.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ WhoAmIInput) (*mcp.CallToolResult, Identity, error) {
		identity, err := api.Me(ctx)
		if err != nil {
			// Galat dikembalikan sebagai galat tool, bukan galat protokol.
			// Bedanya penting bagi klien: yang pertama disampaikan ke model
			// sebagai hasil yang dapat ia tanggapi, yang kedua memutus sesi.
			return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{
					&mcp.TextContent{Text: fmt.Sprintf("tidak dapat menghubungi API sebagai agent: %v", err)},
				},
			}, Identity{}, nil
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf(
					"Agent %s (id %s) dengan peran %s.",
					identity.Name, identity.UserID, identity.Role)},
			},
		}, identity, nil
	})

	// read_document membuka socket ke dokumen lalu mengembalikan isinya.
	//
	// Socketnya DIBIARKAN TERBUKA setelah tool ini selesai — itu bukan
	// kebocoran melainkan rancangannya. Agent tetap tampak sebagai penghuni di
	// kanvas selama ia mengerjakan dokumen itu, dan suntingan berikutnya tidak
	// perlu membayar tiket serta handshake lagi. Sesi yang menganggur ditutup
	// penyapu.
	mcp.AddTool(server, &mcp.Tool{
		Name: "read_document",
		Description: "Membuka dokumen desain lalu mengembalikan isinya beserta " +
			"nomor versi dan ukuran kertas. Wajib dipanggil sebelum menyunting: " +
			"isi dokumen adalah dasar bagi setiap perubahan, dan version yang " +
			"tercatat menjadi acuan bahwa tidak ada siaran yang terlewat.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in ReadDocumentInput) (*mcp.CallToolResult, DocumentState, error) {
		if in.DocumentToken == "" {
			return gagalTool("document_token wajib diisi"), DocumentState{}, nil
		}

		sesi, err := sessions.Session(ctx, in.DocumentToken)
		if err != nil {
			return gagalTool(fmt.Sprintf("tidak dapat membuka dokumen: %v", err)), DocumentState{}, nil
		}

		state, err := sesi.State()
		if err != nil {
			return gagalTool(fmt.Sprintf("sesi dokumen bermasalah: %v", err)), DocumentState{}, nil
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf(
					"Dokumen %s terbuka pada versi %d, kertas %.0f×%.0f pt.",
					state.DocumentToken, state.Version, state.Page.Width, state.Page.Height)},
			},
		}, state, nil
	})

	return server
}

// ReadDocumentInput menyebut dokumen mana yang dibuka.
//
// Token, bukan id angka: itu yang dipakai seluruh permukaan document design,
// dan yang frontend bawa di URL-nya.
type ReadDocumentInput struct {
	DocumentToken string `json:"document_token" jsonschema:"token dokumen desain yang akan dibuka"`
}

// gagalTool membungkus kegagalan sebagai HASIL tool, bukan galat protokol.
//
// Bedanya penting bagi klien: yang pertama sampai ke model sebagai sesuatu yang
// dapat ia tanggapi — memperbaiki argumen, mencoba dokumen lain — sedangkan
// yang kedua memutus sesi dan menyisakan model tanpa keterangan apa pun.
func gagalTool(pesan string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		IsError: true,
		Content: []mcp.Content{&mcp.TextContent{Text: pesan}},
	}
}
