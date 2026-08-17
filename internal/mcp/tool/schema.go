package tool

import (
	"fmt"

	"github.com/mohfakhria/api-widia-kencana/internal/domain/design"

	"github.com/google/jsonschema-go/jsonschema"
)

// enumElemen adalah kosakata tertutup milik design.Element.
//
// Diturunkan dari tetapan domain, bukan diketik ulang sebagai literal: bila
// suatu hari ada jenis elemen atau gaya garis baru, yang ditambah cukup di satu
// tempat dan skema ini ikut. Menyalinnya ke sini akan menghasilkan tool yang
// menolak nilai yang sebenarnya sah, dan tidak ada yang menyadarinya sampai ada
// yang mencoba.
var enumElemen = map[string][]any{
	"type": {
		design.ElementText, design.ElementRect, design.ElementEllipse,
		design.ElementLine, design.ElementImage,
	},
	"align":         {design.AlignLeft, design.AlignCenter, design.AlignRight, design.AlignJustify},
	"verticalAlign": {design.VAlignTop, design.VAlignMiddle, design.VAlignBottom},
	"fontStyle":     {design.FontStyleNormal, design.FontStyleItalic},
	"format":        {design.FormatPlain, design.FormatGrouped, design.FormatCurrency, design.FormatPercent},
	"strokeStyle":   {design.StrokeSolid, design.StrokeLongDash, design.StrokeDash, design.StrokeDot},
	"fit":           {design.FitContain, design.FitCover, design.FitFill},
}

// skemaElemen menyusun skema masukan dan menutup kosakatanya.
//
// SDK menurunkan skema dari tipe Go, dan itu sudah menangkap nama field beserta
// tipenya. Yang TIDAK dapat ia ketahui adalah bahwa `type` hanya boleh berisi
// lima nilai: ia hanya melihat string. Tanpa enum, model akan menebak
// "rectangle" alih-alih "rect" — tebakan yang wajar, dan yang membuat seluruh
// pesan ditolak server tanpa petunjuk apa pun tentang nilai yang benar.
//
// Nama fieldnya adalah nama di JSON, karena yang disunting di sini skema, bukan
// struct Go-nya.
func skemaElemen[T any](field string) (*jsonschema.Schema, error) {
	skema, err := jsonschema.For[T](nil)
	if err != nil {
		return nil, fmt.Errorf("turunkan skema: %w", err)
	}

	daftar, ada := skema.Properties[field]
	if !ada {
		return nil, fmt.Errorf("field %q tidak ada di skema", field)
	}

	// Tipenya []design.Element, jadi elemennya ada di Items. Diperiksa, tidak
	// diasumsikan: bentuk masukannya bisa berubah, dan nil di sini akan
	// menjatuhkan server saat start — lebih baik daripada enum yang diam-diam
	// tidak terpasang.
	if daftar.Items == nil || daftar.Items.Properties == nil {
		return nil, fmt.Errorf("field %q bukan larik objek", field)
	}

	for nama, nilai := range enumElemen {
		properti, ada := daftar.Items.Properties[nama]
		if !ada {
			return nil, fmt.Errorf("properti %q tidak ada di elemen", nama)
		}

		properti.Enum = nilai
	}

	return skema, nil
}
