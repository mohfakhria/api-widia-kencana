package pdf

import (
	"strconv"
	"strings"

	"github.com/mohfakhria/api-widia-kencana/internal/domain/design"
)

// Penanda nomor lembar, diganti saat menggambar dan HANYA pada elemen master.
//
// Sengaja tidak diganti pada elemen halaman, dan alasannya bukan selera:
// editor pun tidak menggantinya di sana, sehingga penggantian di sini akan
// membuat layar dan cetakan berselisih pada dokumen yang di layar terlihat
// benar. Selisih semacam itu tidak muncul sebagai galat — ia muncul sebagai
// invoice yang sudah terkirim.
//
// Dicocokkan PERSIS: peka huruf besar-kecil, tanpa toleransi spasi. `{{ page }}`
// dan `{{Page}}` dibiarkan sebagai teks biasa. Toleransi hanya aman bila editor
// menoleransi hal yang sama persis, dan bentuk yang lebih longgar di satu sisi
// menghasilkan tepat selisih yang aturan di atas ada untuk mencegahnya.
const (
	placeholderSheet  = "{{page}}"
	placeholderSheets = "{{pages}}"
)

// withSheetNumbers mengembalikan SALINAN elemen dengan penanda nomor lembar
// sudah terganti.
//
// Salinan, bukan penyuntingan di tempat, dan itu wajib: elemen master yang sama
// digambar di setiap lembar, jadi menulis hasilnya kembali ke isi dokumen akan
// membekukan nomor lembar pertama untuk seluruh lembar berikutnya — dan
// mengotori isi yang tersimpan dengan angka yang seharusnya tidak pernah ada di
// sana.
//
// Elemen yang bukan teks, atau yang teksnya tidak memuat penanda, dikembalikan
// tanpa perubahan sama sekali.
func withSheetNumbers(element design.Element, sheet, sheets int) design.Element {
	if element.Type != design.ElementText {
		return element
	}
	if !strings.Contains(element.Text, placeholderSheet) &&
		!strings.Contains(element.Text, placeholderSheets) {
		return element
	}

	// Urutannya TIDAK berpengaruh, dan itu layak disebut karena pembaca
	// berikutnya pasti mengiranya berpengaruh: `{{pages}}` tampak memuat
	// `{{page}}` sebagai awalan, padahal tidak — token yang tunggal menuntut
	// kurung tutup langsung setelah "page", sedangkan di `{{pages}}` yang menyusul
	// adalah huruf "s". Diperiksa, bukan disimpulkan; kedua urutan menghasilkan
	// keluaran yang sama untuk "{{page}}{{pages}}" maupun kalimat campuran.
	//
	// Yang jamak tetap ditulis lebih dulu semata-mata supaya barisnya terbaca
	// sejalan dengan urutan bacaan orang.
	element.Text = strings.ReplaceAll(element.Text, placeholderSheets, strconv.Itoa(sheets))
	element.Text = strings.ReplaceAll(element.Text, placeholderSheet, strconv.Itoa(sheet))

	return element
}
