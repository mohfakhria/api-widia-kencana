package design

import (
	"bytes"
	"encoding/json"
	"slices"
)

// Penyuntingan isi dokumen: elemen dan halaman.
//
// Seluruh operasi di berkas ini menjaga invarian yang sama dengan Validate: id
// elemen unik SE-DOKUMEN, bukan per halaman, dan setiap elemen yang tersimpan
// sudah lolos pemeriksaan jenisnya. Menjaganya di sini, bukan di pemanggil,
// membuat mustahil ada jalur penyuntingan yang lupa memeriksanya.

// MaxPages membatasi jumlah halaman satu dokumen.
//
// Ada karena halaman berbiaya jauh melampaui isinya: tiap halaman menjadi satu
// halaman PDF yang digambar dari nol saat ekspor. Jumlah ELEMEN sengaja tidak
// dibatasi — dokumen sungguhan tidak mendekatinya, dan batas yang tidak pernah
// tersentuh hanya menambah satu jalur penolakan yang harus dijelaskan ke
// frontend.
//
// Diperiksa di titik PERTUMBUHAN saja, bukan di Validate. Menaruhnya di Validate
// membuat dokumen lama yang telanjur melewati batas tidak dapat dimuat sama
// sekali, dan room-nya rusak permanen — hukuman yang jauh lebih berat daripada
// masalah yang sedang ditutup.
const MaxPages = 200

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

// CreatePage menyisipkan halaman kosong.
//
// index kosong berarti di akhir. Pointer, bukan int biasa, karena "tidak menyebut
// posisi" adalah maksud yang sah dan sering — sedangkan int biasa mengubah
// penghilangan itu menjadi nol, yaitu paling depan, persis kebalikan dari yang
// dimaksud pengirimnya.
//
// Yang melewati batas dijepit, sama seperti ReorderElement, dan letak
// sesungguhnya itulah yang dikembalikan.
func (c *Content) CreatePage(id string, index *int) (effective int, err error) {
	if id == "" {
		return 0, invalidf("page must have a non-empty id")
	}
	if slices.ContainsFunc(c.Pages, func(p Page) bool { return p.ID == id }) {
		return 0, invalidf("page id %q is already used", id)
	}
	if len(c.Pages) >= MaxPages {
		return 0, invalidf("document already has the maximum of %d pages", MaxPages)
	}

	// Elements dibuat kosong, bukan dibiarkan nil. Slice nil disandikan JSON
	// sebagai null, sedangkan kontrak menjanjikan array — dan null memaksa setiap
	// pembaca memeriksanya lebih dulu.
	page := Page{ID: id, Elements: []Element{}}

	effective = len(c.Pages)
	if index != nil {
		effective = min(max(*index, 0), len(c.Pages))
	}
	c.Pages = slices.Insert(c.Pages, effective, page)

	return effective, nil
}

// PageProps adalah seluruh properti halaman yang dapat disetel page.update.
//
// Dibungkus struct, bukan diteruskan sebagai deretan parameter, karena title dan
// background sama-sama string: tertukar di salah satu dari lima lapisan yang
// dilewatinya akan tetap dapat dikompilasi, dan gejalanya berupa judul halaman
// yang berubah menjadi kode warna.
type PageProps struct {
	Title      string
	Background string
	Hidden     bool
	Locked     bool
}

// UpdatePage menyetel properti halaman, dan HANYA properti halaman.
//
// Sengaja tidak menerima halaman utuh seperti UpdateElement menerima elemen utuh.
// Elemen adalah daun; halaman memuat elemen — mengganti halaman seutuhnya berarti
// setiap perubahan hidden ikut menimpa seluruh isinya, dan dua orang yang
// menyunting elemen di halaman itu akan saling menghapus pekerjaan.
//
// changed bernilai false juga ketika nilainya sudah sama persis, bukan hanya
// ketika halamannya tidak ada. Keduanya sama-sama tidak mengubah dokumen, dan
// menaikkan version untuk perubahan yang tidak mengubah apa pun akan membuat
// klien lain memuat ulang tanpa sebab.
//
// Pemeriksaan ini mungkin di sini karena propertinya sedikit. UpdateElement
// tidak melakukannya: membandingkan elemen utuh jauh lebih mahal daripada
// sesekali menyiarkan yang sama.
func (c *Content) UpdatePage(id string, props PageProps) (changed bool) {
	index := slices.IndexFunc(c.Pages, func(p Page) bool { return p.ID == id })
	if index < 0 {
		return false
	}

	page := &c.Pages[index]
	if page.Title == props.Title && page.Background == props.Background &&
		page.Hidden == props.Hidden && page.Locked == props.Locked {
		return false
	}

	page.Title = props.Title
	page.Background = props.Background
	page.Hidden = props.Hidden
	page.Locked = props.Locked

	return true
}

// VisiblePages adalah satu-satunya definisi "terlihat" di seluruh aplikasi.
//
// Ekspor memakainya untuk dua hal — menggambar, dan menentukan aset mana yang
// perlu diunduh. Bila keduanya menyaring sendiri-sendiri, cepat atau lambat salah
// satunya lupa, dan gejalanya berupa halaman tersembunyi yang menggagalkan ekspor
// karena gambarnya tidak dapat diambil.
func (c *Content) VisiblePages() []Page {
	pages := make([]Page, 0, len(c.Pages))
	for _, page := range c.Pages {
		if page.Hidden {
			continue
		}
		pages = append(pages, page)
	}

	return pages
}

// DeletePage membuang halaman beserta seluruh elemen di atasnya.
//
// Halaman TERAKHIR tidak boleh dibuang, dan itu bukan kerewelan. Dokumen yang
// tersimpan tanpa halaman akan dianggap kosong saat dimuat berikutnya, lalu
// ditimpa panduan bawaan oleh Room.load — sehingga template yang dirawat
// berbulan-bulan berubah menjadi teks tutorial, tanpa error, dan bukan pada saat
// penghapusannya melainkan ketika orang berikutnya membukanya.
//
// applied bernilai false ketika halamannya memang sudah tidak ada, sama seperti
// operasi elemen.
func (c *Content) DeletePage(id string) (applied bool, err error) {
	index := slices.IndexFunc(c.Pages, func(p Page) bool { return p.ID == id })
	if index < 0 {
		return false, nil
	}
	if len(c.Pages) == 1 {
		return false, invalidf("the last page cannot be deleted")
	}

	c.Pages = slices.Delete(c.Pages, index, index+1)

	return true, nil
}

// ReorderPage memindahkan halaman. Penjepitannya sama persis dengan
// ReorderElement, termasuk alasan kenapa dijepit setelah pengangkatan.
func (c *Content) ReorderPage(id string, index int) (effective int, applied bool) {
	from := slices.IndexFunc(c.Pages, func(p Page) bool { return p.ID == id })
	if from < 0 {
		return 0, false
	}

	page := c.Pages[from]
	pages := slices.Delete(c.Pages, from, from+1)
	effective = min(max(index, 0), len(pages))
	c.Pages = slices.Insert(pages, effective, page)

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
