package documentdesign

import "sync"

// manager memegang peta token dokumen ke room yang sedang hidup.
//
// Urutan penguncian selalu manager.mu lalu Room.mu, tidak pernah sebaliknya.
// Selama urutan itu dipatuhi tidak ada jalan menuju deadlock.
type manager struct {
	mu    sync.Mutex
	rooms map[string]*Room
}

func newManager() *manager {
	return &manager{rooms: make(map[string]*Room)}
}

// join mencari-atau-membuat room lalu mendaftarkan penghuninya, seluruhnya di
// bawah satu penguncian. Memecahnya jadi "periksa" lalu "buat" akan membuat dua
// koneksi yang datang bersamaan pada dokumen yang sama menghasilkan dua room
// dengan state bercabang.
func (m *manager) join(token string, sub Subscriber) *Room {
	m.mu.Lock()
	defer m.mu.Unlock()

	room, ok := m.rooms[token]
	if !ok {
		room = newRoom(token)
		m.rooms[token] = room
	}
	room.join(sub)

	return room
}

// leave membuang room begitu penghuni terakhir keluar. Pemeriksaan kosong dan
// pembuangan terjadi di bawah kunci yang sama dengan join, sehingga koneksi
// yang menempel pada saat bersamaan tidak bisa mendarat di room yang sedang
// dibuang.
//
// Pembuangan langsung tanpa jeda masih aman pada tahap ini karena isi room
// belum dipersistenkan, jadi tidak ada yang bisa hilang. Jeda 60 detik menyusul
// bersama penyimpanan.
func (m *manager) leave(token string, sub Subscriber) {
	m.mu.Lock()
	defer m.mu.Unlock()

	room, ok := m.rooms[token]
	if !ok {
		return
	}

	if room.leave(sub) == 0 {
		delete(m.rooms, token)
	}
}
