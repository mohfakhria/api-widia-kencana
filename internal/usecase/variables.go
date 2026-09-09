package usecase

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mohfakhria/api-widia-kencana/internal/domain"
)

// maxVariables dan maxVariableBytes ada untuk satu sebab: tanpa keduanya,
// kantong ini pelan-pelan menjadi penyimpan isi dokumen yang kedua. Seseorang
// akan menaruh seluruh baris tabel harga di sana karena "lebih mudah dibaca",
// lalu content dan variables menyimpan hal yang sama dan mulai berselisih.
const (
	maxVariables     = 50
	maxVariableBytes = 16 << 10
)

// variableRules menjaga sebuah kantong JSONB — documents.variables maupun
// projects.variables.
//
// SATU penjaga untuk keduanya, dan itu sebabnya bentuk kedua kantong sengaja
// dibuat sama. Dua penjaga yang menjelaskan hal yang sama akan berselisih pada
// perubahan berikutnya, dan yang tertinggal adalah yang tidak sedang dibaca
// orang saat itu.
type variableRules struct {
	// label muncul di pesan galat: "document variable ..." atau "project
	// variable ...". Pemanggil perlu tahu kantong yang mana yang ia salah isi.
	label string

	// numericKeys adalah kunci yang WAJIB berupa angka bila ada.
	//
	// Kantong ini sengaja tanpa kosakata — itu gunanya. Tetapi begitu sebuah
	// kunci benar-benar DIJUMLAHKAN oleh laporan, kebebasannya berubah menjadi
	// jebakan. Indeks ekspresi atas kunci itu sudah menolak nilai bertipe salah
	// saat baris DITULIS, tetapi galatnya galat Postgres mentah yang sampai ke
	// klien sebagai 500 menyebut nama indeks — dan NEGATIF lolos begitu saja,
	// karena bagi database ia angka yang sah.
	numericKeys map[string]struct{}
}

var documentVariableRules = variableRules{
	label:       "document",
	numericKeys: map[string]struct{}{"grand_total": {}},
}

// projectVariableRules menjaga kantong pada proyek. project_value ada di
// dalamnya karena nilai proyek DIISI ORANG, bukan diturunkan dari dokumen —
// alasan lengkapnya di migration/projects.sql.
var projectVariableRules = variableRules{
	label:       "project",
	numericKeys: map[string]struct{}{"project_value": {}},
}

// validate menjaga BENTUKNYA, bukan isinya.
//
// Kuncinya sengaja tidak dibatasi kosakata: itulah gunanya kantong ini. Yang
// dijaga hanya hal-hal yang membuatnya berhenti menjadi kantong — kunci kosong,
// nilai bersarang, dan ukuran yang tak berbatas.
//
// NILAINYA HARUS SKALAR. Objek atau larik di dalamnya berarti struktur, dan
// struktur yang cukup penting untuk disimpan cukup penting pula untuk punya
// tabel. Membiarkannya menghasilkan model isi kedua yang tidak pernah divalidasi
// siapa pun.
func (r variableRules) validate(variables map[string]any) error {
	if len(variables) == 0 {
		return nil
	}
	if len(variables) > maxVariables {
		return domain.NewError(domain.ErrInvalidInput,
			fmt.Sprintf("%s variables cannot exceed %d entries", r.label, maxVariables))
	}

	for key, value := range variables {
		if strings.TrimSpace(key) == "" {
			return domain.NewError(domain.ErrInvalidInput,
				fmt.Sprintf("%s variable key cannot be empty", r.label))
		}

		switch value.(type) {
		case nil, bool, float64, string:
			// Skalar JSON. float64 karena encoding/json menguraikan seluruh angka
			// ke sana, termasuk yang bulat.
		default:
			return domain.NewError(domain.ErrInvalidInput,
				fmt.Sprintf("%s variable %q must be a string, number, boolean, or null", r.label, key))
		}
	}

	if err := r.validateNumeric(variables); err != nil {
		return err
	}

	encoded, err := json.Marshal(variables)
	if err != nil {
		return domain.NewError(domain.ErrInvalidInput,
			fmt.Sprintf("%s variables are not valid", r.label))
	}
	if len(encoded) > maxVariableBytes {
		return domain.NewError(domain.ErrInvalidInput,
			fmt.Sprintf("%s variables cannot exceed %d bytes", r.label, maxVariableBytes))
	}

	return nil
}

// validateNumeric menegakkan tipe pada kunci yang dilaporkan.
//
// Negatif ditolak, nol tidak: dokumen maupun proyek bernilai nol adalah keadaan
// yang sah — penawaran gratis, penggantian garansi, pekerjaan yang ditanggung
// sendiri. Yang memang tidak bernilai uang tidak menyertakan kuncinya sama
// sekali; itu berbeda dari nol.
func (r variableRules) validateNumeric(variables map[string]any) error {
	for key := range r.numericKeys {
		value, ada := variables[key]
		if !ada || value == nil {
			continue
		}

		angka, ok := value.(float64)
		if !ok {
			return domain.NewError(domain.ErrInvalidInput,
				fmt.Sprintf("%s variable %q must be a number", r.label, key))
		}
		if angka < 0 {
			return domain.NewError(domain.ErrInvalidInput,
				fmt.Sprintf("%s variable %q cannot be negative", r.label, key))
		}
	}

	return nil
}
