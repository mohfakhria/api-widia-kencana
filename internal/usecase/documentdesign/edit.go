package documentdesign

import (
	"github.com/mohfakhria/api-widia-kencana/internal/domain"
)

// Penerapan perubahan isi dokumen — elemen dan halaman — beserta siarannya.
//
// Ketiga langkahnya selalu berurutan dan tidak boleh dipisah: terapkan ke isi,
// naikkan version, siarkan. Menaikkan version tanpa menyiarkan membuat klien
// melihat celah nomor; menyiarkan tanpa menaikkan version membuat dua perubahan
// berbeda mengaku sebagai revisi yang sama.
//
// beginChange selalu mendahului perubahan dan commitChange selalu mengikutinya,
// dan commitChange hanya dijalankan setelah perubahan terbukti berlaku. Urutan
// itu penting di kedua ujungnya:
// mengambilnya sesudah berarti yang tersimpan keadaan yang sudah berubah, dan
// menyimpannya tanpa memeriksa berlaku berarti ada langkah undo yang bila ditekan
// tidak melakukan apa-apa.
//
// Pembagian membalas atau tidak mengikuti satu aturan: yang dapat DITOLAK
// membalas, yang paling banter tidak berlaku tidak membalas. Penolakan berarti
// layar pengirimnya sudah telanjur berbeda dari dokumen dan hanya ia yang dapat
// membetulkannya; tidak berlaku berarti keadaannya menyatu dengan sendirinya
// lewat siaran yang sedang menuju ke sana.

// applyCreate memasang elemen baru lalu menyiarkannya.
//
// Satu-satunya jalur penyuntingan yang membalas pengirimnya, karena satu-satunya
// yang dapat gagal di sini.
func (r *Room) applyCreate(e elementCreateEvent) {
	if err := r.editable(e.subscriber); err != nil {
		e.reply <- err
		return
	}

	mark := r.beginChange(discreteChange)

	if err := r.content.CreateElement(e.page, e.element); err != nil {
		e.reply <- err
		return
	}

	r.commitChange(mark)
	r.version++
	r.broadcastEdit(r.encoder.EncodeElementCreated(r.version, e.page, e.element))

	e.reply <- nil
}

// applyUpdate mengganti elemen yang sudah ada.
//
// Elemen yang tidak ditemukan didiamkan. Orang lain yang menghapusnya tepat
// sebelum suntingan ini tiba adalah lomba yang wajar pada menang-terakhir, dan
// pengirimnya toh sedang menerima siaran penghapusan itu. Karena version tidak
// naik, tidak ada klien yang melihat celah nomor karenanya.
func (r *Room) applyUpdate(e elementUpdateEvent) {
	if err := r.editable(e.subscriber); err != nil {
		return
	}

	mark := r.beginChange(streamedChange)

	applied, err := r.content.UpdateElement(e.element)
	if err != nil {
		// Tidak seharusnya terjadi: muatannya sudah divalidasi saat diurai di
		// lapisan delivery. Kemunculannya berarti kedua pemeriksaan itu tidak lagi
		// sepakat, dan itu bug yang perlu terlihat.
		r.logger.Error("update document design element",
			"document", r.token, "element", e.element.ID, "error", err)
		return
	}
	if !applied {
		return
	}

	r.commitChange(mark)
	r.version++
	r.broadcastEdit(r.encoder.EncodeElementUpdated(r.version, e.element))
}

func (r *Room) applyDelete(e elementDeleteEvent) {
	if err := r.editable(e.subscriber); err != nil {
		return
	}
	mark := r.beginChange(discreteChange)

	if !r.content.DeleteElement(e.id) {
		return
	}

	r.commitChange(mark)
	r.version++
	r.broadcastEdit(r.encoder.EncodeElementDeleted(r.version, e.id))
}

// applyReorder memindahkan elemen di dalam halamannya.
//
// Yang disiarkan letak SESUNGGUHNYA setelah penjepitan, bukan yang diminta —
// klien yang meminta indeks di luar batas perlu tahu ke mana elemennya benar-benar
// mendarat.
func (r *Room) applyReorder(e elementReorderEvent) {
	if err := r.editable(e.subscriber); err != nil {
		return
	}

	mark := r.beginChange(discreteChange)

	effective, applied := r.content.ReorderElement(e.id, e.index)
	if !applied {
		return
	}

	r.commitChange(mark)
	r.version++
	r.broadcastEdit(r.encoder.EncodeElementReordered(r.version, e.id, effective))
}

// applyPageCreate menyisipkan halaman kosong.
//
// Membalas karena dapat ditolak: id sudah dipakai, atau dokumen sudah menyentuh
// batas jumlah halaman.
func (r *Room) applyPageCreate(e pageCreateEvent) {
	if err := r.editable(e.subscriber); err != nil {
		e.reply <- err
		return
	}

	mark := r.beginChange(discreteChange)

	effective, err := r.content.CreatePage(e.id, e.index)
	if err != nil {
		e.reply <- err
		return
	}

	r.commitChange(mark)
	r.version++
	r.broadcastEdit(r.encoder.EncodePageCreated(r.version, e.id, effective))

	e.reply <- nil
}

// applyPageUpdate menyetel title, hidden, dan locked.
//
// Tidak membalas: yang dapat ditolak — muatan tanpa salah satu field — sudah
// ditahan di lapisan delivery. Yang tersisa di sini paling banter tidak berlaku,
// yaitu halaman yang sudah lenyap atau nilai yang memang sudah sama.
//
// Dari ketiganya hanya hidden yang mengubah hasil cetak: ekspor melewati halaman
// tersembunyi seluruhnya, termasuk tidak mengunduh asetnya. title dan locked
// tidak pernah digambar — keduanya keterangan bagi editor.
func (r *Room) applyPageUpdate(e pageUpdateEvent) {
	if err := r.editable(e.subscriber); err != nil {
		return
	}
	mark := r.beginChange(streamedChange)

	if !r.content.UpdatePage(e.id, e.title, e.hidden, e.locked) {
		return
	}

	r.commitChange(mark)
	r.version++
	r.broadcastEdit(r.encoder.EncodePageUpdated(r.version, e.id, e.title, e.hidden, e.locked))
}

// applyPageDelete membuang halaman beserta seluruh elemennya.
//
// Membalas, berbeda dari applyDelete pada elemen, karena penghapusan halaman
// punya satu jalur penolakan yang tidak dimiliki elemen: halaman terakhir. Alasan
// larangan itu ada di design.DeletePage, dan akibatnya terlalu buruk untuk
// didiamkan.
//
// Kursor orang lain dapat menunjuk halaman yang baru hilang. Tidak ada yang perlu
// dilakukan: frontend menyembunyikan kursor yang halamannya bukan halaman yang
// sedang ia lihat, dan gerakan berikutnya menggantinya sendiri.
func (r *Room) applyPageDelete(e pageDeleteEvent) {
	if err := r.editable(e.subscriber); err != nil {
		e.reply <- err
		return
	}

	mark := r.beginChange(discreteChange)

	applied, err := r.content.DeletePage(e.id)
	if err != nil {
		e.reply <- err
		return
	}
	if !applied {
		e.reply <- nil
		return
	}

	r.commitChange(mark)
	r.version++
	r.broadcastEdit(r.encoder.EncodePageDeleted(r.version, e.id))

	e.reply <- nil
}

// applyPageReorder memindahkan halaman. Tidak membalas — tidak ada yang dapat
// ditolak, dan halaman yang sudah lenyap didiamkan seperti elemen.
func (r *Room) applyPageReorder(e pageReorderEvent) {
	if err := r.editable(e.subscriber); err != nil {
		return
	}

	mark := r.beginChange(discreteChange)

	effective, applied := r.content.ReorderPage(e.id, e.index)
	if !applied {
		return
	}

	r.commitChange(mark)
	r.version++
	r.broadcastEdit(r.encoder.EncodePageReordered(r.version, e.id, effective))
}

// editable menjawab apakah koneksi ini boleh menyunting.
//
// Dua syarat, dan keduanya menghasilkan frasa yang aman disampaikan ke klien.
// Room yang rusak menolak lebih dulu: melayani suntingan atas isi yang tidak akan
// pernah tersimpan hanya menunda kabar buruknya.
func (r *Room) editable(subscriber Subscriber) error {
	if r.broken != nil {
		return r.broken
	}
	if _, ok := r.members[subscriber]; !ok {
		return domain.NewError(domain.ErrUnavailable, "document has not been requested on this connection")
	}

	return nil
}

// broadcastEdit mengirim satu siaran perubahan ke SELURUH penghuni, pengirimnya
// ikut.
//
// Pengirim tidak dilewati — berbeda dari kursor. Kalau ia dilewati, nomor
// version-nya tertinggal setiap kali ia menyunting, lalu siaran pertama dari orang
// lain terlihat melompat dan ia memuat ulang seluruh dokumen tanpa sebab. Ongkos
// mengirimkannya kembali jauh lebih murah daripada itu.
//
// Send, bukan SendEphemeral: perubahan tidak boleh hilang. Klien yang tertinggal
// terlalu jauh diputus, dan menyambung ulang menghasilkan keadaan yang benar —
// sedangkan menerima sebagian menghasilkan keadaan yang salah tanpa ada yang tahu.
//
// Kegagalan menyandikan hanya dicatat. Perubahannya sudah diterapkan dan version
// sudah naik, jadi siaran berikutnya akan tampak melompat bagi klien dan mereka
// meminta document.get sendiri — keadaannya pulih tanpa perlu ditangani di sini.
func (r *Room) broadcastEdit(payload []byte, err error) {
	if err != nil {
		r.logger.Error("encode document design element broadcast",
			"document", r.token, "version", r.version, "error", err)
		return
	}

	for subscriber := range r.members {
		subscriber.Send(payload)
	}
}
