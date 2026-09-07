package usecase

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"time"

	"github.com/mohfakhria/api-widia-kencana/internal/domain"
	"github.com/mohfakhria/api-widia-kencana/internal/domain/entity"
	"github.com/mohfakhria/api-widia-kencana/internal/usecase/port/input"
	"github.com/mohfakhria/api-widia-kencana/internal/usecase/port/output"

	"github.com/google/uuid"
)

const defaultAssetUploadExpiry = 15 * time.Minute
const defaultAssetPreviewExpiry = 15 * time.Minute

// assetGroups adalah kosakata TERTUTUP kelompok aset, dan sekaligus folder
// tempat objeknya mendarat.
//
// Tidak ada kolom kelompok di tabel: yang menyimpannya nama objek itu sendiri —
// images/brand/logo.png — dan entity.Asset.Group membacanya balik. Pola yang
// sama sudah dipakai font, yang bahkan tidak punya tabel sama sekali.
//
// TIDAK ADA BAWAAN. Kelompok yang tidak dikenal ditolak, bukan jatuh ke nilai
// pilihan kami: bawaan yang menelan kekeliruan adalah persis sebabnya hari ini
// ada folder bernama "documents" yang isinya dua berkas logo.
//
// Kelompok dokumen diturunkan dari allowedDocumentTypes, bukan ditulis ulang di
// sini. Dua daftar yang menyebut hal yang sama pasti berselisih suatu hari, dan
// yang menambahkan jenis dokumen baru tidak akan pernah menduga ada daftar kedua
// yang perlu ikut disunting.
//
// fonts/ SENGAJA di luar daftar ini. Berkas di bawahnya tidak punya baris di
// tabel assets — nama objeknya fungsi murni dari keluarga, bobot, dan style —
// dan mengizinkan unggahan aset mendarat di sana menaruh berkas asing di ruang
// nama yang dibaca font-list.
const (
	AssetGroupBrand    = "images/brand"
	AssetGroupDocument = "images/document"

	assetGroupDocumentPrefix = "documents"
)

func assetGroups() map[string]struct{} {
	groups := map[string]struct{}{
		AssetGroupBrand:    {},
		AssetGroupDocument: {},
	}
	for documentType := range allowedDocumentTypes {
		groups[assetGroupDocumentPrefix+"/"+documentType] = struct{}{}
	}

	return groups
}

var allowedAssetStatuses = map[string]struct{}{
	"pending":   {},
	"uploading": {},
	"uploaded":  {},
	"failed":    {},
	"deleted":   {},
}

type assetUseCase struct {
	repo    output.AssetRepository
	storage output.ObjectStorage
	logger  *slog.Logger
}

// Logger dipakai untuk satu hal saja: melaporkan objek lama yang gagal dibuang
// setelah isinya diganti. Kegagalan itu sengaja tidak menggagalkan permintaan —
// berkas barunya sudah terpasang — sehingga tanpa catatan ini objek yatim
// menumpuk tanpa ada yang pernah tahu.
func NewAssetUseCase(repo output.AssetRepository, storage output.ObjectStorage, logger *slog.Logger) input.AssetUseCase {
	if logger == nil {
		logger = slog.Default()
	}

	return &assetUseCase{repo: repo, storage: storage, logger: logger}
}

func (uc *assetUseCase) RequestUpload(ctx context.Context, cmd input.RequestAssetUploadCommand) (*input.AssetUploadRequestResult, error) {
	if uc.storage == nil {
		return nil, domain.NewError(domain.ErrUnavailable, "asset storage is unavailable")
	}
	if cmd.Size <= 0 {
		return nil, domain.NewError(domain.ErrInvalidInput, "asset size must be greater than 0")
	}
	if strings.TrimSpace(cmd.OriginalFilename) == "" {
		return nil, domain.NewError(domain.ErrInvalidInput, "asset filename is required")
	}

	contentType := strings.TrimSpace(cmd.MimeType)
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	group := strings.ToLower(strings.TrimSpace(cmd.Group))
	if _, ok := assetGroups()[group]; !ok {
		return nil, domain.NewError(domain.ErrInvalidInput, "invalid asset group")
	}

	key, err := sanitizeAssetKey(cmd.Key)
	if err != nil {
		return nil, err
	}

	// Ber-key memakai namanya sendiri; tanpa key memakai bentuk lama yang
	// berawalan UUID. Yang kedua tidak terbaca manusia, tetapi ia menjamin dua
	// orang yang mengunggah "foto.png" pada detik yang sama tidak bertabrakan.
	storedFilename := buildStoredAssetFilename(cmd.OriginalFilename)
	if key != nil {
		storedFilename = *key + extensionSuffix(cmd.OriginalFilename)
	}

	objectName := fmt.Sprintf("%s/%s", group, storedFilename)
	expiresAt := time.Now().Add(defaultAssetUploadExpiry)

	asset := &entity.Asset{
		Bucket:             uc.storage.Bucket(),
		ObjectName:         objectName,
		Key:                key,
		OriginalFilename:   strings.TrimSpace(cmd.OriginalFilename),
		StoredFilename:     storedFilename,
		MimeType:           contentType,
		Extension:          strings.TrimPrefix(strings.ToLower(filepath.Ext(cmd.OriginalFilename)), "."),
		Size:               cmd.Size,
		Status:             "pending",
		UploadMethod:       "presigned_put",
		IsPrivate:          true,
		UploadedBy:         cmd.UploadedBy,
		PresignedExpiresAt: &expiresAt,
	}

	created, err := uc.repo.CreatePending(ctx, asset)
	if err != nil {
		return nil, err
	}

	uploadURL, err := uc.storage.PresignPut(ctx, created.ObjectName, defaultAssetUploadExpiry, contentType)
	if err != nil {
		_ = uc.repo.MarkFailed(ctx, created.Token, "presign_failed", err.Error())
		return nil, err
	}

	return &input.AssetUploadRequestResult{
		Asset:     created,
		UploadURL: uploadURL,
		ExpiresAt: expiresAt,
	}, nil
}

func (uc *assetUseCase) CompleteUpload(ctx context.Context, token string, uploadedBy *int64) (*entity.Asset, error) {
	token = strings.TrimSpace(token)
	if err := validateAssetUUIDToken(token, "asset token"); err != nil {
		return nil, err
	}

	asset, err := uc.repo.GetByToken(ctx, token)
	if err != nil {
		return nil, err
	}
	if err := ensureAssetOwner(asset, uploadedBy); err != nil {
		return nil, err
	}
	if asset.Status == "uploaded" {
		return asset, nil
	}
	if asset.Status != "pending" && asset.Status != "uploading" {
		return nil, domain.NewError(domain.ErrInvalidInput, "asset cannot be completed from current status")
	}
	if asset.PresignedExpiresAt != nil && time.Now().After(*asset.PresignedExpiresAt) {
		_ = uc.repo.MarkFailed(ctx, token, "upload_expired", "asset upload URL expired")
		return nil, domain.NewError(domain.ErrInvalidInput, "asset upload URL expired")
	}

	stored, err := uc.storage.Stat(ctx, asset.ObjectName)
	if err != nil {
		_ = uc.repo.MarkFailed(ctx, token, "object_not_found", err.Error())
		return nil, domain.NewError(domain.ErrNotFound, "uploaded asset object not found")
	}
	if stored.Size != asset.Size {
		_ = uc.repo.MarkFailed(ctx, token, "size_mismatch", "uploaded asset size does not match request")
		return nil, domain.NewError(domain.ErrInvalidInput, "uploaded asset size does not match request")
	}
	if !assetContentTypeMatches(asset.MimeType, stored.ContentType) {
		_ = uc.repo.MarkFailed(ctx, token, "mime_type_mismatch", "uploaded asset MIME type does not match request")
		return nil, domain.NewError(domain.ErrInvalidInput, "uploaded asset MIME type does not match request")
	}

	return uc.repo.MarkUploaded(ctx, token, stored)
}

func (uc *assetUseCase) List(ctx context.Context, query input.ListAssetQuery) ([]entity.Asset, error) {
	query.Status = strings.ToLower(strings.TrimSpace(query.Status))
	query.Group = strings.ToLower(strings.TrimSpace(query.Group))
	query.Key = strings.ToLower(strings.TrimSpace(query.Key))
	query.MimeType = strings.TrimSpace(query.MimeType)
	query.Extension = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(query.Extension)), ".")
	if query.Status != "" {
		if _, ok := allowedAssetStatuses[query.Status]; !ok {
			return nil, domain.NewError(domain.ErrInvalidInput, "invalid asset status")
		}
	}

	return uc.repo.List(ctx, query)
}

func (uc *assetUseCase) GetByToken(ctx context.Context, token string, uploadedBy *int64) (*entity.Asset, error) {
	token = strings.TrimSpace(token)
	if err := validateAssetUUIDToken(token, "asset token"); err != nil {
		return nil, err
	}

	asset, err := uc.repo.GetByToken(ctx, token)
	if err != nil {
		return nil, err
	}
	if err := ensureAssetReadable(asset, uploadedBy); err != nil {
		return nil, err
	}

	return asset, nil
}

func (uc *assetUseCase) PresignGet(ctx context.Context, token string, uploadedBy *int64) (*input.AssetPresignGetResult, error) {
	token = strings.TrimSpace(token)
	if err := validateAssetUUIDToken(token, "asset token"); err != nil {
		return nil, err
	}

	asset, err := uc.repo.GetByToken(ctx, token)
	if err != nil {
		return nil, err
	}
	if err := ensureAssetReadable(asset, uploadedBy); err != nil {
		return nil, err
	}
	if asset.Status != "uploaded" {
		return nil, domain.NewError(domain.ErrInvalidInput, "asset is not uploaded")
	}

	url, err := uc.storage.PresignGet(ctx, asset.ObjectName, defaultAssetPreviewExpiry)
	if err != nil {
		return nil, err
	}

	return &input.AssetPresignGetResult{
		Asset:     asset,
		URL:       url,
		ExpiresIn: int64(defaultAssetPreviewExpiry.Seconds()),
	}, nil
}

// ContentURL menyusun URL isi aset tanpa bertanya siapa pemanggilnya.
//
// TOKENNYA ADALAH KREDENSIALNYA. Jalur ini dituju langsung oleh tag <img>, dan
// tag itu tidak dapat mengirim header Authorization — jadi tidak ada tempat lain
// untuk menaruh bukti identitas selain di dalam URL-nya sendiri.
//
// Yang membuatnya dapat diterima: tokennya UUID acak, tidak pernah muncul di
// daftar milik orang lain, dan satu-satunya cara memperolehnya adalah lewat isi
// dokumen yang memuatnya. Yang harus disadari: siapa pun yang memegang UUID itu
// dapat membaca gambarnya, termasuk orang yang tidak pernah login.
//
// Sengaja TIDAK memakai ensureAssetReadable. Bukan karena pemeriksaannya
// dilewati, melainkan karena di sini memang tidak ada siapa pun untuk diperiksa —
// menyamarkannya sebagai pemeriksaan yang lolos akan membuat pembaca berikutnya
// mengira jalur ini terjaga.
func (uc *assetUseCase) ContentURL(ctx context.Context, token string) (string, error) {
	token = strings.TrimSpace(token)
	if err := validateAssetUUIDToken(token, "asset token"); err != nil {
		return "", err
	}

	asset, err := uc.repo.GetByToken(ctx, token)
	if err != nil {
		return "", err
	}
	if asset.Status != "uploaded" {
		return "", domain.NewError(domain.ErrInvalidInput, "asset is not uploaded")
	}

	return uc.storage.PresignGet(ctx, asset.ObjectName, defaultAssetPreviewExpiry)
}

func (uc *assetUseCase) Delete(ctx context.Context, token string, uploadedBy *int64) error {
	token = strings.TrimSpace(token)
	if err := validateAssetUUIDToken(token, "asset token"); err != nil {
		return err
	}

	asset, err := uc.repo.GetByToken(ctx, token)
	if err != nil {
		return err
	}
	if err := ensureAssetOwner(asset, uploadedBy); err != nil {
		return err
	}
	if asset.Status == "uploaded" {
		if err := uc.storage.Delete(ctx, asset.ObjectName); err != nil {
			return err
		}
	}

	return uc.repo.MarkDeleted(ctx, token)
}

// maxAssetReplacementBytes membatasi berkas pengganti.
//
// Lebih kecil daripada unggahan biasa dengan sengaja: yang lewat jalur ini aset
// bernama — logo, stempel, tanda tangan — dan berkas sebesar itu di sana selalu
// keliru.
const maxAssetReplacementBytes = 10 << 20

// ReplaceContent menukar berkas di balik sebuah aset, tanpa menyentuh token,
// key, maupun kelompoknya.
//
// Itu seluruh gunanya: setiap dokumen menunjuk TOKEN, jadi mengganti isinya
// membuat tiga puluh dokumen ikut memakai berkas baru tanpa satu pun disunting.
//
// ISINYA DIBAWA LANGSUNG, bukan lewat presigned URL seperti unggahan biasa, dan
// itu keputusan yang diambil sadar. Alur presign menuntut keadaan "sedang
// diganti" tersimpan di suatu tempat, dan setiap tempat yang wajar untuk itu
// berbahaya: memakai kembali baris yang sama berarti mengembalikan statusnya ke
// pending, dan penyapu — yang menghapus pending kedaluwarsa BESERTA objeknya —
// akan menghapus logo yang sedang hidup lima belas menit setelah seseorang
// berubah pikiran, tanpa satu pun galat. Menaruhnya di kolom baru menuntut
// penyapu ikut tahu. Satu permintaan yang membawa bytes-nya menghapus seluruh
// keadaan antara itu.
//
// Yang ditukar: bytes-nya melewati API, bukan langsung ke object storage. Untuk
// jalur yang jarang dipakai dan berkas berukuran ratusan kilobita, itu murah —
// dan font-add sudah menempuh jalan yang sama.
func (uc *assetUseCase) ReplaceContent(ctx context.Context, cmd input.ReplaceAssetContentCommand) (*entity.Asset, error) {
	if uc.storage == nil {
		return nil, domain.NewError(domain.ErrUnavailable, "asset storage is unavailable")
	}
	if len(cmd.Content) == 0 {
		return nil, domain.NewError(domain.ErrInvalidInput, "replacement file is empty")
	}
	if strings.TrimSpace(cmd.OriginalFilename) == "" {
		return nil, domain.NewError(domain.ErrInvalidInput, "asset filename is required")
	}

	asset, err := uc.repo.GetByToken(ctx, cmd.Token)
	if err != nil {
		return nil, err
	}

	// Hanya aset yang isinya memang sudah ada. Mengganti unggahan yang belum
	// pernah selesai berarti mencampur dua alur yang berbeda pada satu baris,
	// dan yang kedua sudah punya jalannya sendiri lewat asset-upload-complete.
	if asset.Status != assetStatusUploaded {
		return nil, domain.NewError(domain.ErrInvalidInput, "asset is not uploaded")
	}

	group := asset.Group()
	if _, ok := assetGroups()[group]; !ok {
		return nil, domain.NewError(domain.ErrInvalidInput, "invalid asset group")
	}

	contentType := strings.TrimSpace(cmd.MimeType)
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	// Aset ber-key mempertahankan namanya; yang tanpa key mendapat nama baru
	// berawalan UUID. Akibatnya berbeda saat membersihkan: yang pertama menimpa
	// objek yang sama — dan penimpaan pada object storage bersifat utuh, tidak
	// ada keadaan setengah terlihat — sedangkan yang kedua menyisakan objek lama
	// yang harus dibuang setelah barisnya berpindah.
	storedFilename := buildStoredAssetFilename(cmd.OriginalFilename)
	if asset.Key != nil {
		storedFilename = *asset.Key + extensionSuffix(cmd.OriginalFilename)
	}
	objectName := fmt.Sprintf("%s/%s", group, storedFilename)

	stored, err := uc.storage.Upload(ctx, output.UploadObject{
		ObjectName:  objectName,
		Reader:      bytes.NewReader(cmd.Content),
		Size:        int64(len(cmd.Content)),
		ContentType: contentType,
	})
	if err != nil {
		return nil, domain.NewError(domain.ErrInternalFailure, "asset replacement could not be stored")
	}

	// Barisnya berpindah SETELAH berkasnya benar-benar ada. Kalau unggahan di
	// atas gagal, aset lamanya sama sekali tidak tersentuh.
	replaced, err := uc.repo.ReplaceContent(ctx, asset.Token, output.ReplacedContent{
		ObjectName:       objectName,
		OriginalFilename: strings.TrimSpace(cmd.OriginalFilename),
		StoredFilename:   storedFilename,
		MimeType:         contentType,
		Extension:        strings.TrimPrefix(extensionSuffix(cmd.OriginalFilename), "."),
		Size:             stored.Size,
		ETag:             stored.ETag,
	})
	if err != nil {
		return nil, err
	}

	// Objek lama dibuang hanya bila namanya memang berbeda, dan kegagalannya
	// TIDAK menggagalkan penggantian: berkas barunya sudah terpasang dan sudah
	// dipakai. Yang tertinggal objek yatim, bukan aset yang rusak.
	if asset.ObjectName != objectName {
		if err := uc.storage.Delete(ctx, asset.ObjectName); err != nil {
			uc.logger.Warn("delete replaced asset object",
				"asset", asset.Token, "object", asset.ObjectName, "error", err)
		}
	}

	return replaced, nil
}

// sanitizeAssetKey menormalkan nama slot, dan mengembalikan nil bila tidak ada.
//
// NIL, bukan string kosong. Indeks uniknya mengabaikan NULL tetapi menganggap
// dua string kosong bertabrakan — sehingga unggahan KEDUA tanpa key akan
// ditolak, dengan gejala yang jauh dari sebabnya. Nilai nol sebuah string di Go
// adalah "", jadi pengubahan ini harus ditulis sadar; ia tidak terjadi sendiri.
func sanitizeAssetKey(key string) (*string, error) {
	key = strings.ToLower(strings.TrimSpace(key))
	if key == "" {
		return nil, nil
	}

	var builder strings.Builder
	builder.Grow(len(key))
	for _, symbol := range key {
		switch {
		case symbol >= 'a' && symbol <= 'z', symbol >= '0' && symbol <= '9':
			builder.WriteRune(symbol)
		case symbol == ' ', symbol == '-', symbol == '_', symbol == '.':
			if builder.Len() > 0 && !strings.HasSuffix(builder.String(), "-") {
				builder.WriteByte('-')
			}
		}
	}

	cleaned := strings.Trim(builder.String(), "-")
	if cleaned == "" {
		return nil, domain.NewError(domain.ErrInvalidInput, "asset key has no usable characters")
	}

	return &cleaned, nil
}

// extensionSuffix mengembalikan ".png" dan sejenisnya, atau kosong.
//
// Dipakai HANYA supaya nama objek enak dibaca manusia. Yang stabil tetap
// tokennya: mengganti isi slot dengan berkas berformat lain mengubah nama
// objeknya, dan itu tidak apa-apa.
func extensionSuffix(originalFilename string) string {
	ext := strings.ToLower(filepath.Ext(originalFilename))
	if ext == "." {
		return ""
	}

	return ext
}

// ensureAssetReadable: siapa pun yang login boleh membaca aset, asal ia tahu
// tokennya.
//
// Sengaja tidak dibatasi pengunggahnya. Dokumen di aplikasi ini milik bersama —
// tidak ada kepemilikan dokumen sama sekali — sehingga gambar yang tokennya sudah
// masuk ke assetToken sebuah elemen SUDAH menjadi bagian dari dokumen bersama itu.
// Menjaganya tetap privat sesudah itu hanya menyembunyikannya dari editor, bukan
// dari orang: ekspor PDF menyematkannya untuk siapa pun yang membuka dokumennya.
//
// Sebelum ini kedua pintu itu tidak sepakat, dan akibatnya adalah gambar yang
// kosong di layar tetapi muncul di hasil cetak — perpecahan yang justru paling
// ingin dihindari fitur ini.
//
// Tokennya UUID acak dan tidak pernah muncul di daftar milik orang lain, jadi
// yang dapat membacanya tetap hanya orang yang memang diberi tahu — lewat dokumen
// yang memuatnya.
func ensureAssetReadable(asset *entity.Asset, viewer *int64) error {
	if asset == nil {
		return domain.NewError(domain.ErrNotFound, "asset not found")
	}
	if viewer == nil {
		return domain.NewError(domain.ErrUnauthorized, "Invalid or expired token")
	}

	return nil
}

// ensureAssetOwner: hanya pengunggahnya yang boleh mengubah atau membuang.
//
// Aset tanpa pemilik ditolak untuk SIAPA PUN. Sebelumnya ia justru diizinkan
// untuk semua orang, dan itu kebalikan dari yang seharusnya: kolom uploaded_by
// memakai ON DELETE SET NULL, sehingga menghapus satu baris user akan mengubah
// seluruh asetnya menjadi milik bersama yang dapat dihapus siapa saja — diam-diam,
// dan tidak terlihat di daftar siapa pun.
//
// Belum ada jalur di aplikasi ini yang menghapus user, jadi keadaan itu belum
// dapat dicapai lewat API. Satu DELETE manual di database sudah cukup untuk
// membukanya, dan pintu yang hanya butuh satu perintah untuk terbuka lebih baik
// ditutup sekarang.
//
// Konsekuensinya aset tanpa pemilik tidak dapat dihapus lewat API sama sekali,
// dan hanya dapat diurus lewat database.
func ensureAssetOwner(asset *entity.Asset, actor *int64) error {
	if asset == nil {
		return domain.NewError(domain.ErrNotFound, "asset not found")
	}
	if actor == nil {
		return domain.NewError(domain.ErrUnauthorized, "Invalid or expired token")
	}
	if asset.UploadedBy == nil || *asset.UploadedBy != *actor {
		return domain.NewError(domain.ErrForbidden, "asset access forbidden")
	}

	return nil
}

func assetContentTypeMatches(expected string, actual string) bool {
	expected = normalizeAssetContentType(expected)
	actual = normalizeAssetContentType(actual)
	return actual == "" || expected == "" || expected == "application/octet-stream" || actual == expected
}

func normalizeAssetContentType(contentType string) string {
	contentType = strings.ToLower(strings.TrimSpace(contentType))
	if idx := strings.Index(contentType, ";"); idx >= 0 {
		contentType = strings.TrimSpace(contentType[:idx])
	}

	return contentType
}

func validateAssetUUIDToken(token, label string) error {
	if strings.TrimSpace(token) == "" {
		return domain.NewError(domain.ErrInvalidInput, label+" cannot be empty")
	}
	if _, err := uuid.Parse(token); err != nil {
		return domain.NewError(domain.ErrInvalidInput, "invalid "+label)
	}

	return nil
}
