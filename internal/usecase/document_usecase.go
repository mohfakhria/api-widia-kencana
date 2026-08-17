package usecase

import (
	"context"
	"strings"

	"github.com/mohfakhria/api-widia-kencana/internal/domain"
	"github.com/mohfakhria/api-widia-kencana/internal/domain/entity"
	"github.com/mohfakhria/api-widia-kencana/internal/usecase/port/input"
	"github.com/mohfakhria/api-widia-kencana/internal/usecase/port/output"

	"github.com/google/uuid"
)

const defaultDocumentStatus = "draft"
const defaultDocumentType = "custom"

// defaultDocumentPaperStatus menyaring daftar kertas ke yang benar-benar dapat
// dipakai. Lihat alasannya di ListPapers.
const defaultDocumentPaperStatus = "active"

var allowedDocumentStatuses = map[string]struct{}{
	"draft":    {},
	"active":   {},
	"inactive": {},
	"archived": {},
	"deleted":  {},
}

type documentUseCase struct {
	repo output.DocumentRepository
}

func NewDocumentUseCase(repo output.DocumentRepository) input.DocumentUseCase {
	return &documentUseCase{repo: repo}
}

func (uc *documentUseCase) List(ctx context.Context, query input.ListDocumentQuery) ([]entity.Document, error) {
	query.Name = strings.TrimSpace(query.Name)
	query.Token = strings.TrimSpace(query.Token)
	query.DocumentType = strings.ToLower(strings.TrimSpace(query.DocumentType))
	query.Status = strings.ToLower(strings.TrimSpace(query.Status))

	if err := validateOptionalUUIDToken(query.Token, "document token"); err != nil {
		return nil, err
	}
	if query.Status != "" {
		if _, ok := allowedDocumentStatuses[query.Status]; !ok {
			return nil, domain.NewError(domain.ErrInvalidInput, "invalid document status")
		}
	}

	return uc.repo.List(ctx, query)
}

// ListPapers mengembalikan kertas yang boleh dipakai dokumen baru.
//
// Bawaannya "active", BUKAN semua. Penukaran token menjadi id saat membuat
// dokumen sudah menuntut status active — kertas non-aktif ditolak
// "document paper not found" — jadi daftar yang memuatnya akan menawarkan
// pilihan yang pasti gagal begitu dipilih.
//
// Statusnya tetap dapat disebut eksplisit, memakai kosakata yang sama dengan
// status dokumen supaya tidak ada dua daftar nilai sah yang harus diingat.
func (uc *documentUseCase) ListPapers(ctx context.Context, query input.ListDocumentPaperQuery) ([]entity.DocumentPaper, error) {
	query.Status = strings.ToLower(strings.TrimSpace(query.Status))
	if query.Status == "" {
		query.Status = defaultDocumentPaperStatus
	}
	if _, ok := allowedDocumentStatuses[query.Status]; !ok {
		return nil, domain.NewError(domain.ErrInvalidInput, "invalid document paper status")
	}

	return uc.repo.ListPapers(ctx, query)
}

func (uc *documentUseCase) GetByToken(ctx context.Context, token string) (*entity.Document, error) {
	token = strings.TrimSpace(token)
	if err := validateUUIDToken(token, "document token"); err != nil {
		return nil, err
	}

	return uc.repo.GetByToken(ctx, token)
}

func (uc *documentUseCase) Create(ctx context.Context, cmd input.CreateDocumentCommand) (*entity.Document, error) {
	document := mapDocumentCommand(cmd)
	if err := validateDocument(document); err != nil {
		return nil, err
	}

	return uc.repo.Create(ctx, document)
}

func (uc *documentUseCase) Update(ctx context.Context, token string, cmd input.UpdateDocumentCommand) error {
	token = strings.TrimSpace(token)
	if err := validateUUIDToken(token, "document token"); err != nil {
		return err
	}

	document := mapDocumentCommand(input.CreateDocumentCommand(cmd))
	if err := validateDocument(document); err != nil {
		return err
	}

	return uc.repo.Update(ctx, token, document)
}

func (uc *documentUseCase) Delete(ctx context.Context, token string) error {
	token = strings.TrimSpace(token)
	if err := validateUUIDToken(token, "document token"); err != nil {
		return err
	}

	return uc.repo.Delete(ctx, token)
}

func mapDocumentCommand(cmd input.CreateDocumentCommand) *entity.Document {
	status := strings.ToLower(strings.TrimSpace(cmd.Status))
	if status == "" {
		status = defaultDocumentStatus
	}

	documentType := strings.ToLower(strings.TrimSpace(cmd.DocumentType))
	if documentType == "" {
		documentType = defaultDocumentType
	}

	return &entity.Document{
		Paper: entity.DocumentPaper{
			Token: strings.TrimSpace(cmd.DocumentPaperToken),
		},
		ParentToken:  strings.TrimSpace(cmd.ParentToken),
		Name:         strings.TrimSpace(cmd.Name),
		DocumentType: documentType,
		Status:       status,
	}
}

func validateDocument(document *entity.Document) error {
	if err := validateUUIDToken(document.Paper.Token, "document paper token"); err != nil {
		return err
	}
	if err := validateOptionalUUIDToken(document.ParentToken, "parent token"); err != nil {
		return err
	}
	if document.Name == "" {
		return domain.NewError(domain.ErrInvalidInput, "document name cannot be empty")
	}
	if _, ok := allowedDocumentStatuses[document.Status]; !ok {
		return domain.NewError(domain.ErrInvalidInput, "invalid document status")
	}

	return nil
}

func validateUUIDToken(token, label string) error {
	if strings.TrimSpace(token) == "" {
		return domain.NewError(domain.ErrInvalidInput, label+" cannot be empty")
	}
	if _, err := uuid.Parse(token); err != nil {
		return domain.NewError(domain.ErrInvalidInput, "invalid "+label)
	}

	return nil
}

func validateOptionalUUIDToken(token, label string) error {
	if strings.TrimSpace(token) == "" {
		return nil
	}

	return validateUUIDToken(token, label)
}
