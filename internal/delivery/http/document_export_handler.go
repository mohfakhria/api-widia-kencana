package http

import (
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/mohfakhria/api-widia-kencana/internal/delivery/http/dto"
	"github.com/mohfakhria/api-widia-kencana/internal/usecase/port/input"
	"github.com/mohfakhria/api-widia-kencana/pkg/apperror"

	"github.com/gin-gonic/gin"
)

type DocumentExportHandler struct {
	export input.DocumentExportUseCase
	logger *slog.Logger
}

func NewDocumentExportHandler(export input.DocumentExportUseCase, logger *slog.Logger) *DocumentExportHandler {
	if logger == nil {
		logger = slog.Default()
	}

	return &DocumentExportHandler{export: export, logger: logger}
}

// ExportPDF mengembalikan berkas PDF apa adanya, bukan amplop JSON seperti
// endpoint lain di aplikasi ini.
//
// Perbedaan itu disengaja: membungkus PDF sebagai base64 di dalam JSON akan
// membengkakkan muatannya sepertiga dan memaksa frontend menyusun ulang berkasnya
// sendiri, sedangkan byte mentah dapat langsung disimpan pengguna. Kegagalan
// tetap memakai amplop JSON yang sama dengan endpoint lain, karena saat itu tidak
// ada berkas yang dikirim.
func (h *DocumentExportHandler) ExportPDF(c *gin.Context) {
	token := c.Param("token")

	result, err := h.export.ExportPDF(c.Request.Context(), token)
	if err != nil {
		dto.Error(c, apperror.ToHTTPStatus(err), err.Error())
		return
	}

	h.logger.Info("exported document design to pdf",
		"document", token,
		"version", result.Version,
		"bytes", len(result.Content))

	c.Header("Content-Disposition", contentDisposition(result.Filename))
	// Isi dokumen dapat berubah setiap detik, jadi hasil ekspor tidak boleh
	// disimpan proxy maupun browser. Tanpa ini pengguna dapat menekan ekspor lagi
	// setelah menyunting dan menerima berkas yang sama dari cache.
	c.Header("Cache-Control", "no-store")
	c.Header("Content-Length", strconv.Itoa(len(result.Content)))
	c.Data(http.StatusOK, "application/pdf", result.Content)
}

// contentDisposition menyusun header nama berkas dalam dua bentuk sekaligus.
//
// Bentuk filename biasa hanya boleh memuat ASCII, sedangkan nama dokumen dapat
// memuat huruf beraksen. Karena itu keduanya disertakan: bentuk ASCII sebagai
// cadangan, dan bentuk filename* berkode UTF-8 yang dipakai browser mana pun yang
// memahaminya — yaitu semuanya sejak lama.
func contentDisposition(filename string) string {
	fallback := strings.Map(func(symbol rune) rune {
		if symbol > 0x7E || symbol < 0x20 {
			return '_'
		}

		return symbol
	}, filename)

	return "attachment; filename=\"" + fallback + "\"; filename*=UTF-8''" + encodeExtValue(filename)
}

// encodeExtValue menyandikan nama berkas sesuai ext-value pada RFC 5987.
//
// url.PathEscape tidak dapat dipakai di sini: ia membiarkan `$`, `&`, `+`, `=`,
// dan `@` apa adanya karena keduanya sah di dalam path URL, sedangkan pada header
// Content-Disposition karakter itu bukan attr-char dan dapat membuat pengurai
// yang ketat salah membaca batas parameter. Nama dokumen seperti "Invoice A&B"
// atau "50=50" cukup lazim untuk membuat ini terjadi.
//
// Penyandian dilakukan per byte, bukan per huruf, karena ext-value memang
// menuntut tiap byte UTF-8 disandikan sendiri-sendiri.
func encodeExtValue(text string) string {
	var builder strings.Builder
	builder.Grow(len(text))

	for index := range len(text) {
		char := text[index]
		if isAttrChar(char) {
			builder.WriteByte(char)
			continue
		}
		fmt.Fprintf(&builder, "%%%02X", char)
	}

	return builder.String()
}

// isAttrChar mengikuti daftar attr-char pada RFC 5987, yaitu huruf, angka, dan
// segelintir tanda baca yang aman berdiri sendiri di dalam header.
func isAttrChar(char byte) bool {
	switch {
	case char >= 'A' && char <= 'Z',
		char >= 'a' && char <= 'z',
		char >= '0' && char <= '9':
		return true
	}

	return strings.IndexByte("!#$&+-.^_`|~", char) >= 0
}
