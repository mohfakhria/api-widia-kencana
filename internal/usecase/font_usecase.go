package usecase

import (
	"bytes"
	"context"
	"strings"

	"github.com/mohfakhria/api-widia-kencana/internal/domain"
	"github.com/mohfakhria/api-widia-kencana/internal/domain/design"
	"github.com/mohfakhria/api-widia-kencana/internal/usecase/port/input"
	"github.com/mohfakhria/api-widia-kencana/internal/usecase/port/output"
)

// fontContentType dideklarasikan saat mengunggah supaya objeknya dilayani
// dengan tipe yang benar ketika peramban mengambilnya untuk @font-face.
const fontContentType = "font/ttf"

type fontUseCase struct {
	storage   output.ObjectStorage
	source    output.FontSource
	validator output.FontValidator
}

func NewFontUseCase(
	storage output.ObjectStorage,
	source output.FontSource,
	validator output.FontValidator,
) input.FontUseCase {
	return &fontUseCase{storage: storage, source: source, validator: validator}
}

// Register mengambil tiap muka huruf, memvalidasinya, lalu menyimpannya di
// object storage pada nama yang dapat dihitung ulang.
//
// Tidak ada baris database sama sekali, dan itu inti rancangannya: nama objek
// adalah fungsi murni dari family, bobot, dan style, sehingga penyimpanan
// ITULAH indeksnya. Ekspor menyusun nama yang sama dari elemen dokumen lalu
// mengunduhnya — tidak ada tabel kedua yang harus dijaga tetap sinkron dengan
// isi bucket.
func (uc *fontUseCase) Register(ctx context.Context, cmd input.RegisterFontCommand) ([]input.FontFaceResult, error) {
	family := strings.TrimSpace(cmd.Family)
	if family == "" {
		return nil, domain.NewError(domain.ErrInvalidInput, "font family cannot be empty")
	}
	if FontFamilySlug(family) == "" {
		return nil, domain.NewError(domain.ErrInvalidInput, "font family has no usable characters")
	}
	if len(cmd.Weights) == 0 {
		return nil, domain.NewError(domain.ErrInvalidInput, "at least one font weight is required")
	}

	styles := normalizeStyles(cmd.Styles)
	hasil := make([]input.FontFaceResult, 0, len(cmd.Weights)*len(styles))

	for _, style := range styles {
		for _, weight := range cmd.Weights {
			if weight < 100 || weight > 900 || weight%100 != 0 {
				return nil, domain.NewError(domain.ErrInvalidInput,
					"font weight must be a multiple of 100 between 100 and 900")
			}

			hasil = append(hasil, uc.registerFace(ctx, family, weight, style))
		}
	}

	return hasil, nil
}

// registerFace tidak pernah mengembalikan galat: kegagalan satu muka huruf
// dilaporkan sebagai baris hasil, bukan sebagai kegagalan permintaan. Lihat
// alasannya di input.FontUseCase.
func (uc *fontUseCase) registerFace(ctx context.Context, family string, weight int, style string) input.FontFaceResult {
	hasil := input.FontFaceResult{
		Family:     family,
		Weight:     weight,
		Style:      style,
		ObjectName: FontObjectName(family, weight, style),
	}

	data, err := uc.source.Fetch(ctx, family, weight, style)
	if err != nil {
		hasil.Reason = err.Error()
		return hasil
	}

	// Divalidasi SEBELUM diunggah, bukan sesudah. Berkas rusak yang telanjur
	// tersimpan akan merusak setiap ekspor yang memakai keluarga itu, dan
	// membersihkannya menuntut orang menyadari lebih dulu bahwa ia yang bersalah.
	if err := uc.validator.Validate(data); err != nil {
		hasil.Reason = err.Error()
		return hasil
	}

	stored, err := uc.storage.Upload(ctx, output.UploadObject{
		ObjectName:  hasil.ObjectName,
		Reader:      bytes.NewReader(data),
		Size:        int64(len(data)),
		ContentType: fontContentType,
	})
	if err != nil {
		hasil.Reason = "font could not be stored"
		return hasil
	}

	hasil.Size = stored.Size
	hasil.Stored = true

	return hasil
}

// normalizeStyles memastikan tegak selalu ikut dan tidak ada style kembar.
//
// Tegak selalu ikut karena ia yang dipakai renderer ketika sebuah keluarga
// diminta italic tetapi tidak punya muka italic — mendaftarkan italic saja
// menghasilkan keluarga yang justru tidak dapat menggambar teks biasa.
func normalizeStyles(styles []string) []string {
	out := []string{design.FontStyleNormal}
	for _, style := range styles {
		if strings.EqualFold(strings.TrimSpace(style), design.FontStyleItalic) {
			return append(out, design.FontStyleItalic)
		}
	}

	return out
}
