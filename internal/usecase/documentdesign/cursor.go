package documentdesign

import (
	"cmp"
	"slices"
	"time"
)

// cursorTickInterval adalah jarak antar siaran kursor.
//
// Gerakan dipadatkan, bukan disiarkan saat diterima: yang masuk hanya menimpa
// satu entri peta, dan denyut ini yang menyiarkan seluruh peta sekali. Laju
// keluar karenanya terikat pada denyut, berapa pun derasnya masukan.
//
// Menaikkannya ke 30 Hz cukup mengganti tetapan ini saja.
const cursorTickInterval = 70 * time.Millisecond

// cursorStream menandai seluruh siaran kursor sebagai satu aliran
// nilai-terakhir. Siaran baru menggantikan siaran kursor yang masih mengantre di
// klien, karena letak kursor satu denyut lalu sudah tidak berarti apa pun
// begitu ada yang lebih baru.
const cursorStream = "cursor"

// broadcastCursors mengirim letak seluruh kursor ke semua penghuni.
//
// Kegagalan menyusun payload hanya dicatat, tidak menandai room rusak. Kursor
// adalah hiasan di sekitar pekerjaan yang sebenarnya; menghentikan penyuntingan
// karena ia gagal disusun jauh lebih merugikan daripada kursor yang tidak
// bergerak.
func (r *Room) broadcastCursors() {
	// Penanda kotor sengaja TIDAK dibersihkan saat penghuninya kurang dari dua.
	// Dengan begitu orang kedua yang bergabung langsung menerima kursor yang sudah
	// ada pada denyut berikutnya, bukan menunggu ada yang menggerakkannya lagi.
	if !r.cursorsDirty || len(r.members) < 2 {
		return
	}
	r.cursorsDirty = false

	payload, err := r.encodeCursors()
	if err != nil {
		return
	}

	for subscriber := range r.members {
		subscriber.SendEphemeral(cursorStream, payload)
	}
}

// sendCursors mengirim kursor yang sudah ada kepada satu orang saja, dipakai saat
// ia baru bergabung.
//
// Dilewati bila belum ada kursor sama sekali: daftar kosong tidak memberi tahu
// apa pun kepada pendatang yang memang belum punya kursor untuk digambar.
func (r *Room) sendCursors(subscriber Subscriber) {
	if len(r.cursors) == 0 {
		return
	}

	payload, err := r.encodeCursors()
	if err != nil {
		return
	}

	subscriber.SendEphemeral(cursorStream, payload)
}

func (r *Room) encodeCursors() ([]byte, error) {
	payload, err := r.encoder.EncodeCursors(r.presentCursors())
	if err != nil {
		r.logger.Error("encode document design cursors", "document", r.token, "error", err)
		return nil, err
	}

	return payload, nil
}

// presentCursors menyusun muatan siaran.
//
// Diurutkan menurut id semata-mata supaya isinya dapat diulang: iterasi peta di
// Go berurutan acak, dan tanpa pengurutan dua siaran untuk keadaan yang sama akan
// menghasilkan byte yang berbeda-beda.
func (r *Room) presentCursors() []Cursor {
	cursors := make([]Cursor, 0, len(r.cursors))
	for _, cursor := range r.cursors {
		cursors = append(cursors, cursor)
	}

	slices.SortFunc(cursors, func(a, b Cursor) int {
		return cmp.Compare(a.UserID, b.UserID)
	})

	return cursors
}
