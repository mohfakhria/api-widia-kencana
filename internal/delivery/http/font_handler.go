package http

import (
	"net/http"

	"github.com/mohfakhria/api-widia-kencana/internal/delivery/http/dto"
	"github.com/mohfakhria/api-widia-kencana/internal/usecase/port/input"
	"github.com/mohfakhria/api-widia-kencana/pkg/apperror"

	"github.com/gin-gonic/gin"
)

type FontHandler struct {
	font input.FontUseCase
}

func NewFontHandler(font input.FontUseCase) *FontHandler {
	return &FontHandler{font: font}
}

// Register mengambil satu keluarga font dari sumbernya lalu menyimpannya.
//
// Menjawab 200 walau sebagian muka huruf gagal diambil, dan daftar hasilnya yang
// menyebutkan mana yang tersimpan. Satu bobot yang tidak tersedia adalah
// kejadian biasa — banyak keluarga tidak menyediakan seluruh bobot statik — dan
// menjadikannya kegagalan permintaan akan menyembunyikan sembilan muka huruf
// lain yang sebenarnya berhasil.
func (h *FontHandler) Register(c *gin.Context) {
	var req dto.FontRegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		dto.Error(c, http.StatusBadRequest, "Invalid request payload")
		return
	}

	faces, err := h.font.Register(c.Request.Context(), req.ToRegisterFontCommand())
	if err != nil {
		dto.Error(c, apperror.ToHTTPStatus(err), err.Error())
		return
	}

	dto.Success(c, "Success", dto.NewFontRegisterResponse(faces))
}
