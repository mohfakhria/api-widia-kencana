package http

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

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

// maxFontArchiveBytes membatasi arsip yang mau diterima.
//
// Ditegakkan lewat MaxBytesReader, yang memotong pembacaan di tengah alih-alih
// menunggu seluruh berkas masuk memori lebih dulu. Arsip Google untuk satu
// keluarga berbobot banyak berada di kisaran dua megabyte; dua puluh menampung
// beberapa keluarga sekaligus tanpa membuka pintu bagi unggahan yang mengada-ada.
const maxFontArchiveBytes = 20 << 20

// Register menerima arsip ZIP berisi berkas font, lalu menyimpan isinya.
//
// Multipart, bukan JSON: yang dikirim berkas biner, dan menyandikannya sebagai
// base64 di dalam JSON menaikkan ukurannya sepertiga tanpa menambah apa pun.
//
// Menjawab 200 walau sebagian entri dilewati, dan daftar hasilnya yang
// menyebutkan mana yang tersimpan beserta sebab yang tidak. Arsip yang sah
// memang memuat berkas bukan-font — OFL.txt, README.txt, dan font variabel —
// sehingga menjadikannya kegagalan permintaan akan menolak arsip yang benar.
func (h *FontHandler) Register(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxFontArchiveBytes)

	berkas, err := c.FormFile("archive")
	if err != nil {
		// Dipisahkan, karena keduanya tiba lewat galat yang sama. MaxBytesReader
		// menggagalkan penguraian multipart, dan tanpa pemisahan ini orang yang
		// mengunggah arsip 25 MB diberi tahu bahwa ia LUPA MEMILIH BERKAS —
		// petunjuk yang menuntunnya ke arah yang salah sama sekali.
		var kebesaran *http.MaxBytesError
		if errors.As(err, &kebesaran) {
			dto.Error(c, http.StatusRequestEntityTooLarge,
				fmt.Sprintf("archive is larger than %d bytes", maxFontArchiveBytes))

			return
		}

		dto.Error(c, http.StatusBadRequest, "archive file is required")

		return
	}

	dibuka, err := berkas.Open()
	if err != nil {
		dto.Error(c, http.StatusBadRequest, "archive file could not be read")
		return
	}
	defer dibuka.Close()

	isi, err := io.ReadAll(dibuka)
	if err != nil {
		dto.Error(c, http.StatusBadRequest, "archive file could not be read")
		return
	}

	faces, err := h.font.Register(c.Request.Context(), input.RegisterFontCommand{Archive: isi})
	if err != nil {
		dto.Error(c, apperror.ToHTTPStatus(err), err.Error())
		return
	}

	dto.Success(c, "Success", dto.NewFontRegisterResponse(faces))
}

// List menyebut font yang terpasang beserta alamat berkasnya.
//
// Dibaca frontend untuk menyusun @font-face DAN oleh editor untuk daftar
// pilihan. Satu sumber untuk keduanya, dan itu pokoknya: selama frontend
// memuat dari daftar ini, layar dan cetak memakai berkas yang sama persis
// alih-alih dua salinan yang kebetulan senama.
func (h *FontHandler) List(c *gin.Context) {
	families, err := h.font.List(c.Request.Context())
	if err != nil {
		dto.Error(c, apperror.ToHTTPStatus(err), err.Error())
		return
	}

	dto.Success(c, "Success", dto.NewFontListResponse(families))
}

// Content mengalihkan ke berkas font yang sesungguhnya.
//
// Di grup publik, dan itu keharusan seperti asset-content: rute ini dituju oleh
// aturan @font-face di dalam CSS, yang tidak dapat mengirim header
// Authorization.
func (h *FontHandler) Content(c *gin.Context) {
	weight, style, ok := parseFontFace(c.Param("face"))
	if !ok {
		dto.Error(c, http.StatusBadRequest, "Invalid font face")
		return
	}

	url, err := h.font.Content(c.Request.Context(), c.Param("family"), weight, style)
	if err != nil {
		dto.Error(c, apperror.ToHTTPStatus(err), err.Error())
		return
	}

	// Boleh disimpan peramban sebentar. Berbeda dari asset-content yang
	// no-store: berkas font tidak pernah berubah isinya untuk nama yang sama —
	// mendaftar ulang menimpa objek yang sama, dan itu perubahan yang jarang.
	c.Header("Cache-Control", "public, max-age=300")
	c.Redirect(http.StatusFound, url)
}

// parseFontFace mengurai "400-normal.ttf" menjadi bobot dan style.
func parseFontFace(face string) (weight int, style string, ok bool) {
	nama, cocok := strings.CutSuffix(face, ".ttf")
	if !cocok {
		return 0, "", false
	}

	angka, style, cocok := strings.Cut(nama, "-")
	if !cocok {
		return 0, "", false
	}

	weight, err := strconv.Atoi(angka)
	if err != nil {
		return 0, "", false
	}

	return weight, style, true
}
