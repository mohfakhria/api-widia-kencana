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

	// historyCoalesceWindow menyatukan perubahan yang datang beruntun menjadi satu
	// langkah undo.
	//
	// Tanpa ini satu tarikan mouse menjadi puluhan langkah — menggeser elemen
	// menghasilkan element.update dua puluh kali per detik, dan pengguna harus
	// menekan Ctrl+Z tiga puluh kali untuk mengembalikan satu gerakan. Yang
	// memulai langkah baru adalah JEDA, bukan jumlah: selama perubahan terus
	// mengalir, semuanya masuk satu langkah.
	historyCoalesceWindow = time.Second
)

// rememberBefore mengambil cuplikan keadaan sekarang bila kelompok perubahan baru
// dimulai.
//
// Mengembalikan nil bila perubahan ini masih menyambung kelompok sebelumnya —
// pemanggil meneruskannya apa adanya ke pushHistory, yang mengabaikannya.
//
// Dipanggil SEBELUM perubahan diterapkan, dan hasilnya baru disimpan setelah
// perubahan terbukti berlaku. Urutan itu penting: perubahan yang ternyata tidak
// berlaku — sasarannya sudah lenyap — tidak boleh meninggalkan langkah undo yang
// bila ditekan tidak melakukan apa-apa.
func (r *Room) rememberBefore(now time.Time) *design.Content {
	if !r.lastChangeAt.IsZero() && now.Sub(r.lastChangeAt) < historyCoalesceWindow {
		return nil
	}

	return r.content.Clone()
}

// pushHistory menyimpan cuplikan yang tadi diambil, lalu menandai bahwa dokumen
// baru saja berubah.
//
// Tumpukan redo dikosongkan oleh SETIAP perubahan baru. Tanpa itu, redo setelah
// menyunting akan memasang keadaan yang tidak lagi menyambung dengan apa pun yang
// ada di layar.
func (r *Room) pushHistory(before *design.Content, now time.Time) {
	r.lastChangeAt = now

	// clear sebelum dipotong, bukan dipotong saja. Memotong panjangnya tidak
	// melepas apa pun: array di baliknya tetap memegang pointer ke isi dokumen,
	// dan karena setiap perubahan mengosongkan redo, ratusan kilobyte bertahan
	// terus sampai slice-nya kebetulan dialokasikan ulang.
	clear(r.redoStack)
	r.redoStack = r.redoStack[:0]

	if before == nil {
		return
	}

	r.undoStack = append(r.undoStack, before)
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
// lastChangeAt dinolkan supaya perubahan berikutnya PASTI memulai langkah baru.
// Menyambungkannya ke kelompok sebelum undo akan membuat satu langkah undo
// mencampur keadaan dari dua sisi perjalanan.
func (r *Room) commitHistoryMove() {
	r.lastChangeAt = time.Time{}
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
