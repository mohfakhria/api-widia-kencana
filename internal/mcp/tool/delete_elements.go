package tool

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// DeleteElementsInput menyebut elemen mana yang dihapus.
type DeleteElementsInput struct {
	DocumentToken string   `json:"document_token" jsonschema:"token dokumen desain"`
	IDs           []string `json:"ids" jsonschema:"id elemen yang dihapus"`
}

func registerDeleteElements(server *mcp.Server, deps Deps) {
	mcp.AddTool(server, &mcp.Tool{
		Name: "delete_elements",
		Description: "Menghapus satu atau beberapa elemen berdasarkan id. " +
			"Setiap elemen menjadi satu langkah undo tersendiri, jadi menghapus " +
			"lima elemen berarti pengguna perlu menekan undo lima kali untuk " +
			"mengembalikannya. Id yang tidak ada di dokumen ditolak server.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in DeleteElementsInput) (*mcp.CallToolResult, EditResult, error) {
		if in.DocumentToken == "" {
			return fail("document_token wajib diisi"), EditResult{}, nil
		}
		if len(in.IDs) == 0 {
			return fail("ids kosong — tidak ada yang dihapus"), EditResult{}, nil
		}
		if tolak := batasi(len(in.IDs), "id"); tolak != nil {
			return tolak, EditResult{}, nil
		}

		sesi, err := deps.Sessions.Open(ctx, in.DocumentToken)
		if err != nil {
			return fail(fmt.Sprintf("tidak dapat membuka dokumen: %v", err)), EditResult{}, nil
		}

		pesan := make([]any, 0, len(in.IDs))
		for _, id := range in.IDs {
			if id == "" {
				return fail("ada id kosong di dalam daftar"), EditResult{}, nil
			}

			pesan = append(pesan, deleteMessage{
				Type:   "element.delete",
				Origin: sesi.Origin(),
				ID:     id,
			})
		}

		hasil, err := kirim(ctx, sesi, pesan)
		keluaran := EditResult{
			DocumentToken: in.DocumentToken,
			Applied:       hasil.Terkirim,
			Confirmed:     hasil.Terkonfirmasi,
			Version:       hasil.Version,
		}

		if err != nil {
			return fail(fmt.Sprintf("pengiriman terhenti setelah %d pesan: %v", hasil.Terkirim, err)), keluaran, nil
		}

		return laporkan(hasil, "elemen dihapus"), keluaran, nil
	})
}
