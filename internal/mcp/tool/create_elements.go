package tool

import (
	"context"
	"fmt"

	"github.com/mohfakhria/api-widia-kencana/internal/domain/design"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// CreateElementsInput menambahkan elemen ke satu halaman.
//
// Elements memakai design.Element apa adanya, dan itu keputusan pokoknya:
// skema JSON tool ini DITURUNKAN dari tipe domain, bukan ditulis ulang sebagai
// prosa. Model karenanya menerima kontraknya sebagai sesuatu yang dapat
// diperiksa mesin — nama field yang tidak ada di sana tidak akan pernah ia
// kirim, dan itu penting karena server menolak SELURUH pesan bila ada satu
// field asing.
type CreateElementsInput struct {
	DocumentToken string           `json:"document_token" jsonschema:"token dokumen desain"`
	Page          string           `json:"page" jsonschema:"id halaman tempat elemen dipasang"`
	Elements      []design.Element `json:"elements" jsonschema:"elemen yang ditambahkan; tiap elemen wajib punya id unik se-dokumen"`
}

// EditResult adalah hasil satu pemanggilan tool penyuntingan.
//
// Applied dan Confirmed menjawab dua pertanyaan berbeda: berapa pesan yang
// keluar dari socket, dan apakah dokumen benar-benar berubah karenanya. Yang
// kedua tidak selalu dapat dipastikan — lihat kirim di edit.go.
type EditResult struct {
	DocumentToken string `json:"document_token"`
	Applied       int    `json:"applied"`
	Confirmed     bool   `json:"confirmed"`
	Version       int64  `json:"version"`
}

func registerCreateElements(server *mcp.Server, deps Deps) error {
	skema, err := skemaElemen[CreateElementsInput]("elements")
	if err != nil {
		return err
	}

	mcp.AddTool(server, &mcp.Tool{
		Name:        "create_elements",
		InputSchema: skema,
		Description: "Menambahkan satu atau beberapa elemen ke sebuah halaman. " +
			"Setiap elemen WAJIB membawa id yang unik di seluruh dokumen — bukan " +
			"hanya di halamannya — dan koordinat dalam point dengan titik asal di " +
			"sudut kiri-atas halaman. Panggil read_document lebih dulu untuk " +
			"mengetahui id halaman, ukuran kertas, dan elemen yang sudah ada.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in CreateElementsInput) (*mcp.CallToolResult, EditResult, error) {
		if in.DocumentToken == "" || in.Page == "" {
			return fail("document_token dan page wajib diisi"), EditResult{}, nil
		}
		if len(in.Elements) == 0 {
			return fail("elements kosong — tidak ada yang dibuat"), EditResult{}, nil
		}
		if tolak := batasi(len(in.Elements), "elemen"); tolak != nil {
			return tolak, EditResult{}, nil
		}

		sesi, err := deps.Sessions.Open(ctx, in.DocumentToken)
		if err != nil {
			return fail(fmt.Sprintf("tidak dapat membuka dokumen: %v", err)), EditResult{}, nil
		}

		// Divalidasi DI SINI, sebelum satu pesan pun dikirim. Server menolak per
		// pesan, sehingga elemen kelima yang cacat akan menyisakan empat yang
		// sudah telanjur mendarat — dokumen setengah jadi yang lebih sulit
		// dibereskan daripada tidak dikerjakan sama sekali.
		for i := range in.Elements {
			if in.Elements[i].ID == "" {
				return fail(fmt.Sprintf("elemen ke-%d tidak punya id", i+1)), EditResult{}, nil
			}
		}

		pesan := make([]any, 0, len(in.Elements))
		for i := range in.Elements {
			pesan = append(pesan, createMessage{
				Type:    "element.create",
				Origin:  sesi.Origin(),
				Page:    in.Page,
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

		return laporkan(hasil, "elemen dibuat"), keluaran, nil
	})

	return nil
}
