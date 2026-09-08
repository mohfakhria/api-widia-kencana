package http

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/mohfakhria/api-widia-kencana/internal/delivery/http/dto"
	"github.com/mohfakhria/api-widia-kencana/internal/delivery/http/middleware"
	"github.com/mohfakhria/api-widia-kencana/internal/usecase/port/input"
	"github.com/mohfakhria/api-widia-kencana/pkg/apperror"

	"github.com/gin-gonic/gin"
)

type AssetHandler struct {
	asset input.AssetUseCase
}

func NewAssetHandler(asset input.AssetUseCase) *AssetHandler {
	return &AssetHandler{asset: asset}
}

// assetRef menyusun rujukan dari query, bukan dari path.
//
// Satu rute menerima kedua cara menyebut aset — ?token= dan ?key= — sehingga
// tidak ada rute kembar yang isinya sama persis kecuali cara memanggilnya.
// Pemeriksaan "keduanya sekaligus" ada di usecase, bukan di sini: ia aturan
// tentang aset, bukan tentang HTTP.
func assetRef(c *gin.Context) input.AssetRef {
	return input.AssetRef{
		Token: c.Query("token"),
		Key:   c.Query("key"),
	}
}

func (h *AssetHandler) RequestUpload(c *gin.Context) {
	var req dto.AssetUploadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		dto.Error(c, http.StatusBadRequest, "Invalid request payload")
		return
	}

	result, err := h.asset.RequestUpload(c.Request.Context(), req.ToRequestAssetUploadCommand(currentUserID(c)))
	if err != nil {
		dto.Error(c, apperror.ToHTTPStatus(err), err.Error())
		return
	}

	dto.Success(c, "Asset upload URL generated successfully", dto.NewAssetUploadRequestResponse(result))
}

func (h *AssetHandler) CompleteUpload(c *gin.Context) {
	asset, err := h.asset.CompleteUpload(c.Request.Context(), assetRef(c), currentUserID(c))
	if err != nil {
		dto.Error(c, apperror.ToHTTPStatus(err), err.Error())
		return
	}

	dto.Success(c, "Asset upload completed successfully", dto.NewAssetDataResponse(asset))
}

// Content mengalihkan ke isi aset di object storage.
//
// Ada supaya frontend dapat menulis <img src="/api/asset-content?token=..."> dan
// selesai — URL-nya tetap, tidak pernah kedaluwarsa, dan tidak menuntut mesin
// penyegar. Yang kedaluwarsa adalah sasaran pengalihannya, dan itu disusun ulang
// pada setiap permintaan.
//
// TIDAK DIJAGA AuthRequired, dan memang tidak bisa: tag <img> tidak dapat
// mengirim header Authorization.
//
// MENERIMA ?key= JUGA, dan itu melonggarkan sifat yang dulu menjadi seluruh
// pengamanannya. Dulu kredensialnya adalah tokennya sendiri: UUID acak yang
// tidak dapat ditebak, sehingga yang dapat membacanya hanya orang yang memang
// diberi tahu. Key justru dirancang untuk mudah ditebak — logo-widia-kencana —
// sehingga aset ber-key menjadi dapat dienumerasi siapa pun.
//
// Diterima dengan sadar, dengan batas yang jelas: hanya aset yang SENGAJA
// DINAMAI yang terjangkau begitu. Gambar yang diunggah orang ke dalam dokumen
// tidak punya key, jadi ia tetap hanya dapat dibuka lewat tokennya. Jangan
// memberi key kepada aset yang tidak boleh dilihat sembarang orang.
//
// Byte-nya tidak pernah melewati proses ini — hanya alamatnya. Menyalurkan
// isinya sendiri akan membuat setiap pembukaan dokumen berisi sepuluh gambar
// mendorong puluhan megabyte melalui API, yang justru dihindari seluruh
// rancangan presigned.
func (h *AssetHandler) Content(c *gin.Context) {
	url, err := h.asset.ContentURL(c.Request.Context(), assetRef(c))
	if err != nil {
		dto.Error(c, apperror.ToHTTPStatus(err), err.Error())
		return
	}

	// no-store, bukan sekadar no-cache. Pengalihan yang tersimpan akan menunjuk
	// ke URL bertanda tangan yang keburu kedaluwarsa, dan gejalanya gambar yang
	// gagal dimuat lalu sembuh sendiri beberapa menit kemudian — jenis kesalahan
	// yang paling sulit dilacak karena tidak dapat diulang.
	c.Header("Cache-Control", "no-store")
	c.Redirect(http.StatusFound, url)
}

// maxAssetReplacementBytes membatasi berkas pengganti yang mau diterima.
//
// Sedikit di atas batas usecase, dan selisihnya disengaja — sama seperti pada
// font-add: dengan begitu penolakan datang dari aplikasi sebagai JSON yang
// menyebut angkanya, bukan sebagai halaman HTML bawaan nginx.
const maxAssetReplacementBytes = 11 << 20

// Replace mengganti ISI sebuah aset. Token, key, dan kelompoknya tetap.
//
// Multipart dan satu permintaan, berbeda dari unggahan biasa yang memakai
// presigned URL dua langkah. Alasannya ada di usecase: alur dua langkah menuntut
// keadaan "sedang diganti" tersimpan di suatu tempat, dan tempat yang paling
// wajar untuk itu membuat penyapu menghapus berkas yang sedang hidup.
func (h *AssetHandler) Replace(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxAssetReplacementBytes)

	berkas, err := c.FormFile("file")
	if err != nil {
		var kebesaran *http.MaxBytesError
		if errors.As(err, &kebesaran) {
			dto.Error(c, http.StatusRequestEntityTooLarge,
				fmt.Sprintf("replacement is larger than %d bytes", maxAssetReplacementBytes))

			return
		}

		dto.Error(c, http.StatusBadRequest, "replacement file is required")

		return
	}

	dibuka, err := berkas.Open()
	if err != nil {
		dto.Error(c, http.StatusBadRequest, "replacement file could not be read")
		return
	}
	defer dibuka.Close()

	isi, err := io.ReadAll(dibuka)
	if err != nil {
		dto.Error(c, http.StatusBadRequest, "replacement file could not be read")
		return
	}

	asset, err := h.asset.ReplaceContent(c.Request.Context(), input.ReplaceAssetContentCommand{
		Ref:              assetRef(c),
		OriginalFilename: berkas.Filename,
		// Tipe diambil dari bagian multipart-nya, bukan ditebak dari ekstensi:
		// yang menentukan bagaimana peramban kelak menyajikannya adalah nilai
		// ini, dan menebaknya salah menghasilkan gambar yang terunduh alih-alih
		// tampil.
		MimeType: berkas.Header.Get("Content-Type"),
		Content:  isi,
	})
	if err != nil {
		dto.Error(c, apperror.ToHTTPStatus(err), err.Error())
		return
	}

	dto.Success(c, "Success", gin.H{"asset": dto.NewAssetResponse(asset)})
}

func (h *AssetHandler) List(c *gin.Context) {
	var req dto.AssetListFilterRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		dto.Error(c, http.StatusBadRequest, "Invalid query parameters")
		return
	}

	query := req.ToListAssetQuery()
	query.UploadedBy = currentUserID(c)
	assets, err := h.asset.List(c.Request.Context(), query)
	if err != nil {
		dto.Error(c, apperror.ToHTTPStatus(err), err.Error())
		return
	}

	dto.Success(c, "Success", dto.NewAssetListResponse(assets))
}

// Get mengembalikan satu aset, dan dengan ?presign=true sekaligus URL sementara
// untuk mengunduhnya.
//
// Dulu keduanya dua rute. Yang kedua sebenarnya "detail ditambah satu URL",
// bukan perilaku yang berbeda — dan rute yang hanya menambah satu field pada
// jawaban rute lain adalah rute yang menuntut perawatan tanpa membawa apa pun.
//
// Presign TIDAK disertakan secara bawaan, dan itu bukan penghematan byte:
// menerbitkannya menandatangani alamat yang dapat dipakai siapa pun yang
// menerimanya, jadi ia harus diminta secara sadar.
func (h *AssetHandler) Get(c *gin.Context) {
	ref := assetRef(c)

	if strings.EqualFold(c.Query("presign"), "true") {
		result, err := h.asset.PresignGet(c.Request.Context(), ref, currentUserID(c))
		if err != nil {
			dto.Error(c, apperror.ToHTTPStatus(err), err.Error())
			return
		}

		dto.Success(c, "Success", dto.NewAssetPresignGetResponse(result))

		return
	}

	asset, err := h.asset.Get(c.Request.Context(), ref, currentUserID(c))
	if err != nil {
		dto.Error(c, apperror.ToHTTPStatus(err), err.Error())
		return
	}

	dto.Success(c, "Success", dto.NewAssetDataResponse(asset))
}

func (h *AssetHandler) Delete(c *gin.Context) {
	if err := h.asset.Delete(c.Request.Context(), c.Query("token"), currentUserID(c)); err != nil {
		dto.Error(c, apperror.ToHTTPStatus(err), err.Error())
		return
	}

	dto.Success(c, "Asset deleted successfully", nil)
}

// currentUserID mengembalikan nil bila permintaan tidak terautentikasi.
//
// Pointer, bukan nilai: nol adalah id yang mustahil, tetapi memperlakukannya
// sebagai "tidak ada" berarti setiap pemakai harus mengingat perjanjian itu.
// nil tidak menuntut siapa pun mengingat apa pun.
func currentUserID(c *gin.Context) *int64 {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		return nil
	}

	if user.UserID == 0 {
		return nil
	}

	userID := user.UserID

	return &userID
}
