package input

import (
	"context"

	"github.com/mohfakhria/api-widia-kencana/internal/domain/entity"
)

type DocumentUseCase interface {
	List(ctx context.Context, query ListDocumentQuery) ([]entity.Document, error)

	// ListPapers ada di antarmuka dokumen, bukan antarmukanya sendiri, karena
	// kertas tidak berdiri sendiri: ia tidak dapat dibuat, diubah, maupun
	// dihapus lewat API, dan satu-satunya alasan ia perlu dibaca adalah untuk
	// mengisi document_paper_token saat membuat dokumen.
	ListPapers(ctx context.Context, query ListDocumentPaperQuery) ([]entity.DocumentPaper, error)
	GetByToken(ctx context.Context, token string) (*entity.Document, error)
	Create(ctx context.Context, cmd CreateDocumentCommand) (*entity.Document, error)
	Update(ctx context.Context, token string, cmd UpdateDocumentCommand) error
	Delete(ctx context.Context, token string) error
}

type ListDocumentQuery struct {
	Name         string
	Token        string
	DocumentType string
	Status       string
}

// ListDocumentPaperQuery menyaring daftar kertas.
//
// Hanya Status, dan itu bukan kekurangan: tabelnya belasan baris dan tidak
// tumbuh oleh pemakaian. Menyaring nama atau jenis media di server hanya
// memindahkan pekerjaan yang lebih baik dilakukan di layar, tempat pengguna
// melihat seluruh pilihannya sekaligus.
type ListDocumentPaperQuery struct {
	Status string
}

type CreateDocumentCommand struct {
	DocumentPaperToken string
	ParentToken        string
	Name               string
	DocumentType       string
	Status             string
}

type UpdateDocumentCommand = CreateDocumentCommand
