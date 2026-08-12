package documentdesign

import "sync"

// maxConnectionsPerUser membatasi berapa koneksi realtime yang boleh dipegang
// satu user sekaligus.
//
// Kuota tiket TIDAK menutup ini. Tiket hangus begitu ditukar, sehingga satu user
// dapat mengulang terbitkan-lalu-sambung tanpa henti dan menumbuhkan koneksi
// tanpa batas. Pencacah inilah yang membatasi sumber daya, bukan kuota tiket.
const maxConnectionsPerUser = 10

// connectionCounter mencacah koneksi hidup per user.
//
// Entri dihapus begitu pencacahnya nol, supaya peta ini tidak menyimpan jejak
// setiap user yang pernah menyambung.
type connectionCounter struct {
	mu     sync.Mutex
	counts map[int64]int
}

func newConnectionCounter() *connectionCounter {
	return &connectionCounter{counts: make(map[int64]int)}
}

// acquire mengembalikan jumlah koneksi user setelah percobaan ini, dan apakah
// percobaannya diterima. Jumlahnya ikut dikembalikan karena berguna di log:
// koneksi ganda yang tak disengaja langsung terlihat dari angkanya.
func (c *connectionCounter) acquire(userID int64) (int, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.counts[userID] >= maxConnectionsPerUser {
		return c.counts[userID], false
	}
	c.counts[userID]++

	return c.counts[userID], true
}

func (c *connectionCounter) release(userID int64) {
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
