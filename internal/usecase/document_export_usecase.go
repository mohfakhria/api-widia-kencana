package usecase

import (
	"context"
	"errors"
	"io"
	"strings"

	"github.com/mohfakhria/api-widia-kencana/internal/domain"
	"github.com/mohfakhria/api-widia-kencana/internal/domain/design"
	"github.com/mohfakhria/api-widia-kencana/internal/usecase/port/input"
	"github.com/mohfakhria/api-widia-kencana/internal/usecase/port/output"

	"github.com/google/uuid"
)

// maxImageBytes membatasi satu aset gambar yang disematkan ke dalam PDF.
//
// Ukuran objek berasal dari unggahan pengguna, jadi tanpa batas ini satu berkas
// besar dapat menghabiskan memori proses — dan dokumen dapat memuat banyak
// gambar sekaligus.
const maxImageBytes = 16 << 20

// assetStatusUploaded adalah satu-satunya keadaan aset yang isinya benar-benar
// ada di object storage.
const assetStatusUploaded = "uploaded"

type documentExportUseCase struct {
	documents output.DocumentRepository
	content   output.DocumentContentSource
	assets    output.AssetRepository
	storage   output.ObjectStorage
	renderer  output.DocumentRenderer
}

func NewDocumentExportUseCase(
	documents output.DocumentRepository,
	content output.DocumentContentSource,
	assets output.AssetRepository,
	storage output.ObjectStorage,
	renderer output.DocumentRenderer,
) input.DocumentExportUseCase {
	return &documentExportUseCase{
		documents: documents,
		content:   content,
		assets:    assets,
		storage:   storage,
		renderer:  renderer,
	}
}

func (uc *documentExportUseCase) ExportPDF(ctx context.Context, documentToken string) (*input.DocumentExportResult, error) {
	if _, err := uuid.Parse(documentToken); err != nil {
		return nil, domain.NewError(domain.ErrInvalidInput, "invalid document token")
	}

	document, err := uc.documents.GetByToken(ctx, documentToken)
	if err != nil {
		return nil, err
	}

	snapshot, err := uc.content.Snapshot(ctx, documentToken)
	if err != nil {
		return nil, err
	}

	parsed, err := design.Decode(snapshot.Content)
	if err != nil {
		return nil, err
	}

	width, height, ok := design.PaperPoints(snapshot.Paper.Width, snapshot.Paper.Height, snapshot.Paper.Unit)
	if !ok {
		return nil, domain.NewError(domain.ErrInvalidInput,
			"document paper uses an unsupported unit: "+snapshot.Paper.Unit)
	}

	images, err := uc.loadImages(ctx, parsed)
	if err != nil {
		return nil, err
	}

	rendered, err := uc.renderer.RenderPDF(ctx, output.RenderDocument{
		Content:    parsed,
		PageWidth:  width,
		PageHeight: height,
		Images:     images,
	})
	if err != nil {
		return nil, err
	}

	return &input.DocumentExportResult{
		Filename: exportFilename(document.Name),
		Content:  rendered,
		Version:  snapshot.Version,
	}, nil
}

// loadImages mengambil isi setiap aset yang dipakai elemen gambar.
//
// Aset yang sudah tidak ada atau belum selesai diunggah dilewati tanpa error:
// frontend pun tidak dapat menampilkannya, jadi melewatinya justru membuat layar
// dan cetakan tetap sama. Kegagalan mengunduh diperlakukan berbeda — itu gangguan
// sementara, dan menghasilkan PDF tanpa gambar yang seharusnya ada akan
// menyamarkannya sebagai hasil yang benar.
func (uc *documentExportUseCase) loadImages(ctx context.Context, content *design.Content) (map[string]output.RenderImage, error) {
	tokens := imageTokens(content)
	if len(tokens) == 0 {
		return nil, nil
	}

	images := make(map[string]output.RenderImage, len(tokens))
	for _, token := range tokens {
		asset, err := uc.assets.GetByToken(ctx, token)
		if err != nil {
			if errors.Is(err, domain.ErrNotFound) {
				continue
			}
			return nil, err
		}
		if asset.Status != assetStatusUploaded {
			continue
		}

		data, err := uc.downloadImage(ctx, asset.ObjectName)
		if err != nil {
			return nil, err
		}
		if data == nil {
			continue
		}

		images[token] = output.RenderImage{Data: data, MimeType: asset.MimeType}
	}

	return images, nil
}

// downloadImage mengembalikan nil tanpa error bila asetnya melebihi batas — satu
// gambar raksasa tidak sepadan dengan ekspor yang gagal seluruhnya.
func (uc *documentExportUseCase) downloadImage(ctx context.Context, objectName string) ([]byte, error) {
	reader, err := uc.storage.Download(ctx, objectName)
	if err != nil {
		return nil, domain.NewError(domain.ErrInternalFailure, "failed to read document image asset")
	}
	defer reader.Close()

	// Dibaca satu byte melebihi batas supaya kelebihannya dapat dibedakan dari
	// berkas yang kebetulan berukuran persis sebesar batas.
	data, err := io.ReadAll(io.LimitReader(reader, maxImageBytes+1))
	if err != nil {
		return nil, domain.NewError(domain.ErrInternalFailure, "failed to read document image asset")
	}
	if len(data) > maxImageBytes {
		return nil, nil
	}

	return data, nil
}

// imageTokens mengumpulkan token aset yang dipakai, tanpa pengulangan.
//
// Satu gambar yang dipakai di sepuluh halaman cukup diunduh sekali, dan renderer
// pun hanya menyematkannya sekali ke dalam berkas.
func imageTokens(content *design.Content) []string {
	seen := make(map[string]struct{})
	tokens := make([]string, 0)

	for _, page := range content.Pages {
		for _, element := range page.Elements {
			if element.Type != design.ElementImage || element.AssetToken == "" {
				continue
			}
			if _, exists := seen[element.AssetToken]; exists {
				continue
			}
			seen[element.AssetToken] = struct{}{}
			tokens = append(tokens, element.AssetToken)
		}
	}

	return tokens
}

// exportFilename menyusun nama berkas dari nama dokumen.
//
// Karakter yang punya arti khusus pada nama berkas atau pada header HTTP dibuang,
// bukan diganti dengan sandi. Nama ini hanya saran bagi browser; yang penting ia
// tidak dapat menyelipkan tanda kutip atau pemisah direktori ke dalam header
// Content-Disposition.
func exportFilename(name string) string {
	cleaned := strings.Map(func(symbol rune) rune {
		switch symbol {
		case '"', '\\', '/', ':', '*', '?', '<', '>', '|', '\r', '\n':
			return -1
		}
		if symbol < 0x20 {
			return -1
		}

		return symbol
	}, name)

	cleaned = strings.TrimSpace(cleaned)
	if cleaned == "" {
		cleaned = "document"
	}

	return cleaned + ".pdf"
}
