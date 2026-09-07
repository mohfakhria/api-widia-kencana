package input

import (
	"context"
	"time"

	"github.com/mohfakhria/api-widia-kencana/internal/domain/entity"
)

type AssetUseCase interface {
	RequestUpload(ctx context.Context, cmd RequestAssetUploadCommand) (*AssetUploadRequestResult, error)
	CompleteUpload(ctx context.Context, token string, uploadedBy *int64) (*entity.Asset, error)

	// ReplaceContent mengganti berkas di balik satu aset tanpa mengubah tokennya,
	// sehingga dokumen yang menunjuknya ikut memakai berkas baru tanpa disunting.
	ReplaceContent(ctx context.Context, cmd ReplaceAssetContentCommand) (*entity.Asset, error)
	List(ctx context.Context, query ListAssetQuery) ([]entity.Asset, error)
	GetByToken(ctx context.Context, token string, uploadedBy *int64) (*entity.Asset, error)
	PresignGet(ctx context.Context, token string, uploadedBy *int64) (*AssetPresignGetResult, error)

	// ContentURL menyusun URL isi aset TANPA memeriksa siapa pemanggilnya.
	//
	// Dipakai jalur yang dituju langsung oleh tag <img>, dan tag itu tidak dapat
	// mengirim header Authorization — kredensialnya karena itu adalah token aset
	// itu sendiri. Setiap pemanggil harus sadar bahwa ia sedang membuka jalur yang
	// tidak bertanya siapa.
	ContentURL(ctx context.Context, token string) (string, error)
	Delete(ctx context.Context, token string, uploadedBy *int64) error
}

type RequestAssetUploadCommand struct {
	OriginalFilename string
	MimeType         string
	Size             int64
	// Group adalah kelompok sekaligus folder tujuannya — images/brand,
	// documents/quotation. Wajib, dari kosakata tertutup; tidak ada bawaan.
	Group string
	// Key opsional: nama slot yang isinya dapat diganti tanpa mengubah token.
	Key        string
	UploadedBy *int64
}

// ReplaceAssetContentCommand mengganti ISI sebuah aset, bukan identitasnya.
//
// Token, key, dan kelompoknya tidak berubah — dan itu seluruh gunanya: setiap
// dokumen yang menunjuk token itu ikut memakai berkas yang baru tanpa disunting
// satu per satu.
//
// Isinya dibawa langsung, bukan lewat presigned URL seperti unggahan biasa.
// Lihat alasannya di assetUseCase.ReplaceContent.
type ReplaceAssetContentCommand struct {
	Token            string
	OriginalFilename string
	MimeType         string
	Content          []byte
}

type AssetUploadRequestResult struct {
	Asset     *entity.Asset
	UploadURL string
	ExpiresAt time.Time
}

type ListAssetQuery struct {
	Status string
	Group  string
	// Key mencari satu slot. Unik di antara aset yang hidup, jadi hasilnya nol
	// atau satu — inilah cara menemukan aset TANPA tahu tokennya.
	Key        string
	MimeType   string
	Extension  string
	UploadedBy *int64
}

type AssetPresignGetResult struct {
	Asset     *entity.Asset
	URL       string
	ExpiresIn int64
}
