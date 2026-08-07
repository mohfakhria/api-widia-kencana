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
	items  []queueItem
	limit  int
	closed bool
}

// queueItem adalah satu pesan beserta kunci penggabungnya.
//
// Kunci kosong berarti pesan berdiri sendiri: snapshot, presence, dan error
// masing-masing adalah fakta tersendiri yang tidak boleh saling menimpa. Kunci
// terisi berarti pesan itu bagian dari satu aliran nilai-terakhir, dan yang baru
// menggantikan yang lama.
type queueItem struct {
	key     string
	payload []byte
}

func newDesignQueue(limit int) *designQueue {
	queue := &designQueue{limit: limit}
	queue.cond = sync.NewCond(&queue.mu)

	return queue
}

// enqueue menaruh pesan yang berdiri sendiri.
//
// Tidak pernah memblokir sehingga aman dipanggil broadcaster sambil memegang
// kunci room. Mengembalikan false bila antrean sudah ditutup atau penuh;
// pemanggil yang memutuskan apa artinya.
func (q *designQueue) enqueue(payload []byte) bool {
	return q.push("", payload)
}

// conflate menaruh payload sebagai nilai terbaru untuk satu aliran.
//
// Bila masih ada pesan dengan kunci yang sama menunggu di antrean, isinya
// DIGANTI di tempat — bukan ditambahkan di belakangnya. Itu yang membuat klien
// yang tersendat menerima posisi terkini saat ia menyusul, bukan tumpukan posisi
// basi yang sudah tidak berarti.
//
// Menggantinya di tempat, bukan memindahkan ke belakang, menjaga urutan
// pesan ini terhadap jenis pesan lain tetap seperti saat ia pertama masuk.
func (q *designQueue) conflate(key string, payload []byte) bool {
	return q.push(key, payload)
}

func (q *designQueue) push(key string, payload []byte) bool {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.closed {
		return false
	}

	if key != "" {
		for index := range q.items {
			if q.items[index].key != key {
				continue
			}

			// Tidak perlu Signal. Antrean yang berisi berarti penunggunya sudah
			// dibangunkan ketika isian ini pertama masuk — dequeue hanya menunggu
			// selagi antrean kosong.
			q.items[index].payload = payload

			return true
		}
	}

	if len(q.items) >= q.limit {
		return false
	}

	q.items = append(q.items, queueItem{key: key, payload: payload})
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

	batch := make([][]byte, 0, len(q.items))
	for _, item := range q.items {
		batch = append(batch, item.payload)
	}
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
