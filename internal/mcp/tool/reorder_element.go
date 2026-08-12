package tool

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ReorderElementInput memindahkan satu elemen di dalam tumpukan halamannya.
//
// Index BUKAN pointer dan tidak boleh omitempty: nol adalah tujuan yang sah —
// paling bawah — sehingga ia harus tetap terkirim. Itu pula alasan tool ini
// menerima satu elemen saja, bukan daftar: memindahkan beberapa sekaligus
// membuat setiap index bergeser oleh pemindahan sebelumnya, dan yang dimaksud
// pemanggil menjadi tidak dapat ditebak.
type ReorderElementInput struct {
	DocumentToken string `json:"document_token" jsonschema:"token dokumen desain"`
	ID            string `json:"id" jsonschema:"id elemen yang dipindahkan"`
	Index         int    `json:"index" jsonschema:"letak tujuan di dalam halamannya dihitung dari nol; nol berarti paling bawah"`
}

func registerReorderElement(server *mcp.Server, deps Deps) {
	mcp.AddTool(server, &mcp.Tool{
		Name: "reorder_element",
		Description: "Memindahkan satu elemen di dalam urutan gambar halamannya. " +
			"Urutan itu ADALAH urutan tumpukan: yang belakangan menutupi yang " +
			"terdahulu, jadi index terbesar berarti paling depan dan nol paling " +
			"belakang. Panggil read_document lebih dulu untuk melihat urutan yang " +
			"berlaku sekarang, dan pindahkan satu per satu — setiap pemindahan " +
			"menggeser index elemen lain.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in ReorderElementInput) (*mcp.CallToolResult, EditResult, error) {
		if in.DocumentToken == "" || in.ID == "" {
			return fail("document_token dan id wajib diisi"), EditResult{}, nil
		}
		if in.Index < 0 {
			return fail("index tidak boleh negatif"), EditResult{}, nil
		}

		sesi, err := deps.Sessions.Open(ctx, in.DocumentToken)
		if err != nil {
			return fail(fmt.Sprintf("tidak dapat membuka dokumen: %v", err)), EditResult{}, nil
		}

		pesan := []any{reorderMessage{
			Type:   "element.reorder",
			Origin: sesi.Origin(),
			ID:     in.ID,
			Index:  in.Index,
		}}

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

		return laporkan(hasil, "elemen dipindahkan"), keluaran, nil
	})
}
