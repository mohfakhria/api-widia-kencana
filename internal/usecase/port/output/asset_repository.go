package output

import (
	"context"
	"time"

	"github.com/mohfakhria/api-widia-kencana/internal/domain/entity"
	"github.com/mohfakhria/api-widia-kencana/internal/usecase/port/input"
)

type AssetRepository interface {
	CreatePending(ctx context.Context, asset *entity.Asset) (*entity.Asset, error)
	GetByToken(ctx context.Context, token string) (*entity.Asset, error)
	List(ctx context.Context, query input.ListAssetQuery) ([]entity.Asset, error)
	MarkUploaded(ctx context.Context, token string, stored *StoredObject) (*entity.Asset, error)
	MarkDeleted(ctx context.Context, token string) error
	MarkFailed(ctx context.Context, token string, code string, message string) error

	// FindExpired mengambil aset yang masa unggahnya sudah lewat tetapi belum
	// pernah dilaporkan selesai, tertua lebih dulu.
	//
	// Berbatas karena pemanggilnya penyapu berkala: satu denyut mengambil
	// sebagian, denyut berikutnya melanjutkan. Tanpa batas, satu sapuan pertama
	// pada database yang sudah lama menumpuk akan menarik seluruh tunggakan
	// sekaligus.
	FindExpired(ctx context.Context, before time.Time, limit int) ([]entity.Asset, error)
}
