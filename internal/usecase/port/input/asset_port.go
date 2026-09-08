package input

import (
	"context"
	"time"

	"github.com/mohfakhria/api-widia-kencana/internal/domain/entity"
)

// AssetRef menyebut satu aset: lewat token, atau lewat key.
//
// Dua cara menyebut benda yang sama, dan itu sebabnya ia struct dan bukan dua
// parameter — setiap endpoint pembaca menerima keduanya, dan menyalin sepasang
// argumen ke seluruh antarmuka akan mengundang salah satunya terlupa.
//
// Token identitas; key nama panggilan yang boleh dilepas dan dipakai ulang.
// Karena itu operasi yang MENGHAPUS hanya menerima token: nama panggilan dapat
// berpindah pemilik setelah aset lama dibuang, dan menghapus lewat nama berarti
// menghapus benda yang keliru.
type AssetRef struct {
	Token string
	Key   string
}

type AssetUseCase interface {
	RequestUpload(ctx context.Context, cmd RequestAssetUploadCommand) (*AssetUploadRequestResult, error)
	CompleteUpload(ctx context.Context, ref AssetRef, uploadedBy *int64) (*entity.Asset, error)

	// ReplaceContent mengganti berkas di balik satu aset tanpa mengubah tokennya,
	// sehingga dokumen yang menunjuknya ikut memakai berkas baru tanpa disunting.
	ReplaceContent(ctx context.Context, cmd ReplaceAssetContentCommand) (*entity.Asset, error)
	List(ctx context.Context, query ListAssetQuery) ([]entity.Asset, error)
	Get(ctx context.Context, ref AssetRef, uploadedBy *int64) (*entity.Asset, error)
	PresignGet(ctx context.Context, ref AssetRef, uploadedBy *int64) (*AssetPresignGetResult, error)

	// ContentURL menyusun URL isi aset TANPA memeriksa siapa pemanggilnya.
	//
	// Dipakai jalur yang dituju langsung oleh tag <img>, dan tag itu tidak dapat
	// mengirim header Authorization — kredensialnya karena itu adalah token aset
	// itu sendiri. Setiap pemanggil harus sadar bahwa ia sedang membuka jalur yang
	// tidak bertanya siapa.
	ContentURL(ctx context.Context, ref AssetRef) (string, error)
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
	Ref              AssetRef
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
