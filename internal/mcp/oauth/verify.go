package oauth

import (
	"context"
	"fmt"
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/auth"
)

// Verifier menjawab sisi resource server: token ini sah atau tidak.
//
// Dicari di penyimpanan, bukan diperiksa tanda tangannya. Itu konsekuensi
// langsung dari authorization server yang berada di proses yang sama, dan yang
// dibeli dengannya adalah pencabutan seketika — begitu token dihapus, ia
// berhenti berlaku pada permintaan berikutnya, bukan pada tenggatnya.
func (s *Server) Verifier() auth.TokenVerifier {
	return func(_ context.Context, token string, _ *http.Request) (*auth.TokenInfo, error) {
		t, ok := s.store.ambilToken(token)
		if !ok {
			return nil, fmt.Errorf("%w: token tidak dikenal atau sudah kedaluwarsa", auth.ErrInvalidToken)
		}

		return &auth.TokenInfo{
			Scopes:     []string{scopeDesign},
			Expiration: t.ExpiresAt,

			// UserID dipakai transport SDK untuk mencegah satu sesi MCP dipakai
			// dua orang berbeda. Diisi dengan id pengguna, bukan client_id: yang
			// hendak dijaga adalah orangnya.
			UserID: t.Subject.UserID,

			Extra: map[string]any{
				"client_id": t.ClientID,
				"email":     t.Subject.Email,
				"name":      t.Subject.Name,
				"role":      t.Subject.Role,
			},
		}, nil
	}
}
