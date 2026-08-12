package tool

import (
	"context"
	"fmt"

	"github.com/mohfakhria/api-widia-kencana/internal/mcp/apiclient"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// WhoAmIInput sengaja kosong.
//
// SDK menuntut tipe masukan berupa struct atau map supaya skema JSON-nya
// bertipe "object", sesuai spesifikasi MCP. Struct kosong menghasilkan objek
// tanpa properti — bentuk yang benar untuk tool tanpa argumen.
type WhoAmIInput struct{}

// registerWhoAmI memasang tool paling sederhana yang masih membuktikan sesuatu.
//
// Ia menembus seluruh rantai — protokol MCP, penjaga token, sesi agent, lalu
// API — dan mengembalikan identitas yang API sendiri akui.
//
// Nilainya bukan pada apa yang ia kerjakan melainkan pada apa yang kegagalannya
// beri tahu. Login yang berhasil sebagai akun KELIRU terlihat sama persis dengan
// yang benar sampai jauh kemudian, ketika sesuatu ditolak barier peran tanpa
// menyebut sebabnya.
func registerWhoAmI(server *mcp.Server, deps Deps) {
	mcp.AddTool(server, &mcp.Tool{
		Name: "whoami",
		Description: "Menyebutkan identitas agent yang dipakai server ini saat " +
			"menyunting dokumen: id pengguna, nama, dan perannya menurut API.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ WhoAmIInput) (*mcp.CallToolResult, apiclient.Identity, error) {
		identity, err := deps.API.Me(ctx)
		if err != nil {
			return fail(fmt.Sprintf("tidak dapat menghubungi API sebagai agent: %v", err)), apiclient.Identity{}, nil
		}

		return text(fmt.Sprintf("Agent %s (id %s) dengan peran %s.",
			identity.Name, identity.UserID, identity.Role)), identity, nil
	})
}
