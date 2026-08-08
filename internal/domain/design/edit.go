package design

import (
	"bytes"
	"encoding/json"
	"slices"
)

// Penyuntingan satu elemen di atas isi dokumen.
//
// Seluruh operasi di berkas ini menjaga invarian yang sama dengan Validate: id
// elemen unik SE-DOKUMEN, bukan per halaman, dan setiap elemen yang tersimpan
// sudah lolos pemeriksaan jenisnya. Menjaganya di sini, bukan di pemanggil,
// membuat mustahil ada jalur penyuntingan yang lupa memeriksanya.

// DecodeElement mengurai satu elemen dari muatan pesan penyuntingan.
//
// DisallowUnknownFields sama seperti pada Decode, dan alasannya sama: properti
// yang tidak dikenal ditolak di pintu masuk, supaya tidak ada properti yang
// tampil di layar tetapi hilang saat dicetak.
func DecodeElement(raw json.RawMessage) (Element, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return Element{}, invalidf("element payload is empty")
	}

	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()

	var element Element
	if err := decoder.Decode(&element); err != nil {
		return Element{}, invalidf("element is not valid: %s", err)
	}

	// Diperiksa di sini karena validate() mengurus isi elemen, sedangkan id kosong
	// baru terdeteksi Content.Validate yang memeriksa keunikan antar elemen.
	if element.ID == "" {
		return Element{}, invalidf("element must have a non-empty id")
	}
	if err := element.validate(); err != nil {
		return Element{}, err
	}

	return element, nil
}

// CreateElement memasang elemen baru di akhir halaman, yaitu paling atas.
//
// Gagal bila halamannya tidak ada, atau bila id-nya sudah dipakai elemen lain di
// mana pun dalam dokumen ini. Keduanya kesalahan pemanggil yang sesungguhnya —
// berbeda dari update dan delete yang sasarannya wajar saja sudah lenyap — dan
// karena itu dilaporkan, bukan didiamkan.
func (c *Content) CreateElement(pageID string, element Element) error {
	if err := element.validate(); err != nil {
		return err
	}

	pageIndex := slices.IndexFunc(c.Pages, func(p Page) bool { return p.ID == pageID })
	if pageIndex < 0 {
		return invalidf("page %q does not exist", pageID)
	}
	if _, _, found := c.findElement(element.ID); found {
		return invalidf("element id %q is already used", element.ID)
	}

	c.Pages[pageIndex].Elements = append(c.Pages[pageIndex].Elements, element)

	return nil
}

// UpdateElement mengganti elemen berid sama, apa adanya dan seluruhnya.
//
// applied bernilai false ketika elemennya tidak ada. Itu BUKAN kegagalan: dengan
// menang-terakhir, orang lain yang menghapus elemen tepat sebelum suntingan ini
// tiba adalah kejadian biasa, dan pengirimnya toh sedang menerima siaran
// penghapusan itu. Melaporkannya sebagai error justru memaksa pemuatan ulang
// penuh untuk keadaan yang sudah menyatu dengan sendirinya.
//
// Letaknya di dalam halaman tidak berubah, sehingga urutan gambar tetap. Yang
// memindahkan urutan hanya ReorderElement.
func (c *Content) UpdateElement(element Element) (applied bool, err error) {
	if err := element.validate(); err != nil {
		return false, err
	}

	pageIndex, elementIndex, found := c.findElement(element.ID)
	if !found {
		return false, nil
	}

	c.Pages[pageIndex].Elements[elementIndex] = element

	return true, nil
}

// DeleteElement membuang satu elemen. Mengembalikan false bila memang sudah tidak
// ada — dua orang menghapus elemen yang sama bukan kesalahan siapa pun.
func (c *Content) DeleteElement(id string) bool {
	pageIndex, elementIndex, found := c.findElement(id)
	if !found {
		return false
	}

	elements := c.Pages[pageIndex].Elements
	c.Pages[pageIndex].Elements = slices.Delete(elements, elementIndex, elementIndex+1)

	return true
}

// ReorderElement memindahkan elemen ke posisi lain di dalam halamannya sendiri,
// karena urutan elemen adalah urutan gambar.
//
// Index yang melewati batas DIJEPIT ke ujung terdekat alih-alih ditolak: batasnya
// berubah setiap kali ada yang menambah atau menghapus elemen, jadi klien yang
// menghitung dari keadaan beberapa milidetik lalu wajar saja meleset. Yang
// dikembalikan adalah letak sesungguhnya setelah penjepitan, dan itulah yang
// wajib disiarkan — bukan angka yang diminta.
func (c *Content) ReorderElement(id string, index int) (effective int, applied bool) {
	pageIndex, elementIndex, found := c.findElement(id)
	if !found {
		return 0, false
	}

	elements := c.Pages[pageIndex].Elements
	element := elements[elementIndex]

	// Dijepit SETELAH pengangkatan, bukan sebelumnya. Panjangnya sudah berkurang
	// satu di titik ini, sehingga posisi sisip terakhir yang sah adalah len — dan
	// menjepit terhadap panjang yang lama akan menyisakan satu posisi mustahil.
	elements = slices.Delete(elements, elementIndex, elementIndex+1)
	effective = min(max(index, 0), len(elements))
	c.Pages[pageIndex].Elements = slices.Insert(elements, effective, element)

	return effective, true
}

// findElement mencari elemen ke seluruh halaman.
//
// Menelusuri semuanya, bukan menerima petunjuk halaman dari pemanggil: id elemen
// unik se-dokumen, jadi halaman yang disebutkan klien tidak menambah keterangan
// apa pun — ia hanya menambah satu keadaan yang tidak punya penanganan benar,
// yaitu ketika halaman yang disebut bukan tempat elemen itu berada.
//
// Biayanya sebanding jumlah elemen dokumen, dan dokumen di sini berukuran
// puluhan elemen. Peta indeks baru berguna bila ukurannya berubah drastis, dan
// peta itu harus dijaga tetap sinkron pada setiap operasi — ongkos yang belum
// dibayar oleh manfaatnya.
func (c *Content) findElement(id string) (pageIndex, elementIndex int, found bool) {
	for pi := range c.Pages {
		for ei := range c.Pages[pi].Elements {
			if c.Pages[pi].Elements[ei].ID == id {
				return pi, ei, true
			}
		}
	}

	return 0, 0, false
}
