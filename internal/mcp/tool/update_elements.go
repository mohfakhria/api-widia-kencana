package tool

import (
	"context"
	"fmt"

	"github.com/mohfakhria/api-widia-kencana/internal/domain/design"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// UpdateElementsInput mengganti elemen yang sudah ada.
//
// Tidak ada field "page": elemen dicari berdasarkan id di seluruh dokumen, dan
// element.update TIDAK dapat memindahkan elemen antar halaman. Untuk itu,
// hapus lalu buat lagi di halaman tujuan.
type UpdateElementsInput struct {
	DocumentToken string           `json:"document_token" jsonschema:"token dokumen desain"`
	Elements      []design.Element `json:"elements" jsonschema:"elemen pengganti, LENGKAP; id-nya harus sudah ada di dokumen"`
}

func registerUpdateElements(server *mcp.Server, deps Deps) error {
	skema, err := skemaElemen[UpdateElementsInput]("elements")
	if err != nil {
		return err
	}

	mcp.AddTool(server, &mcp.Tool{
		Name:        "update_elements",
		InputSchema: skema,
		Description: "Mengganti satu atau beberapa elemen yang sudah ada. " +
			"PENGGANTIAN SELURUHNYA, bukan penggabungan: elemen yang dikirim " +
			"menimpa elemen lama apa adanya, sehingga properti yang tidak ikut " +
			"dikirim akan HILANG — termasuk locked, groupId, format, dan formula. " +
			"Menghilangkan formula mengubah sel yang dihitung menjadi angka mati " +
			"yang tidak akan pernah ikut berubah lagi, dan tidak ada galat yang " +
			"menandainya. Ambil elemen lewat read_document, ubah yang perlu, lalu " +
			"kirim balik utuh. Tidak dapat memindahkan elemen antar halaman.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in UpdateElementsInput) (*mcp.CallToolResult, EditResult, error) {
		if in.DocumentToken == "" {
			return fail("document_token wajib diisi"), EditResult{}, nil
		}
		if len(in.Elements) == 0 {
			return fail("elements kosong — tidak ada yang diubah"), EditResult{}, nil
		}
		if tolak := batasi(len(in.Elements), "elemen"); tolak != nil {
			return tolak, EditResult{}, nil
		}

		sesi, err := deps.Sessions.Open(ctx, in.DocumentToken)
		if err != nil {
			return fail(fmt.Sprintf("tidak dapat membuka dokumen: %v", err)), EditResult{}, nil
		}

		for i := range in.Elements {
			if in.Elements[i].ID == "" {
				return fail(fmt.Sprintf("elemen ke-%d tidak punya id", i+1)), EditResult{}, nil
			}
		}

		pesan := make([]any, 0, len(in.Elements))
		for i := range in.Elements {
			pesan = append(pesan, updateMessage{
				Type:    "element.update",
				Origin:  sesi.Origin(),
				Element: in.Elements[i],
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

		return laporkan(hasil, "elemen diubah"), keluaran, nil
	})

	return nil
}
