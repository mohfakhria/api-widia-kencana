package documentdesign

import "github.com/mohfakhria/api-widia-kencana/internal/domain"

// Penerapan perubahan elemen beserta siarannya.
//
// Ketiga langkahnya selalu berurutan dan tidak boleh dipisah: terapkan ke isi,
// naikkan version, siarkan. Menaikkan version tanpa menyiarkan membuat klien
// melihat celah nomor; menyiarkan tanpa menaikkan version membuat dua perubahan
// berbeda mengaku sebagai revisi yang sama.

// applyCreate memasang elemen baru lalu menyiarkannya.
//
// Satu-satunya jalur penyuntingan yang membalas pengirimnya, karena satu-satunya
// yang dapat gagal di sini.
func (r *Room) applyCreate(e elementCreateEvent) {
	if err := r.editable(e.subscriber); err != nil {
		e.reply <- err
		return
	}

	if err := r.content.CreateElement(e.page, e.element); err != nil {
		e.reply <- err
		return
	}

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

	r.version++
	r.broadcastEdit(r.encoder.EncodeElementUpdated(r.version, e.element))
}

func (r *Room) applyDelete(e elementDeleteEvent) {
	if err := r.editable(e.subscriber); err != nil {
		return
	}
	if !r.content.DeleteElement(e.id) {
		return
	}

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

	effective, applied := r.content.ReorderElement(e.id, e.index)
	if !applied {
		return
	}

	r.version++
	r.broadcastEdit(r.encoder.EncodeElementReordered(r.version, e.id, effective))
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
