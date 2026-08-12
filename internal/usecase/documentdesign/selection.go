package documentdesign

import (
	"cmp"
	"slices"
)

// maxSelectedElements membatasi berapa id yang boleh dibawa satu seleksi.
//
// Ini masukan tak tepercaya yang DISIARKAN ULANG ke setiap penghuni: tanpa
// batas, satu klien dapat mengirim seratus ribu id dan server menggandakannya
// sebanyak jumlah orang di ruangan. Kursor kebal karena bentuknya tetap; daftar
// tidak.
//
// Angkanya kebetulan sama dengan design.MaxPages, dan sengaja TIDAK disambungkan
// ke sana. Keduanya membatasi hal yang berbeda karena alasan yang berbeda;
// menautkannya berarti menaikkan batas jumlah halaman diam-diam mengubah batas
// seleksi, dan hubungan seperti itu tidak akan terlihat oleh siapa pun yang
// mengubah salah satunya.
const maxSelectedElements = 200

// selectionStream menandai seluruh siaran seleksi sebagai satu aliran
// nilai-terakhir, terpisah dari aliran kursor.
//
// Terpisah, bukan digabung, walau keduanya sama-sama kehadiran: kursor bergerak
// puluhan kali per detik sedangkan seleksi berubah beberapa kali. Satu aliran
// bersama berarti setiap gerakan kursor ikut mengirim ulang seluruh daftar
// seleksi semua orang.
const selectionStream = "selection"

// broadcastSelections mengirim seleksi seluruh orang ke semua penghuni.
//
// Menumpang denyut kursor, tidak disiarkan saat diterima. Seleksi memang jarang
// berubah, tetapi seret marquee dapat menderas — dan menumpang denyut yang sudah
// ada membatasi lajunya tanpa mesin baru.
//
// Berbeda dari kursor dalam satu hal: peta yang KOSONG tetap disiarkan. Orang
// terakhir yang membatalkan pilihannya menghapus entrinya dari peta, dan
// siaran berisi daftar kosong itulah satu-satunya cara sorotannya hilang dari
// layar orang lain.
//
// Berbeda pula dalam penjaganya. Kursor berhenti pada kurang dari DUA penghuni
// karena kursor sendiri tidak berguna bagi pemiliknya; seleksi berhenti pada
// NOL. Bedanya menggigit tepat saat seseorang pergi: kepergiannya menghapus
// entrinya dan menyalakan penanda kotor, tetapi bila yang tersisa cuma satu
// orang, penjaga bergaya kursor akan memblokir justru siaran yang membersihkan
// sorotan hantu dari layar orang itu — dan tidak ada siaran berikutnya yang
// akan memperbaikinya, karena tidak ada lagi yang mengubah seleksi.
//
// Ongkos dari penjaga yang lebih longgar: orang yang sedang sendirian menerima
// kembali seleksinya sendiri, satu pesan kecil per denyut selama ia masih
// mengubah pilihan. Itu jauh lebih murah daripada sorotan yang tidak bisa
// dihilangkan dengan cara apa pun selain memuat ulang halaman.
func (r *Room) broadcastSelections() {
	if !r.selectionsDirty || len(r.members) == 0 {
		return
	}
	r.selectionsDirty = false

	payload, err := r.encodeSelections()
	if err != nil {
		return
	}

	for subscriber := range r.members {
		subscriber.SendEphemeral(selectionStream, payload)
	}
}

// sendSelections mengirim seleksi yang sudah ada kepada satu orang saja, dipakai
// saat ia baru bergabung.
//
// Dilewati bila belum ada seleksi sama sekali, sama seperti kursor: daftar
// kosong tidak memberi tahu apa pun kepada pendatang yang layarnya memang masih
// bersih.
func (r *Room) sendSelections(subscriber Subscriber) {
	if len(r.selections) == 0 {
		return
	}

	payload, err := r.encodeSelections()
	if err != nil {
		return
	}

	subscriber.SendEphemeral(selectionStream, payload)
}

func (r *Room) encodeSelections() ([]byte, error) {
	payload, err := r.encoder.EncodeSelections(r.presentSelections())
	if err != nil {
		r.logger.Error("encode document design selections", "document", r.token, "error", err)
		return nil, err
	}

	return payload, nil
}

// presentSelections menyusun muatan siaran, diurutkan menurut id orangnya supaya
// isinya dapat diulang — iterasi peta di Go berurutan acak.
func (r *Room) presentSelections() []Selection {
	selections := make([]Selection, 0, len(r.selections))
	for userID, ids := range r.selections {
		selections = append(selections, Selection{UserID: userID, ElementIDs: ids})
	}

	slices.SortFunc(selections, func(a, b Selection) int {
		return cmp.Compare(a.UserID, b.UserID)
	})

	return selections
}

// applySelection menyimpan pilihan satu orang.
//
// Daftar yang kelewat panjang DIPOTONG, bukan ditolak. Penolakan tiba di klien
// sebagai malformed_message, dan penanganan yang berlaku untuk kode itu adalah
// meminta seluruh dokumen ulang lalu menampilkan galat kepada penggunanya —
// akibat yang sama sekali tidak sebanding dengan sebabnya, yaitu satu seret
// marquee yang kebetulan melewati dua ratus elemen. Kehadiran memudar, ia tidak
// gagal.
//
// Daftar kosong menghapus entrinya, bukan menyimpan daftar kosong. Peta karena
// itu hanya berisi orang yang benar-benar sedang memilih sesuatu, dan siarannya
// tetap memberi tahu penerima bahwa sorotan orang itu harus hilang — yang
// disiarkan seluruh peta, sehingga yang tidak ada di dalamnya berarti tidak
// memilih apa-apa.
func (r *Room) applySelection(selection Selection) {
	if len(selection.ElementIDs) == 0 {
		if _, had := r.selections[selection.UserID]; !had {
			return
		}
		delete(r.selections, selection.UserID)
		r.selectionsDirty = true

		return
	}

	ids := selection.ElementIDs
	if len(ids) > maxSelectedElements {
		ids = ids[:maxSelectedElements]
	}

	r.selections[selection.UserID] = ids
	r.selectionsDirty = true
}
