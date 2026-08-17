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

	// Master diperiksa SEBELUM daftar halaman. Urutan ini yang membuat id
	// tercadang tetap tidak ambigu pada dokumen lama yang telanjur punya halaman
	// bernama "master" — halaman itu menjadi tidak terjangkau, bukan menyandera
	// lapisan master.
	owner := &c.Master.Elements
	if pageID != MasterPageID {
		pageIndex := slices.IndexFunc(c.Pages, func(p Page) bool { return p.ID == pageID })
		if pageIndex < 0 {
			return invalidf("page %q does not exist", pageID)
		}
		owner = &c.Pages[pageIndex].Elements
	}

	// Keunikan id berlaku SE-DOKUMEN, termasuk lintas master dan halaman: elemen
	// dicari lewat id saja, jadi id yang sama di dua tempat membuat update dan
	// delete menyasar salah satunya secara acak.
	if _, _, found := c.locateElement(element.ID); found {
		return invalidf("element id %q is already used", element.ID)
	}

	*owner = append(*owner, element)

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

	owner, index, found := c.locateElement(element.ID)
	if !found {
		return false, nil
	}

	(*owner)[index] = element

	return true, nil
}

// DeleteElement membuang satu elemen. Mengembalikan false bila memang sudah tidak
// ada — dua orang menghapus elemen yang sama bukan kesalahan siapa pun.
func (c *Content) DeleteElement(id string) bool {
	owner, index, found := c.locateElement(id)
	if !found {
		return false
	}

	*owner = slices.Delete(*owner, index, index+1)

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
func (c *Content) ReorderElement(id string, target int) (effective int, applied bool) {
	owner, index, found := c.locateElement(id)
	if !found {
		return 0, false
	}

	// Larik pemiliknya, apa pun itu. Elemen master karenanya diurutkan di antara
	// elemen master saja, dan tidak ada satu baris pun di sini yang perlu
	// mengetahuinya.
	elements := *owner
	element := elements[index]

	// Dijepit SETELAH pengangkatan, bukan sebelumnya. Panjangnya sudah berkurang
	// satu di titik ini, sehingga posisi sisip terakhir yang sah adalah len — dan
	// menjepit terhadap panjang yang lama akan menyisakan satu posisi mustahil.
	elements = slices.Delete(elements, index, index+1)
	effective = min(max(target, 0), len(elements))
	*owner = slices.Insert(elements, effective, element)

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
	// Id tercadang. Halaman bernama "master" akan membuat element.create
	// bermakna dua hal sekaligus, dan yang kalah adalah lapisan master — yang
	// tidak punya cara lain untuk ditunjuk.
	if id == MasterPageID {
		return 0, invalidf("page id %q is reserved for the master layer", MasterPageID)
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

// locateElement mencari elemen ke seluruh halaman DAN ke lapisan master.
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
//
// Yang dikembalikan LARIK PEMILIKNYA, bukan indeks halaman.
//
// Bentuk kembalian itulah yang membuat lapisan master tidak menambah satu pun
// percabangan: update, delete, dan reorder bekerja pada larik yang diberikan di
// sini tanpa pernah menanyakan apakah pemiliknya sebuah halaman atau master.
// Aturan "indeks reorder menghitung di antara elemen master saja" karenanya
// menjadi akibat dari struktur datanya, bukan sebuah kasus khusus yang harus
// diingat — dan kasus khusus yang harus diingat adalah kasus khusus yang suatu
// hari terlupakan.
func (c *Content) locateElement(id string) (owner *[]Element, index int, found bool) {
	for pi := range c.Pages {
		for ei := range c.Pages[pi].Elements {
			if c.Pages[pi].Elements[ei].ID == id {
				return &c.Pages[pi].Elements, ei, true
			}
		}
	}

	for ei := range c.Master.Elements {
		if c.Master.Elements[ei].ID == id {
			return &c.Master.Elements, ei, true
		}
	}

	return nil, 0, false
}

// MaxGuides membatasi jumlah garis bantu satu dokumen.
//
// Berbeda dari elemen yang sengaja tidak dibatasi, guide dibatasi karena ia
// masukan tak tepercaya yang DISIARKAN ULANG ke setiap penghuni dan ikut
// tersimpan: tanpa batas, satu klien dapat menumbuhkan isi dokumen tanpa henti
// dengan pesan yang masing-masing sah. Dua ratus jauh melampaui jumlah yang
// masih berguna bagi manusia yang melihatnya di layar.
//
// Diperiksa di titik PERTUMBUHAN saja, sama seperti MaxPages dan dengan alasan
// yang sama: dokumen yang telanjur melewati batas tetap dapat dimuat.
const MaxGuides = 200

// DecodeGuide mengurai satu guide dari muatan pesan penyuntingan.
//
// DisallowUnknownFields sama seperti DecodeElement: properti yang tidak dikenal
// ditolak di pintu masuk, bukan diabaikan diam-diam.
func DecodeGuide(raw json.RawMessage) (Guide, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return Guide{}, invalidf("guide payload is empty")
	}

	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()

	var guide Guide
	if err := decoder.Decode(&guide); err != nil {
		return Guide{}, invalidf("guide is not valid: %s", err)
	}

	if guide.ID == "" {
		return Guide{}, invalidf("guide must have a non-empty id")
	}
	if err := guide.validate(); err != nil {
		return Guide{}, err
	}

	return guide, nil
}

// CreateGuide memasang garis bantu baru.
//
// Gagal bila id-nya sudah dipakai — kesalahan pemanggil yang sesungguhnya, sama
// seperti CreateElement, dan karena itu dilaporkan alih-alih didiamkan.
func (c *Content) CreateGuide(guide Guide) error {
	if err := guide.validate(); err != nil {
		return err
	}
	if c.findGuide(guide.ID) >= 0 {
		return invalidf("guide id %q is already used", guide.ID)
	}
	if len(c.Guides) >= MaxGuides {
		return invalidf("document already has the maximum of %d guides", MaxGuides)
	}

	c.Guides = append(c.Guides, guide)

	return nil
}

// UpdateGuide mengganti satu guide SELURUHNYA.
//
// Guide yang sudah lenyap DIDIAMKAN, sama seperti UpdateElement: dua orang yang
// menyunting guide yang sama pada saat yang hampir bersamaan adalah lomba yang
// wajar pada menang-terakhir, dan yang kalah toh sedang menerima siaran
// penghapusannya.
func (c *Content) UpdateGuide(guide Guide) (applied bool, err error) {
	if err := guide.validate(); err != nil {
		return false, err
	}

	index := c.findGuide(guide.ID)
	if index < 0 {
		return false, nil
	}

	c.Guides[index] = guide

	return true, nil
}

// DeleteGuide membuang satu guide. False berarti memang sudah tidak ada.
func (c *Content) DeleteGuide(id string) bool {
	index := c.findGuide(id)
	if index < 0 {
		return false
	}

	c.Guides = slices.Delete(c.Guides, index, index+1)

	return true
}

// findGuide mengembalikan -1 bila tidak ditemukan.
//
// Pencarian lurus, bukan peta: daftarnya berbatas dua ratus dan hampir selalu
// berisi belasan, sehingga peta yang harus dijaga tetap sinkron dengan larik
// hanya menambah satu keadaan yang dapat melenceng.
func (c *Content) findGuide(id string) int {
	for index := range c.Guides {
		if c.Guides[index].ID == id {
			return index
		}
	}

	return -1
}
