package http

import "sync"

// designQueue adalah antrean satu arah milik satu klien, dilindungi mutex dan
// dibangunkan lewat sync.Cond.
//
// Panjangnya dibatasi. Antrean tanpa batas akan tumbuh sesuka hati ketika klien
// lebih lambat daripada laju pesan, dan itu kebocoran memori yang hanya terlihat
// saat produksi sedang sibuk.
//
// Perlu diingat sync.Cond tidak mengenal context: goroutine yang menunggu di
// Wait hanya bangun oleh Signal atau Broadcast. Karena itu close wajib dipanggil
// saat koneksi berakhir, kalau tidak penunggunya menggantung selamanya.
type designQueue struct {
	mu     sync.Mutex
	cond   *sync.Cond
	items  [][]byte
	limit  int
	closed bool
}

func newDesignQueue(limit int) *designQueue {
	queue := &designQueue{limit: limit}
	queue.cond = sync.NewCond(&queue.mu)

	return queue
}

// enqueue tidak pernah memblokir sehingga aman dipanggil broadcaster sambil
// memegang kunci room. Mengembalikan false bila antrean sudah ditutup atau
// penuh; pemanggil yang memutuskan apa artinya.
func (q *designQueue) enqueue(payload []byte) bool {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.closed || len(q.items) >= q.limit {
		return false
	}

	q.items = append(q.items, payload)
	q.cond.Signal()

	return true
}

// dequeue menunggu sampai ada isi lalu mengembalikan seluruh antrean sekaligus,
// supaya penulis dapat menguras banyak pesan dalam satu kali bangun.
//
// Isi yang tersisa tetap dikembalikan walau antrean sudah ditutup, sehingga
// pesan yang terlanjur mengantre tidak hilang saat koneksi berakhir. Nilai false
// hanya muncul ketika antrean ditutup dan benar-benar kosong.
func (q *designQueue) dequeue() ([][]byte, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()

	for len(q.items) == 0 && !q.closed {
		q.cond.Wait()
	}
	if len(q.items) == 0 {
		return nil, false
	}

	batch := q.items
	q.items = nil

	return batch, true
}

func (q *designQueue) close() {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.closed {
		return
	}

	q.closed = true
	q.cond.Broadcast()
}

// designBuffer adalah satu buffer untuk satu klien, berisi kedua arah.
//
// Masing-masing arah punya kunci dan cond sendiri agar pembaca dan penulis tidak
// pernah saling menunggu: klien yang membanjiri jalur masuk tidak menghambat
// pengiriman keluar, dan sebaliknya.
type designBuffer struct {
	inbound  *designQueue
	outbound *designQueue
}

func newDesignBuffer(limit int) *designBuffer {
	return &designBuffer{
		inbound:  newDesignQueue(limit),
		outbound: newDesignQueue(limit),
	}
}

func (b *designBuffer) close() {
	b.inbound.close()
	b.outbound.close()
}
