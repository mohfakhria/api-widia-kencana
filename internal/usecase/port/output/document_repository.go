package output

import (
	"context"
	"encoding/json"

	"github.com/mohfakhria/api-widia-kencana/internal/domain/entity"
	"github.com/mohfakhria/api-widia-kencana/internal/usecase/port/input"
)

type DocumentRepository interface {
	List(ctx context.Context, query input.ListDocumentQuery) ([]entity.Document, error)
	ListPapers(ctx context.Context, query input.ListDocumentPaperQuery) ([]entity.DocumentPaper, error)
	GetByToken(ctx context.Context, token string) (*entity.Document, error)
	Create(ctx context.Context, document *entity.Document) (*entity.Document, error)
	Update(ctx context.Context, token string, document *entity.Document) error
	Delete(ctx context.Context, token string) error

	// GetContent dan SaveContent melayani isi kanvas, terpisah dari metadata
	// dokumen supaya content yang berukuran besar tidak ikut terbaca oleh List
	// maupun GetByToken.
	//
	// SaveContent hanya berhasil bila content_version di database masih sama
	// dengan fromVersion, lalu menyetelnya ke toVersion. Kegagalannya dibedakan:
	// ErrNotFound berarti dokumennya hilang atau sudah dihapus, ErrConflict
	// berarti ada penulis lain — keduanya permanen bagi pemanggil, sedangkan
	// error lain bersifat sementara dan layak dicoba lagi.
	GetContent(ctx context.Context, token string) (*entity.DocumentContent, error)
	SaveContent(ctx context.Context, token string, content json.RawMessage, fromVersion, toVersion int64) error
}
