package documentdesign

import (
	"encoding/json"
	"sync"
)

// Subscriber adalah satu koneksi yang menerima siaran dari room.
//
// Send wajib tidak memblokir. Room memanggilnya sambil memegang kuncinya
// sendiri, sehingga implementasi yang menunggu I/O akan membekukan seluruh
// penghuni room hanya karena satu klien lambat.
type Subscriber interface {
	Send(payload []byte)
}

// emptyContent dikembalikan sebagai nilai baru tiap pemanggilan, bukan sebagai
// variabel package, supaya tidak ada pemanggil yang bisa mengubah isi bersama.
func emptyContent() json.RawMessage {
	return json.RawMessage(`{"pages":[]}`)
}

// Room memegang isi satu dokumen selama ada yang menyuntingnya.
//
// Pada tahap ini isinya lahir kosong dan hilang saat proses berakhir.
// Pemuatan dan penyimpanan ke database menyusul.
type Room struct {
	token string

	mu      sync.Mutex
	content json.RawMessage
	version int64
	members map[Subscriber]struct{}
}

func newRoom(token string) *Room {
	return &Room{
		token:   token,
		content: emptyContent(),
		members: make(map[Subscriber]struct{}),
	}
}

func (r *Room) Token() string {
	return r.token
}

func (r *Room) join(sub Subscriber) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.members[sub] = struct{}{}
}

// leave mengembalikan jumlah penghuni yang tersisa, dipakai manager untuk
// memutuskan apakah room sudah kosong.
func (r *Room) leave(sub Subscriber) int {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.members, sub)
	return len(r.members)
}

// Snapshot mengembalikan salinan isi room beserta versinya. Salinan dibuat
// supaya pemanggil tidak memegang slice yang masih dipakai room.
func (r *Room) Snapshot() (json.RawMessage, int64) {
	r.mu.Lock()
	defer r.mu.Unlock()

	content := make(json.RawMessage, len(r.content))
	copy(content, r.content)

	return content, r.version
}
