package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mohfakhria/api-widia-kencana/internal/domain"
	"github.com/mohfakhria/api-widia-kencana/internal/domain/entity"
	"github.com/mohfakhria/api-widia-kencana/internal/usecase/port/input"
	"github.com/mohfakhria/api-widia-kencana/internal/usecase/port/output"

	"github.com/google/uuid"
)

const defaultDocumentStatus = "draft"

// allowedDocumentTypes adalah kosakata tertutup jenis dokumen.
//
// TIDAK ADA BAWAAN, dan itu keputusan yang diambil sadar setelah sempat ada:
// sebelumnya jenis yang tidak disebut menjadi "custom". Dengan keenam jenis di
// bawah yang semuanya dokumen bisnis bermakna, tidak ada bawaan yang jujur —
// memilih salah satunya berarti melabeli dokumen orang secara diam-diam, dan
// label yang keliru tidak pernah muncul sebagai galat di kemudian hari. Permintaan
// yang ditolak sekarang jauh lebih murah daripada dokumen salah jenis yang
// ditemukan berbulan-bulan kemudian.
//
// Bertanda hubung, bukan garis bawah. Nilainya melintas di URL dan dibaca
// manusia; nama fieldnya sendiri tetap document_type karena ia sudah dipakai
// frontend dan menggantinya memutus mereka tanpa menambah apa pun.
var allowedDocumentTypes = map[string]struct{}{
	"quotation":      {},
	"purchase-order": {},
	"bast":           {},
	"delivery-note":  {},
	"service-report": {},
	"invoice":        {},
}

// defaultDocumentPaperStatus menyaring daftar kertas ke yang benar-benar dapat
// dipakai. Lihat alasannya di ListPapers.
const defaultDocumentPaperStatus = "active"

var allowedDocumentStatuses = map[string]struct{}{
	"draft":    {},
	"active":   {},
	"inactive": {},
	"archived": {},
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
	// Ditolak, bukan diteruskan ke SQL. Sebelumnya jenis yang salah ketik
	// menghasilkan 200 berisi daftar kosong — tidak dapat dibedakan dari "memang
	// tidak ada dokumen berjenis itu" — sementara status yang salah ketik pada
	// endpoint yang sama menjawab 400. Dua filter bersaudara yang berperilaku
	// berlawanan adalah jebakan yang hanya terlihat setelah seseorang kehilangan
	// waktu karenanya.
	if query.DocumentType != "" {
		if _, ok := allowedDocumentTypes[query.DocumentType]; !ok {
			return nil, domain.NewError(domain.ErrInvalidInput, "invalid document type")
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

	// Tidak diberi bawaan. Yang kosong sampai ke validateDocument apa adanya lalu
	// ditolak di sana, supaya sebabnya terbaca oleh pemanggil.
	documentType := strings.ToLower(strings.TrimSpace(cmd.DocumentType))

	return &entity.Document{
		Paper: entity.DocumentPaper{
			Token: strings.TrimSpace(cmd.DocumentPaperToken),
		},
		ParentToken:  strings.TrimSpace(cmd.ParentToken),
		Name:         strings.TrimSpace(cmd.Name),
		DocumentType: documentType,
		Status:       status,
		Variables:    cmd.Variables,
	}
}

// Batas kantong variabel.
//
// maxDocumentVariables dan maxDocumentVariableBytes ada untuk satu sebab: tanpa
// keduanya, kantong ini pelan-pelan menjadi penyimpan isi dokumen yang kedua.
// Seseorang akan menaruh seluruh baris tabel harga di sana karena "lebih mudah
// dibaca", lalu content dan variables menyimpan hal yang sama dan mulai
// berselisih.
const (
	maxDocumentVariables     = 50
	maxDocumentVariableBytes = 16 << 10
)

// validateDocumentVariables menjaga BENTUKNYA, bukan isinya.
//
// Kuncinya sengaja tidak dibatasi kosakata: itulah gunanya kantong ini. Yang
// dijaga hanya hal-hal yang membuatnya berhenti menjadi kantong — kunci kosong,
// nilai bersarang, dan ukuran yang tak berbatas.
//
// NILAINYA HARUS SKALAR. Objek atau larik di dalamnya berarti struktur, dan
// struktur yang cukup penting untuk disimpan cukup penting pula untuk punya
// tabel. Membiarkannya menghasilkan model isi kedua yang tidak pernah divalidasi
// siapa pun.
func validateDocumentVariables(variables map[string]any) error {
	if len(variables) == 0 {
		return nil
	}
	if len(variables) > maxDocumentVariables {
		return domain.NewError(domain.ErrInvalidInput,
			fmt.Sprintf("document variables cannot exceed %d entries", maxDocumentVariables))
	}

	for key, value := range variables {
		if strings.TrimSpace(key) == "" {
			return domain.NewError(domain.ErrInvalidInput, "document variable key cannot be empty")
		}

		switch value.(type) {
		case nil, bool, float64, string:
			// Skalar JSON. float64 karena encoding/json menguraikan seluruh angka
			// ke sana, termasuk yang bulat.
		default:
			return domain.NewError(domain.ErrInvalidInput,
				fmt.Sprintf("document variable %q must be a string, number, boolean, or null", key))
		}
	}

	if err := validateNumericVariables(variables); err != nil {
		return err
	}

	encoded, err := json.Marshal(variables)
	if err != nil {
		return domain.NewError(domain.ErrInvalidInput, "document variables are not valid")
	}
	if len(encoded) > maxDocumentVariableBytes {
		return domain.NewError(domain.ErrInvalidInput,
			fmt.Sprintf("document variables cannot exceed %d bytes", maxDocumentVariableBytes))
	}

	return nil
}

// numericVariableKeys adalah kunci yang WAJIB berupa angka bila ada.
//
// Kantong ini sengaja tanpa kosakata — itu gunanya. Tetapi begitu sebuah kunci
// benar-benar DIJUMLAHKAN oleh laporan, kebebasannya berubah menjadi jebakan:
// satu dokumen yang mengirim "Rp 184.405.410" alih-alih 184405410 membuat
// SUM((variables->>'grand_total')::numeric) meledak SAAT LAPORAN DIJALANKAN —
// berbulan-bulan setelah datanya ditulis, di depan orang yang sedang menunggu
// angka.
//
// Karena itu hanya kunci yang dilaporkan yang dijaga tipenya, bukan seluruh
// kantong. Daftarnya bertambah ketika ada kunci baru yang ikut dijumlahkan.
var numericVariableKeys = map[string]struct{}{
	"grand_total": {},
}

// validateNumericVariables menegakkan tipe pada kunci yang dilaporkan.
//
// Negatif ditolak, nol tidak: dokumen bernilai nol adalah keadaan yang sah —
// penawaran gratis, atau penggantian garansi. Yang memang tidak bernilai uang
// tidak menyertakan kuncinya sama sekali; itu berbeda dari nol.
func validateNumericVariables(variables map[string]any) error {
	for key := range numericVariableKeys {
		value, ada := variables[key]
		if !ada || value == nil {
			continue
		}

		angka, ok := value.(float64)
		if !ok {
			return domain.NewError(domain.ErrInvalidInput,
				fmt.Sprintf("document variable %q must be a number", key))
		}
		if angka < 0 {
			return domain.NewError(domain.ErrInvalidInput,
				fmt.Sprintf("document variable %q cannot be negative", key))
		}
	}

	return nil
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
	// Kosong dibedakan dari salah. Keduanya ditolak, tetapi yang pertama berarti
	// pemanggil belum menyebutkannya sama sekali — dan sejak jenis menjadi wajib,
	// itu kekeliruan yang paling mungkin terjadi pada klien yang sudah ada.
	if document.DocumentType == "" {
		return domain.NewError(domain.ErrInvalidInput, "document type cannot be empty")
	}
	if _, ok := allowedDocumentTypes[document.DocumentType]; !ok {
		return domain.NewError(domain.ErrInvalidInput, "invalid document type")
	}
	if _, ok := allowedDocumentStatuses[document.Status]; !ok {
		return domain.NewError(domain.ErrInvalidInput, "invalid document status")
	}
	if err := validateDocumentVariables(document.Variables); err != nil {
		return err
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
