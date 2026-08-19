package usecase

import (
	"archive/zip"
	"bytes"
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"path"
	"slices"
	"strings"
	"time"

	"github.com/mohfakhria/api-widia-kencana/internal/domain"
	"github.com/mohfakhria/api-widia-kencana/internal/usecase/port/input"
	"github.com/mohfakhria/api-widia-kencana/internal/usecase/port/output"
)

// fontContentType dideklarasikan saat mengunggah supaya objeknya dilayani
// dengan tipe yang benar ketika peramban mengambilnya untuk @font-face.
const fontContentType = "font/ttf"

// fontContentExpiry adalah umur alamat presigned yang dilayani font-content.
//
// Pendek, dan itu tidak mengganggu: yang dipegang peramban adalah rute kami yang
// permanen, dan ia mengikuti pengalihan baru setiap kali benar-benar mengambil
// berkasnya.
const fontContentExpiry = 15 * time.Minute

type fontUseCase struct {
	storage   output.ObjectStorage
	inspector output.FontInspector
	logger    *slog.Logger
}

func NewFontUseCase(
	storage output.ObjectStorage,
	inspector output.FontInspector,
	logger *slog.Logger,
) input.FontUseCase {
	if logger == nil {
		logger = slog.Default()
	}

	return &fontUseCase{storage: storage, inspector: inspector, logger: logger}
}

// maxArchiveEntries dan maxFontFileBytes membatasi arsip yang mau dibuka.
//
// Arsip adalah masukan tak tepercaya, dan yang berbahaya bukan ukuran berkasnya
// melainkan ukuran ISINYA: satu ZIP kecil dapat mengembang menjadi berkali lipat
// saat dibuka. Karena itu batasnya diterapkan saat MEMBACA tiap entri, bukan
// dipercaya dari header arsip.
const (
	maxArchiveEntries = 200
	maxFontFileBytes  = 5 << 20
)

// Register membuka arsip lalu menyimpan setiap berkas font yang sah di dalamnya.
//
// Jati diri tiap muka huruf — keluarga, bobot, style — dibaca DARI BERKASNYA,
// bukan dari nama berkas maupun dari masukan pengguna. Nama berkas tidak cukup:
// "BarlowCondensed-Bold.ttf" menghasilkan slug "barlowcondensed", sedangkan
// elemen dokumen mengirim "barlow condensed" yang menghasilkan
// "barlow-condensed", dan keduanya tidak akan pernah bertemu.
//
// Tidak ada baris database sama sekali, dan itu inti rancangannya: nama objek
// adalah fungsi murni dari family, bobot, dan style, sehingga penyimpanan
// ITULAH indeksnya. Ekspor menyusun nama yang sama dari elemen dokumen lalu
// mengunduhnya — tidak ada tabel kedua yang harus dijaga tetap sinkron dengan
// isi bucket.
func (uc *fontUseCase) Register(ctx context.Context, cmd input.RegisterFontCommand) ([]input.FontFaceResult, error) {
	if len(cmd.Archive) == 0 {
		return nil, domain.NewError(domain.ErrInvalidInput, "font archive is empty")
	}

	arsip, err := zip.NewReader(bytes.NewReader(cmd.Archive), int64(len(cmd.Archive)))
	if err != nil {
		return nil, domain.NewError(domain.ErrInvalidInput, "font archive is not a valid zip file")
	}
	if len(arsip.File) > maxArchiveEntries {
		return nil, domain.NewError(domain.ErrInvalidInput, "font archive has too many entries")
	}

	hasil := make([]input.FontFaceResult, 0, len(arsip.File))

	// Nama objek yang sudah terpakai DI DALAM arsip ini. Bobot diturunkan dari
	// subfamily, dan subfamily yang tak dikenal jatuh ke 400; dua berkas yang
	// sama-sama jatuh ke sana akan menempati nama objek yang sama, dan yang
	// terakhir menimpa yang pertama tanpa sepatah kata pun. Yang dilaporkan
	// bukan kegagalan menyimpan melainkan bahwa keduanya bertabrakan.
	terpakai := make(map[string]string, len(arsip.File))

	for _, entri := range arsip.File {
		// Unggahan besar berisi seratus lebih muka huruf, dan tiap satunya satu
		// perjalanan ke penyimpanan. Bila peramban sudah pergi, meneruskannya
		// hanya membakar waktu untuk jawaban yang tidak akan dibaca siapa pun.
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		if lewati, alasan := skipEntry(entri.Name); lewati {
			// Entri yang bukan font dilaporkan, tidak didiamkan. Arsip Google
			// memuat OFL.txt dan README.txt yang memang bukan font, dan pengunggah
			// berhak tahu bahwa keduanya memang sengaja tidak masuk.
			if alasan != "" {
				hasil = append(hasil, input.FontFaceResult{Entry: entri.Name, Reason: alasan})
			}

			continue
		}

		baris := uc.registerEntry(ctx, entri, terpakai)
		if baris.Stored {
			terpakai[baris.ObjectName] = entri.Name
		}

		hasil = append(hasil, baris)
	}

	if len(hasil) == 0 {
		return nil, domain.NewError(domain.ErrInvalidInput, "font archive contains no font files")
	}

	return hasil, nil
}

// skipEntry menjawab apakah satu entri arsip perlu dilewati, beserta sebabnya.
//
// Alasan kosong berarti dilewati TANPA dilaporkan — direktori dan berkas sistem
// macOS tidak menarik bagi siapa pun.
func skipEntry(name string) (lewati bool, alasan string) {
	base := path.Base(name)

	switch {
	case strings.HasSuffix(name, "/"), strings.HasPrefix(base, "."), strings.Contains(name, "__MACOSX/"):
		return true, ""
	case strings.EqualFold(path.Ext(base), ".ttf"):
		// Font variabel TIDAK disaring di sini. Ia dikenali dari tabel fvar di
		// dalam berkasnya oleh FontInspector — nama berkas bukan fakta tentang
		// isinya, dan arsip yang berkasnya dinamai ulang akan lolos dari
		// penyaring nama apa pun.
		return false, ""
	default:
		return true, "not a .ttf file"
	}
}

// registerEntry membaca satu entri, memeriksanya, lalu menyimpannya.
//
// Tidak pernah mengembalikan galat: kegagalan satu berkas dilaporkan sebagai
// baris hasil. Lihat alasannya di input.FontUseCase.
func (uc *fontUseCase) registerEntry(
	ctx context.Context,
	entri *zip.File,
	terpakai map[string]string,
) input.FontFaceResult {
	hasil := input.FontFaceResult{Entry: entri.Name}

	data, err := readArchiveFile(entri)
	if err != nil {
		hasil.Reason = err.Error()
		return hasil
	}

	identity, err := uc.inspector.Inspect(data)
	if err != nil {
		hasil.Reason = err.Error()
		return hasil
	}

	// Keluarga yang slug-nya kosong — nama beraksara non-Latin seluruhnya —
	// menghasilkan nama objek bersegmen kosong. Ia terunggah dengan patuh lalu
	// TIDAK PERNAH muncul lagi: penguraian nama objek menolaknya, sehingga ia
	// hilang dari daftar font maupun dari jalur pengambilan, selamanya.
	if FontFamilySlug(identity.Family) == "" {
		hasil.Family = identity.Family
		hasil.Reason = "font family name has no usable characters"

		return hasil
	}

	hasil.Family = identity.Family
	hasil.Weight = identity.Weight
	hasil.Style = identity.Style
	hasil.ObjectName = FontObjectName(identity.Family, identity.Weight, identity.Style)

	if sebelumnya, bertabrakan := terpakai[hasil.ObjectName]; bertabrakan {
		hasil.Reason = fmt.Sprintf("same family, weight and style as %s in this archive", sebelumnya)
		return hasil
	}

	stored, err := uc.storage.Upload(ctx, output.UploadObject{
		ObjectName:  hasil.ObjectName,
		Reader:      bytes.NewReader(data),
		Size:        int64(len(data)),
		ContentType: fontContentType,
	})
	if err != nil {
		// Dua pembaca, dua kebutuhan yang berbeda. Pengunggah menerima kalimat
		// tumpul karena sebab aslinya menyebut isi perut penyimpanan; operator
		// menerima sebab itu utuh — tanpanya, penyimpanan yang mati menghasilkan
		// seratus baris seragam yang tidak menunjuk ke mana pun.
		uc.logger.Error("store font face",
			"object", hasil.ObjectName, "entry", entri.Name, "error", err)

		hasil.Reason = "font could not be stored"

		return hasil
	}

	hasil.Size = stored.Size
	hasil.Stored = true

	return hasil
}

// readArchiveFile membaca satu entri dengan batas yang ditegakkan saat membaca.
//
// LimitReader dipasang satu byte di atas batas supaya kelebihannya terdeteksi,
// bukan terpotong diam-diam menjadi berkas yang tampak sah tetapi cacat.
func readArchiveFile(entri *zip.File) ([]byte, error) {
	reader, err := entri.Open()
	if err != nil {
		return nil, errors.New("archive entry could not be opened")
	}
	defer reader.Close()

	data, err := io.ReadAll(io.LimitReader(reader, maxFontFileBytes+1))
	if err != nil {
		return nil, errors.New("archive entry could not be read")
	}
	if len(data) > maxFontFileBytes {
		return nil, fmt.Errorf("font file is larger than %d bytes", maxFontFileBytes)
	}

	return data, nil
}

// FontContentPath adalah alamat stabil yang melayani satu muka huruf.
//
// Bentuknya mencerminkan nama objeknya supaya keduanya dapat dibaca berdampingan
// tanpa penerjemah di tengah.
func FontContentPath(family string, weight int, style string) string {
	return fmt.Sprintf("/api/font-content/%s/%d-%s.ttf", family, weight, style)
}

// List membaca isi object storage lalu mengelompokkannya per keluarga.
//
// Objek yang namanya tidak dapat diurai DILEWATI, bukan menggagalkan daftar.
// Bucket dapat memuat berkas yang ditaruh tangan manusia, dan satu nama yang
// salah bentuk tidak sepadan dengan seluruh daftar font yang hilang dari editor.
func (uc *fontUseCase) List(ctx context.Context) ([]input.FontFamilyListing, error) {
	objects, err := uc.storage.List(ctx, FontScope+"/")
	if err != nil {
		uc.logger.Error("list font objects", "prefix", FontScope+"/", "error", err)

		return nil, domain.NewError(domain.ErrInternalFailure, "font list could not be read")
	}

	perFamily := make(map[string][]input.FontFaceListing)
	for _, object := range objects {
		family, weight, style, ok := ParseFontObjectName(object.ObjectName)
		if !ok {
			continue
		}

		perFamily[family] = append(perFamily[family], input.FontFaceListing{
			Weight:      weight,
			Style:       style,
			Size:        object.Size,
			ContentPath: FontContentPath(family, weight, style),
		})
	}

	// Diurutkan supaya balasan yang sama menghasilkan byte yang sama: iterasi
	// peta di Go acak, dan daftar yang berubah urutan tiap panggilan membuat
	// selisih di sisi klien sulit dibaca.
	families := make([]input.FontFamilyListing, 0, len(perFamily))
	for family, faces := range perFamily {
		slices.SortFunc(faces, func(a, b input.FontFaceListing) int {
			if a.Weight != b.Weight {
				return cmp.Compare(a.Weight, b.Weight)
			}

			return cmp.Compare(a.Style, b.Style)
		})
		families = append(families, input.FontFamilyListing{Family: family, Faces: faces})
	}
	slices.SortFunc(families, func(a, b input.FontFamilyListing) int {
		return cmp.Compare(a.Family, b.Family)
	})

	return families, nil
}

// Content mengembalikan alamat sementara untuk mengambil satu muka huruf.
//
// Presign dibuat SETIAP permintaan, bukan disimpan: yang stabil adalah rute yang
// dipegang peramban, sedangkan sasarannya boleh berumur pendek. Pola yang sama
// dipakai asset-content.
func (uc *fontUseCase) Content(ctx context.Context, family string, weight int, style string) (string, error) {
	objectName := FontObjectName(family, weight, style)
	if _, err := uc.storage.Stat(ctx, objectName); err != nil {
		return "", domain.NewError(domain.ErrNotFound, "font face is not installed")
	}

	return uc.storage.PresignGet(ctx, objectName, fontContentExpiry)
}
