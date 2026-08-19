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

// Batas font untuk satu ekspor.
//
// maxFontFaceBytes menjaga satu objek yang mengada-ada; pendaftaran lewat API
// sudah menolak yang lebih besar, jadi ia hanya berlaku bagi berkas yang ditaruh
// langsung ke dalam bucket. maxFontTotalBytes membatasi keseluruhannya, karena
// yang diunduh SELURUH keluarga yang dipakai — dokumen yang menyebut sepuluh
// keluarga berbobot lengkap dapat menarik puluhan megabita ke memori sekaligus.
const (
	maxFontFaceBytes  = 5 << 20
	maxFontTotalBytes = 48 << 20
)

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

	fonts, err := uc.loadFonts(ctx, parsed)
	if err != nil {
		return nil, err
	}

	rendered, err := uc.renderer.RenderPDF(ctx, output.RenderDocument{
		Token:      documentToken,
		Content:    parsed,
		PageWidth:  width,
		PageHeight: height,
		Images:     images,
		Fonts:      fonts,
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

// fontRequest adalah satu muka huruf yang diminta isi dokumen.
type fontRequest struct {
	weight int
	style  string
}

// loadFonts mengambil berkas font bagi muka huruf yang dipakai dokumen.
//
// Dua jalur, dan yang menentukan adalah apakah SELURUH permintaan pada satu
// keluarga dapat dipenuhi persis:
//
//   - Semua ada → yang diunduh hanya itu. Dokumen yang memakai satu muka huruf
//     menarik satu berkas, bukan delapan belas.
//   - Ada yang meleset → SELURUH keluarga diunduh. Renderer menyelesaikan bobot
//     yang tidak ada dengan memilih yang terdekat di dalam keluarga yang sama,
//     dan pilihan itu hanya benar bila ia melihat seluruh isinya; diberi
//     sepotong, ia jatuh ke Helvetica padahal keluarganya masih punya bobot
//     berdekatan — cetakan yang berbeda dari layar justru karena kita berhemat.
//
// Hasil akhirnya sama pada kedua jalur. Yang berbeda hanya berapa berkas yang
// menyeberang jaringan, dan itu terasa: ekspor adalah satu klik yang ditunggu
// orang.
//
// Keluarga yang tidak terpasang menghasilkan daftar kosong, dan itu BUKAN galat:
// resolve memang dirancang tidak pernah gagal, dan menolak mencetak dokumen yang
// menyebut font tak dikenal adalah keputusan yang sudah pernah dibalik. Yang
// menjadi galat hanya kegagalan penyimpanan — itu gangguan sementara, dan PDF
// ber-Helvetica yang lahir darinya menyamar sebagai hasil yang benar.
func (uc *documentExportUseCase) loadFonts(ctx context.Context, content *design.Content) (map[output.FontFace][]byte, error) {
	requested := requestedFaces(content)
	if len(requested) == 0 {
		return nil, nil
	}

	fonts := make(map[output.FontFace][]byte)
	total := 0

	for family, wanted := range requested {
		slug := FontFamilySlug(family)
		if slug == "" {
			continue
		}

		objects, err := uc.storage.List(ctx, FontScope+"/"+slug+"/")
		if err != nil {
			return nil, domain.NewError(domain.ErrInternalFailure, "failed to read document export fonts")
		}

		tersedia := make(map[fontRequest]string, len(objects))
		for _, object := range objects {
			if _, weight, style, ok := ParseFontObjectName(object.ObjectName); ok {
				tersedia[fontRequest{weight: weight, style: style}] = object.ObjectName
			}
		}

		unduh := make(map[fontRequest]string, len(wanted))
		for request := range wanted {
			objectName, ada := tersedia[request]
			if !ada {
				// Satu yang meleset sudah cukup: yang menambalnya adalah bobot
				// lain di keluarga yang sama, dan kita belum tahu yang mana.
				unduh = tersedia

				break
			}
			unduh[request] = objectName
		}

		for request, objectName := range unduh {
			data, err := uc.downloadFont(ctx, objectName)
			if err != nil {
				return nil, err
			}
			if data == nil {
				continue
			}

			total += len(data)
			if total > maxFontTotalBytes {
				return nil, domain.NewError(domain.ErrInvalidInput,
					"document uses more font files than the export limit allows")
			}

			// Dikunci nama keluarga yang DIPAKAI ELEMEN, bukan slug-nya. Renderer
			// mencari dengan nama yang dibawa elemen — "barlow condensed" lengkap
			// dengan spasinya — dan slug hanya jembatan menuju nama objek.
			fonts[output.FontFace{Family: family, Weight: request.weight, Style: request.style}] = data
		}
	}

	return fonts, nil
}

// requestedFaces mengumpulkan muka huruf yang diminta, dikelompokkan per keluarga.
//
// Keluarga inti dilewati: metriknya melekat pada spesifikasi PDF dan tidak ada
// berkas yang perlu diambil untuknya.
//
// Sel tabel tidak menyimpan keluarga maupun style sendiri — hanya bobot — jadi
// keluarga dan style elemennya berlaku bagi seluruh selnya, sementara bobot tiap
// sel ikut diminta sendiri-sendiri.
func requestedFaces(content *design.Content) map[string]map[fontRequest]struct{} {
	requested := make(map[string]map[fontRequest]struct{})

	tambah := func(family string, weight int, style string) {
		if requested[family] == nil {
			requested[family] = make(map[fontRequest]struct{})
		}
		requested[family][fontRequest{weight: weight, style: style}] = struct{}{}
	}

	kumpulkan := func(elements []design.Element) {
		for index := range elements {
			element := &elements[index]
			if element.Type != design.ElementText && element.Type != design.ElementTable {
				continue
			}

			family := element.ResolvedFontFamily()
			if family == design.DefaultFontFamily {
				continue
			}

			style := element.ResolvedFontStyle()
			tambah(family, element.ResolvedFontWeight(), style)

			for _, row := range element.Rows {
				for _, cell := range row.Cells {
					tambah(family, cell.ResolvedCellWeight(), style)
				}
			}
		}
	}

	for _, page := range content.VisiblePages() {
		kumpulkan(page.Elements)
	}

	// Lapisan master digambar di SETIAP halaman, jadi fontnya sama wajibnya.
	// Melewatkannya menghasilkan kop surat ber-Helvetica di atas badan dokumen
	// yang hurufnya benar — persis satu-satunya tempat yang paling terlihat.
	kumpulkan(content.Master.Elements)

	return requested
}

// downloadFont mengembalikan nil tanpa error bila berkasnya melebihi batas, sama
// seperti gambar: satu objek yang mengada-ada tidak sepadan dengan ekspor yang
// gagal seluruhnya, dan renderer masih punya jalan mundur.
func (uc *documentExportUseCase) downloadFont(ctx context.Context, objectName string) ([]byte, error) {
	reader, err := uc.storage.Download(ctx, objectName)
	if err != nil {
		return nil, domain.NewError(domain.ErrInternalFailure, "failed to read document export fonts")
	}
	defer reader.Close()

	data, err := io.ReadAll(io.LimitReader(reader, maxFontFaceBytes+1))
	if err != nil {
		return nil, domain.NewError(domain.ErrInternalFailure, "failed to read document export fonts")
	}
	if len(data) > maxFontFaceBytes {
		return nil, nil
	}

	return data, nil
}

// fontFamilies mengumpulkan keluarga font yang dipakai, tanpa pengulangan.
//
// Keluarga inti dilewati: metriknya melekat pada spesifikasi PDF dan tidak ada
// berkas yang perlu diambil untuknya.
//
// Sel tabel tidak menyimpan keluarga sendiri — hanya bobot per sel — sehingga
// keluarga elemen tabel sudah mewakili seluruh selnya.
func fontFamilies(content *design.Content) []string {
	seen := make(map[string]struct{})
	families := make([]string, 0)

	kumpulkan := func(elements []design.Element) {
		for index := range elements {
			element := &elements[index]
			if element.Type != design.ElementText && element.Type != design.ElementTable {
				continue
			}

			family := element.ResolvedFontFamily()
			if family == design.DefaultFontFamily {
				continue
			}
			if _, exists := seen[family]; exists {
				continue
			}
			seen[family] = struct{}{}
			families = append(families, family)
		}
	}

	for _, page := range content.VisiblePages() {
		kumpulkan(page.Elements)
	}

	// Lapisan master digambar di SETIAP halaman, jadi fontnya sama wajibnya.
	// Melewatkannya menghasilkan kop surat ber-Helvetica di atas badan dokumen
	// yang hurufnya benar — persis satu-satunya tempat yang paling terlihat.
	kumpulkan(content.Master.Elements)

	return families
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

	kumpulkan := func(elements []design.Element) {
		for _, element := range elements {
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

	// Halaman tersembunyi dilewati, dan itu bukan sekadar penghematan: kegagalan
	// mengunduh satu aset menggagalkan SELURUH ekspor, sehingga gambar rusak di
	// halaman yang bahkan tidak dicetak akan membatalkan dokumen yang sebenarnya
	// baik-baik saja.
	for _, page := range content.VisiblePages() {
		kumpulkan(page.Elements)
	}

	// Lapisan master ikut, dengan alasan yang sama seperti pada fontFamilies: ia
	// digambar di setiap halaman. Sebelumnya ia terlewat, dan akibatnya logo di
	// master hilang dari cetakan tanpa satu pun galat.
	kumpulkan(content.Master.Elements)

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
