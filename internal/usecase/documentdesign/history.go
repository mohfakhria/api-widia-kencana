package documentdesign

import (
	"time"

	"github.com/mohfakhria/api-widia-kencana/internal/domain/design"
)

const (
	// historyLimit membatasi berapa langkah undo yang disimpan satu room.
	//
	// Cuplikan menyalin seluruh isi dokumen, jadi ongkosnya mengikuti ukuran
	// dokumen: terukur sekitar 18 KB per cuplikan untuk dokumen 59 elemen, 136 KB
	// untuk 500 elemen. Dua puluh langkah membuat jumlah undo dapat ditebak
	// pengguna, dengan harga memori yang berbeda-beda antar dokumen.
	historyLimit = 20

	// historyCoalesceWindow adalah jeda yang mengakhiri satu aliran perubahan.
	//
	// Hanya mengatur perubahan yang memang berupa aliran — lihat streamedChange.
	// Karena itu ia boleh pendek: yang perlu disatukan cuma pesan-pesan dari satu
	// gerakan yang sama, dan gerakan yang berhenti seperempat detik sudah selesai.
	historyCoalesceWindow = 400 * time.Millisecond

	// historyGroupMaxSpan membatasi berapa lama satu langkah undo boleh menampung.
	//
	// Tanpa batas ini, menggeser tanpa melepas mouse selama semenit menjadi SATU
	// langkah, dan satu Ctrl+Z membuang seluruh menit itu. Jeda saja tidak cukup
	// sebagai pembatas, karena aliran yang tidak pernah berhenti tidak pernah
	// menghasilkan jeda.
	historyGroupMaxSpan = 2 * time.Second
)

// changeKind memisahkan perubahan yang berdiri sendiri dari yang datang mengalir.
//
// Pembedaan ini yang menentukan benar-tidaknya pengelompokan. Sebelumnya seluruh
// perubahan diperlakukan sama dan dikelompokkan semata-mata berdasarkan jeda,
// dan akibatnya terukur: dua klik berjarak 300 ms menjadi satu langkah, dan dua
// orang yang masing-masing menyunting tiap 1,2 detik — keduanya merasa santai —
// menghasilkan satu langkah untuk dua belas tindakan, karena dokumennya tidak
// pernah sunyi walau tidak ada seorang pun yang tergesa.
type changeKind int

const (
	// discreteChange adalah satu perbuatan sadar: menambah, menghapus, memindah
	// urutan. Tidak pernah datang sebagai aliran, jadi tidak pernah digabungkan —
	// tiap satu selalu memulai langkah undo tersendiri.
	discreteChange changeKind = iota

	// streamedChange adalah satu gerakan yang terpecah menjadi banyak pesan:
	// menggeser elemen, mengetik judul halaman. Puluhan pesan per detik, dan
	// pengguna menganggapnya satu tindakan.
	streamedChange
)

// historyMark adalah keadaan dokumen tepat sebelum satu perubahan, beserta jenis
// dan waktu perubahan itu.
//
// Ketiganya dibawa bersama supaya jenisnya cukup disebut SEKALI di tiap pemanggil.
// Dulu ia disebut dua kali — saat mengambil cuplikan dan saat menyimpannya — dan
// tidak ada yang memeriksa kedua sebutan itu cocok; pemanggil kesembilan yang
// keliru tidak akan menghasilkan galat apa pun, hanya riwayat yang aneh.
type historyMark struct {
	at   time.Time
	kind changeKind
	// before nil berarti perubahan ini menyambung kelompok yang sedang berjalan
	// dan karena itu tidak memulai langkah undo tersendiri.
	before *design.Content
}

// beginChange mengambil cuplikan keadaan sekarang bila kelompok perubahan baru
// dimulai.
//
// Dipanggil SEBELUM perubahan diterapkan, dan hasilnya baru disimpan lewat
// commitChange setelah perubahan terbukti berlaku. Urutan itu penting: perubahan
// yang ternyata tidak berlaku — sasarannya sudah lenyap — tidak boleh meninggalkan
// langkah undo yang bila ditekan tidak melakukan apa-apa.
func (r *Room) beginChange(kind changeKind) historyMark {
	now := time.Now()
	mark := historyMark{at: now, kind: kind}

	if !r.continuesGroup(now, kind) {
		mark.before = r.content.Clone()
	}

	return mark
}

// continuesGroup menjawab apakah perubahan ini masih bagian dari kelompok yang
// sedang berjalan. Tiga syarat, dan ketiganya harus terpenuhi.
func (r *Room) continuesGroup(now time.Time, kind changeKind) bool {
	// Satu: yang berdiri sendiri tidak menyambung apa pun, DAN tidak pernah ada
	// kelompok terbuka miliknya untuk disambung — lihat commitChange. Kelompok yang
	// terbuka selalu kelompok aliran, dan groupStartedAt nol berarti tidak ada.
	if kind != streamedChange || r.groupStartedAt.IsZero() {
		return false
	}

	// Dua: aliran yang sempat berhenti sudah selesai.
	if r.lastChangeAt.IsZero() || now.Sub(r.lastChangeAt) >= historyCoalesceWindow {
		return false
	}

	// Tiga: kelompok yang sudah terlalu panjang dipotong walau alirannya belum
	// berhenti. Ini yang menjaga agar satu Ctrl+Z tidak pernah membuang lebih
	// dari beberapa detik pekerjaan.
	return now.Sub(r.groupStartedAt) < historyGroupMaxSpan
}

// commitChange menyimpan cuplikan yang tadi diambil, lalu menandai bahwa dokumen
// baru saja berubah.
//
// Tumpukan redo dikosongkan oleh SETIAP perubahan baru, termasuk yang tergabung.
// Tanpa itu, redo setelah menyunting akan memasang keadaan yang tidak lagi
// menyambung dengan apa pun yang ada di layar.
func (r *Room) commitChange(m historyMark) {
	r.lastChangeAt = m.at

	// clear sebelum dipotong, bukan dipotong saja. Memotong panjangnya tidak
	// melepas apa pun: array di baliknya tetap memegang pointer ke isi dokumen,
	// dan karena setiap perubahan mengosongkan redo, ratusan kilobyte bertahan
	// terus sampai slice-nya kebetulan dialokasikan ulang.
	clear(r.redoStack)
	r.redoStack = r.redoStack[:0]

	if m.before == nil {
		return
	}

	// HANYA aliran yang membuka kelompok. Tindakan yang berdiri sendiri menutup
	// dirinya seketika, supaya perubahan berikutnya tidak ikut terserap.
	//
	// Membuka kelompok untuknya juga pernah dicoba dan salah: memulai langkah baru
	// di AWAL tindakan diskret tidak ada gunanya bila langkah itu lalu menelan
	// geseran yang datang sesudahnya. Terukur waktu itu — membuat elemen lalu
	// segera menggesernya menjadi satu langkah, sehingga satu Ctrl+Z menghapus
	// elemennya alih-alih mengembalikan posisinya; dan pada dokumen berdua,
	// tindakan satu orang menelan sisa geseran orang lain.
	if m.kind == streamedChange {
		r.groupStartedAt = m.at
	} else {
		r.groupStartedAt = time.Time{}
	}

	r.undoStack = append(r.undoStack, m.before)
	if len(r.undoStack) > historyLimit {
		// Yang tertua dibuang dengan menggeser sisanya ke depan, bukan dengan
		// memotong dari kepala. Memotong dari kepala membuat slice merayap
		// menjauhi awal array-nya, sehingga array itu terus tumbuh dan disalin
		// ulang walau isinya tidak pernah lebih dari historyLimit.
		copy(r.undoStack, r.undoStack[1:])
		r.undoStack[len(r.undoStack)-1] = nil
		r.undoStack = r.undoStack[:len(r.undoStack)-1]
	}
}

// applyUndo memasang kembali keadaan sebelum kelompok perubahan terakhir.
//
// Berlaku untuk SELURUH DOKUMEN, bukan untuk langkah orang yang menekannya. Itu
// keputusan sadar: dokumen ini disunting bersama manusia dan agent, dan undo per
// orang berarti manusia tidak dapat membatalkan kesalahan agent.
func (r *Room) applyUndo(e undoEvent) {
	if err := r.editable(e.subscriber); err != nil {
		return
	}
	if len(r.undoStack) == 0 {
		return
	}

	last := len(r.undoStack) - 1
	sekarang := r.content

	r.content = r.undoStack[last]
	r.undoStack[last] = nil
	r.undoStack = r.undoStack[:last]
	r.redoStack = append(r.redoStack, sekarang)

	r.commitHistoryMove()
}

func (r *Room) applyRedo(e redoEvent) {
	if err := r.editable(e.subscriber); err != nil {
		return
	}
	if len(r.redoStack) == 0 {
		return
	}

	last := len(r.redoStack) - 1
	sekarang := r.content

	r.content = r.redoStack[last]
	r.redoStack[last] = nil
	r.redoStack = r.redoStack[:last]
	r.undoStack = append(r.undoStack, sekarang)

	r.commitHistoryMove()
}

// commitHistoryMove menaikkan version lalu menyiarkan snapshot penuh.
//
// Snapshot, bukan siaran perubahan: satu langkah undo dapat menyentuh apa saja —
// termasuk mengembalikan halaman beserta seluruh elemennya — dan tidak ada bentuk
// delta yang mewakilinya. Ongkosnya sepadan karena undo adalah tekan tombol,
// bukan aliran seperti kursor.
//
// Kedua penanda waktu dinolkan supaya perubahan berikutnya PASTI memulai langkah
// baru.
// Menyambungkannya ke kelompok sebelum undo akan membuat satu langkah undo
// mencampur keadaan dari dua sisi perjalanan.
func (r *Room) commitHistoryMove() {
	r.lastChangeAt = time.Time{}
	r.groupStartedAt = time.Time{}
	r.version++

	payload, err := r.encodeSnapshot()
	if err != nil {
		// Isi yang tidak dapat dikodekan juga tidak akan pernah dapat disimpan.
		r.markBroken(err, "document content can no longer be encoded")
		return
	}

	for subscriber := range r.members {
		subscriber.Send(payload)
	}
}
