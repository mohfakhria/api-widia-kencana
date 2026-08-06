package documentdesign

import "sync"

// maxConnectionsPerUser membatasi berapa koneksi realtime yang boleh dipegang
// satu user sekaligus.
//
// Kuota tiket tidak menutup ini. Tiket hangus begitu ditukar, sehingga satu user
// dapat mengulang terbitkan-lalu-sambung tanpa henti dan menumbuhkan jumlah
// koneksi tanpa batas — padahal tiap koneksi memakai empat goroutine dan sekitar
// 72 KB. Pencacah inilah yang membatasi pemakaian sumber daya, bukan kuota tiket.
//
// Sepuluh sudah longgar untuk pemakaian wajar: beberapa tab pada beberapa dokumen
// sekaligus, ditambah ruang untuk koneksi lama yang belum sempat dibersihkan
// setelah jaringan terputus.
const maxConnectionsPerUser = 10

// connectionCounter mencacah koneksi hidup per user.
//
// Entri dihapus begitu pencacahnya nol, supaya peta ini tidak menyimpan jejak
// setiap user yang pernah menyambung.
type connectionCounter struct {
	mu     sync.Mutex
	counts map[string]int
}

func newConnectionCounter() *connectionCounter {
	return &connectionCounter{counts: make(map[string]int)}
}

// acquire mengembalikan jumlah koneksi user setelah percobaan ini, dan apakah
// percobaannya diterima. Jumlahnya ikut dikembalikan karena berguna di log:
// koneksi ganda yang tak disengaja langsung terlihat dari angkanya.
func (c *connectionCounter) acquire(userID string) (int, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.counts[userID] >= maxConnectionsPerUser {
		return c.counts[userID], false
	}
	c.counts[userID]++

	return c.counts[userID], true
}

func (c *connectionCounter) release(userID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	count, ok := c.counts[userID]
	if !ok {
		return
	}
	if count <= 1 {
		delete(c.counts, userID)
		return
	}

	c.counts[userID] = count - 1
}
