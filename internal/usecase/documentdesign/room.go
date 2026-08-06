package documentdesign

import (
	"context"
	"encoding/json"

	"github.com/mohfakhria/api-widia-kencana/internal/domain"
)

// roomInboxSize membatasi berapa kejadian yang boleh mengantre menuju satu room.
// Pengirim yang menemukannya penuh akan menunggu, sehingga penyunting yang lebih
// cepat daripada laju penerapan tertahan alih-alih menumbuhkan antrean.
const roomInboxSize = 64

// Subscriber adalah satu koneksi yang menerima siaran dari room.
//
// Send wajib tidak memblokir. Orchestrator memanggilnya saat menyiarkan, dan
// implementasi yang menunggu I/O akan menghentikan seluruh penyuntingan dokumen
// itu hanya karena satu klien lambat.
type Subscriber interface {
	Send(payload []byte)
}

// Snapshot adalah isi dokumen pada satu titik waktu.
type Snapshot struct {
	Content json.RawMessage
	Version int64
}

// roomEvent adalah pesan yang diproses orchestrator satu per satu. Antarmuka
// penanda ini membuat himpunan kejadiannya tertutup: hanya tipe di paket ini
// yang bisa masuk ke inbox.
type roomEvent interface {
	isRoomEvent()
}

type joinEvent struct {
	subscriber Subscriber
	// Arah channel dinyatakan lewat tipe: orchestrator hanya boleh mengirim.
	reply chan<- Snapshot
}

type leaveEvent struct {
	subscriber Subscriber
}

func (joinEvent) isRoomEvent()  {}
func (leaveEvent) isRoomEvent() {}

// emptyContent dikembalikan sebagai nilai baru tiap pemanggilan, bukan sebagai
// variabel package, supaya tidak ada pemanggil yang bisa mengubah isi bersama.
func emptyContent() json.RawMessage {
	return json.RawMessage(`{"pages":[]}`)
}

// Room memegang isi satu dokumen selama ada yang menyuntingnya.
//
// Seluruh perubahan mengalir lewat inbox dan diterapkan oleh satu goroutine
// orchestrator, sehingga tidak pernah ada dua penyuntingan yang berjalan
// bersamaan pada dokumen yang sama. Urutan penerapan adalah urutan inbox, bukan
// hasil undian penjadwal.
//
// Pada tahap ini isinya lahir kosong dan hilang saat proses berakhir. Pemuatan
// dan penyimpanan ke database menyusul, dan tempatnya nanti adalah select yang
// sama di run().
type Room struct {
	token string

	inbox chan roomEvent

	// stop ditutup manager untuk menghentikan orchestrator; done ditutup
	// orchestrator saat benar-benar berhenti. Pengirim memakai done sebagai
	// jalan keluar supaya tidak pernah menunggu room yang sudah mati.
	stop chan struct{}
	done chan struct{}

	// Field di bawah ini hanya boleh disentuh goroutine run(). Tidak ada mutex,
	// dan memang tidak boleh ditambahkan: kepemilikan tunggal itulah jaminannya.
	content json.RawMessage
	version int64
	members map[Subscriber]struct{}
}

func newRoom(token string) *Room {
	return &Room{
		token:   token,
		inbox:   make(chan roomEvent, roomInboxSize),
		stop:    make(chan struct{}),
		done:    make(chan struct{}),
		content: emptyContent(),
		members: make(map[Subscriber]struct{}),
	}
}

func (r *Room) Token() string {
	return r.token
}

// run adalah orchestrator dokumen ini. Ia dimiliki manager yang menjalankannya
// saat room dibuat dan menghentikannya dengan menutup stop.
//
// Inbox sengaja tidak pernah ditutup. Menutupnya berarti pengirim bisa panik
// ketika mengirim ke channel tertutup; sebagai gantinya orchestrator menutup
// done saat keluar, dan setiap pengirim menjadikannya jalan keluar.
func (r *Room) run() {
	defer close(r.done)

	for {
		select {
		case <-r.stop:
			return
		case event := <-r.inbox:
			r.handle(event)
		}
	}
}

func (r *Room) handle(event roomEvent) {
	switch e := event.(type) {
	case joinEvent:
		r.members[e.subscriber] = struct{}{}
		// reply berkapasitas satu, jadi pengiriman ini tidak pernah menahan
		// orchestrator walau peminta sudah menyerah menunggu.
		e.reply <- Snapshot{Content: r.snapshotContent(), Version: r.version}
	case leaveEvent:
		delete(r.members, e.subscriber)
	}
}

// snapshotContent menyalin isi sebelum menyerahkannya keluar, supaya penerima
// tidak memegang slice yang masih dipakai orchestrator.
func (r *Room) snapshotContent() json.RawMessage {
	content := make(json.RawMessage, len(r.content))
	copy(content, r.content)

	return content
}

// join menunggu orchestrator memproses pendaftaran lalu mengembalikan isi
// dokumen saat itu. Ini satu-satunya operasi yang sinkron, karena pemanggil
// memang membutuhkan snapshot untuk dikirim ke klien yang baru menempel.
func (r *Room) join(ctx context.Context, sub Subscriber) (Snapshot, error) {
	reply := make(chan Snapshot, 1)
	event := joinEvent{subscriber: sub, reply: reply}

	select {
	case r.inbox <- event:
	case <-r.done:
		return Snapshot{}, domain.NewError(domain.ErrUnavailable, "document design room is closed")
	case <-ctx.Done():
		return Snapshot{}, ctx.Err()
	}

	select {
	case snapshot := <-reply:
		return snapshot, nil
	case <-r.done:
		return Snapshot{}, domain.NewError(domain.ErrUnavailable, "document design room is closed")
	case <-ctx.Done():
		return Snapshot{}, ctx.Err()
	}
}

// leave tidak menunggu hasil. Bila room sudah berhenti, keanggotaan tidak lagi
// berarti apa-apa dan pengiriman cukup dibatalkan.
func (r *Room) leave(sub Subscriber) {
	select {
	case r.inbox <- leaveEvent{subscriber: sub}:
	case <-r.done:
	}
}
