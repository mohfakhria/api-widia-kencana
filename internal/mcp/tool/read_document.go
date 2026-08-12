package tool

import (
	"context"
	"fmt"

	"github.com/mohfakhria/api-widia-kencana/internal/mcp/session"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ReadDocumentInput menyebut dokumen mana yang dibuka.
//
// Token, bukan id angka: itu yang dipakai seluruh permukaan document design, dan
// yang frontend bawa di URL-nya.
type ReadDocumentInput struct {
	DocumentToken string `json:"document_token" jsonschema:"token dokumen desain yang akan dibuka"`
}

// registerReadDocument memasang tool pembaca dokumen.
//
// Socketnya DIBIARKAN TERBUKA setelah tool ini selesai — itu bukan kebocoran
// melainkan rancangannya. Agent tetap tampak sebagai penghuni di kanvas selama
// ia mengerjakan dokumen itu, dan suntingan berikutnya tidak perlu membayar
// tiket serta handshake lagi. Sesi yang menganggur ditutup penyapu.
func registerReadDocument(server *mcp.Server, deps Deps) {
	mcp.AddTool(server, &mcp.Tool{
		Name: "read_document",
		Description: "Membuka dokumen desain lalu mengembalikan isinya beserta " +
			"nomor versi dan ukuran kertas. Wajib dipanggil sebelum menyunting: " +
			"isi dokumen adalah dasar bagi setiap perubahan, dan version yang " +
			"tercatat menjadi acuan bahwa tidak ada siaran yang terlewat.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in ReadDocumentInput) (*mcp.CallToolResult, session.State, error) {
		if in.DocumentToken == "" {
			return fail("document_token wajib diisi"), session.State{}, nil
		}

		sesi, err := deps.Sessions.Open(ctx, in.DocumentToken)
		if err != nil {
			return fail(fmt.Sprintf("tidak dapat membuka dokumen: %v", err)), session.State{}, nil
		}

		state, err := sesi.State()
		if err != nil {
			return fail(fmt.Sprintf("sesi dokumen bermasalah: %v", err)), session.State{}, nil
		}

		return text(fmt.Sprintf("Dokumen %s terbuka pada versi %d, kertas %.0f×%.0f pt.",
			state.DocumentToken, state.Version, state.Page.Width, state.Page.Height)), state, nil
	})
}
