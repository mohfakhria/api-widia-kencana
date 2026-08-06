package design

import "strings"

// Berkas ini adalah satu-satunya tempat nilai bawaan diterapkan.
//
// Renderer tidak boleh menuliskan angka bawaannya sendiri. Bila ia melakukannya,
// nilai bawaan tersebar di banyak tempat dan perbedaan sekecil apa pun terhadap
// nilai bawaan frontend akan muncul sebagai pergeseran yang sulit dilacak antara
// tampilan layar dan hasil cetak.

func (e *Element) ResolvedFontFamily() string {
	if e.FontFamily == "" {
		return DefaultFontFamily
	}

	return strings.ToLower(e.FontFamily)
}

func (e *Element) ResolvedFontSize() float64 {
	if e.FontSize == 0 {
		return DefaultFontSize
	}

	return e.FontSize
}

func (e *Element) ResolvedFontWeight() int {
	if e.FontWeight == 0 {
		return DefaultFontWeight
	}

	return e.FontWeight
}

func (e *Element) ResolvedFontStyle() string {
	if e.FontStyle == "" {
		return DefaultFontStyle
	}

	return e.FontStyle
}

func (e *Element) ResolvedColor() string {
	if e.Color == "" {
		return DefaultColor
	}

	return e.Color
}

func (e *Element) ResolvedAlign() string {
	if e.Align == "" {
		return DefaultAlign
	}

	return e.Align
}

func (e *Element) ResolvedLineHeight() float64 {
	if e.LineHeight == 0 {
		return DefaultLineHeight
	}

	return e.LineHeight
}

func (e *Element) ResolvedFit() string {
	if e.Fit == "" {
		return DefaultImageFit
	}

	return e.Fit
}

// ParseColor mengubah #rgb atau #rrggbb menjadi komponen merah, hijau, biru.
//
// Hanya dipanggil setelah Validate meloloskan nilainya, jadi bentuk yang tidak
// dikenal tidak mungkin sampai ke sini. Meski begitu ia tetap mengembalikan
// penanda berhasil alih-alih panik, supaya urutan pemanggilan yang keliru
// menghasilkan warna hitam dan bukan proses yang mati.
func ParseColor(value string) (r, g, b int, ok bool) {
	digits, found := strings.CutPrefix(value, "#")
	if !found {
		return 0, 0, 0, false
	}

	// Bentuk pendek diperluas dengan menggandakan tiap digit, sama seperti CSS:
	// #abc setara #aabbcc.
	if len(digits) == 3 {
		digits = string([]byte{
			digits[0], digits[0],
			digits[1], digits[1],
			digits[2], digits[2],
		})
	}
	if len(digits) != 6 {
		return 0, 0, 0, false
	}

	values := make([]int, 3)
	for index := range values {
		high, highOK := hexDigit(digits[index*2])
		low, lowOK := hexDigit(digits[index*2+1])
		if !highOK || !lowOK {
			return 0, 0, 0, false
		}
		values[index] = high*16 + low
	}

	return values[0], values[1], values[2], true
}

func hexDigit(char byte) (int, bool) {
	switch {
	case char >= '0' && char <= '9':
		return int(char - '0'), true
	case char >= 'a' && char <= 'f':
		return int(char-'a') + 10, true
	case char >= 'A' && char <= 'F':
		return int(char-'A') + 10, true
	default:
		return 0, false
	}
}
