package documentdesign

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/mohfakhria/api-widia-kencana/internal/domain"
	"github.com/mohfakhria/api-widia-kencana/internal/domain/design"
	"github.com/mohfakhria/api-widia-kencana/internal/domain/entity"
	"github.com/mohfakhria/api-widia-kencana/internal/usecase/port/output"
)

const (
	// roomInboxSize membatasi berapa kejadian yang boleh mengantre menuju satu
	// room. Pengirim yang menemukannya penuh akan menunggu, sehingga penyunting
	// yang lebih cepat daripada laju penerapan tertahan alih-alih menumbuhkan
	// antrean.
	roomInboxSize = 64

	// flushInterval adalah jarak antar penyimpanan ke database. Kerugian maksimal
	// saat proses mati mendadak sebesar ini, selalu.
	flushInterval = 2 * time.Second

	contentLoadTimeout = 5 * time.Second

	// contentSaveTimeout membatasi satu kali penulisan. Tiga detik sudah lebih
	// dari cukup untuk memperbarui satu baris; bila lebih lama dari itu ada yang
	// tidak beres, dan mencoba lagi pada denyut berikutnya adalah jawaban yang
	// benar.
	contentSaveTimeout = 3 * time.Second

	// drainSaveWait sengaja LEBIH PANJANG daripada contentSaveTimeout.
	//
	// Goroutine penyimpan selalu mengirim hasilnya dalam batas tenggatnya
	// sendiri, jadi penantian yang lebih panjang membuat hasil itu praktis selalu
	// tiba lebih dulu. Bila keduanya disamakan, timer dapat menang tipis dan
	// drain menyerah tepat sebelum hasilnya datang — lalu keluar tanpa pernah
	// mencoba penyimpanan terakhir. Cabang timeout di bawah hanya menjaga
	// kemungkinan driver yang mengabaikan context.
	drainSaveWait = contentSaveTimeout + time.Second
)

// Subscriber adalah satu koneksi yang menerima siaran dari room.
//
// Send wajib tidak memblokir. Orchestrator memanggilnya saat menyiarkan, dan
// implementasi yang menunggu I/O akan menghentikan seluruh penyuntingan dokumen
// itu hanya karena satu klien lambat. Disconnect tunduk pada aturan yang sama.
type Subscriber interface {
	Send(payload []byte)

	// Disconnect memutus koneksi ini beserta alasannya. Dipanggil ketika room
	// tidak lagi dapat melayani — membiarkan orang menyunting sesuatu yang tidak
	// akan pernah tersimpan jauh lebih buruk daripada memutusnya.
	Disconnect(reason string)
}

// MessageEncoder menyusun payload yang dikirim room ke klien.
//
// Room tidak mengenal bentuk kawatnya; lapisan delivery yang menentukan. Port
// ini ada supaya orchestrator dapat memasukkan snapshot ke antrean keluar
// sendiri, pada langkah yang sama dengan pendaftaran anggota. Bila penyusunan
// payload diserahkan ke goroutine handler, siaran dari perubahan berikutnya
// dapat menyalip snapshot dan klien menerima delta untuk keadaan yang belum ia
// punya.
type MessageEncoder interface {
	EncodeSnapshot(content json.RawMessage, version int64) ([]byte, error)
}

// roomEvent adalah pesan yang diproses orchestrator satu per satu. Antarmuka
// penanda ini membuat himpunan kejadiannya tertutup: hanya tipe di paket ini
// yang bisa masuk ke inbox.
type roomEvent interface {
	isRoomEvent()
}

// syncEvent adalah permintaan klien atas isi dokumen.
//
// Klien baru menjadi anggota — dan karenanya penerima siaran — pada langkah ini,
// bukan saat socket terbuka. Dengan begitu mustahil ada delta yang sampai
// sebelum snapshot yang menjadi dasarnya.
type syncEvent struct {
	subscriber Subscriber
	// Arah channel dinyatakan lewat tipe: orchestrator hanya boleh mengirim.
	reply chan<- error
}

type leaveEvent struct {
	subscriber Subscriber
}

// snapshotEvent meminta salinan isi terkini, dipakai ekspor.
//
// Ekspor wajib lewat sini dan bukan membaca database, karena penyimpanan bersifat
// tunda: perubahan terakhir bisa tertinggal sampai satu denyut flush penuh. Tanpa
// jalur ini pengguna dapat menggeser sebuah elemen lalu mengekspor dan menerima
// PDF yang belum memuat geseran itu.
type snapshotEvent struct {
	reply chan<- snapshotResult
}

type snapshotResult struct {
	content json.RawMessage
	version int64
	paper   entity.DocumentPaperSize
	err     error
}

func (syncEvent) isRoomEvent()     {}
func (leaveEvent) isRoomEvent()    {}
func (snapshotEvent) isRoomEvent() {}

// saveResult dikirim goroutine penyimpan kembali ke orchestrator.
type saveResult struct {
	version int64
	err     error
}

// Room memegang isi satu dokumen selama ada yang menyuntingnya.
//
// Seluruh perubahan mengalir lewat inbox dan diterapkan oleh satu goroutine
// orchestrator, sehingga tidak pernah ada dua penyuntingan yang berjalan
// bersamaan pada dokumen yang sama. Urutan penerapan adalah urutan inbox, bukan
// hasil undian penjadwal.
type Room struct {
	token     string
	documents output.DocumentRepository
	encoder   MessageEncoder
	logger    *slog.Logger

	inbox chan roomEvent
	saved chan saveResult

	// stop ditutup manager untuk menghentikan orchestrator; done ditutup
	// orchestrator saat benar-benar berhenti. Pengirim memakai done sebagai
	// jalan keluar supaya tidak pernah menunggu room yang sudah mati.
	stop chan struct{}
	done chan struct{}

	// Field di bawah ini hanya boleh disentuh goroutine run(). Tidak ada mutex,
	// dan memang tidak boleh ditambahkan: kepemilikan tunggal itulah jaminannya.
	content *design.Content
	// paper disimpan karena ekspor membutuhkan ukuran halaman, sedangkan isi
	// dokumen sengaja tidak memuatnya — ukuran kertas adalah milik dokumen, bukan
	// milik tiap halaman, dan menyalinnya ke dalam isi hanya membuka peluang
	// keduanya bertentangan.
	paper entity.DocumentPaperSize
	// version adalah nomor revisi di memori; savedVersion adalah nilai version
	// yang terakhir berhasil ditulis. Kotor berarti keduanya berbeda — bukan
	// boolean, karena boolean bisa terhapus keliru oleh perubahan yang masuk
	// selama penulisan berlangsung.
	version      int64
	savedVersion int64
	saving       bool
	// broken diisi bila room tidak lagi dapat melayani: isinya gagal dimuat, atau
	// gagal disimpan secara permanen. Sekali terisi, join ditolak dan penyimpanan
	// tidak dicoba lagi.
	//
	// Pesannya sengaja berupa frasa yang aman disampaikan ke klien; detail
	// internal penyebabnya hanya masuk log, di tempat dan saat ia terjadi.
	broken error
	// members hanya berisi klien yang sudah meminta dan menerima snapshot.
	// Koneksi yang terbuka tetapi belum meminta dokumen sengaja tidak masuk,
	// supaya ia tidak pernah menerima delta untuk keadaan yang belum ia punya.
	members map[Subscriber]struct{}
}

func newRoom(token string, documents output.DocumentRepository, encoder MessageEncoder, logger *slog.Logger) *Room {
	return &Room{
		token:     token,
		documents: documents,
		encoder:   encoder,
		logger:    logger,
		inbox:     make(chan roomEvent, roomInboxSize),
		saved:     make(chan saveResult, 1),
		stop:      make(chan struct{}),
		done:      make(chan struct{}),
		content:   &design.Content{Pages: []design.Page{}},
		members:   make(map[Subscriber]struct{}),
	}
}

// run adalah orchestrator dokumen ini. Ia dimiliki manager yang menjalankannya
// saat room dibuat dan menghentikannya dengan menutup stop.
//
// Inbox sengaja tidak pernah ditutup. Menutupnya berarti pengirim bisa panik
// ketika mengirim ke channel tertutup; sebagai gantinya orchestrator menutup
// done saat keluar, dan setiap pengirim menjadikannya jalan keluar.
func (r *Room) run(ctx context.Context) {
	defer close(r.done)

	// Pemuatan terjadi sebelum inbox dilayani. Penempel pertama menunggunya lewat
	// reply channel, penempel berikutnya mengantre di inbox di belakangnya —
	// sehingga tidak pernah ada dua query untuk dokumen yang sama.
	r.load(ctx)

	ticker := time.NewTicker(flushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-r.stop:
			r.drain(ctx)
			return
		case event := <-r.inbox:
			r.handle(event)
		case result := <-r.saved:
			r.handleSaved(result)
		case <-ticker.C:
			r.flush(ctx)
		}
	}
}

func (r *Room) load(ctx context.Context) {
	loadCtx, cancel := context.WithTimeout(ctx, contentLoadTimeout)
	defer cancel()

	stored, err := r.documents.GetContent(loadCtx, r.token)
	if err != nil {
		// Room tidak dapat melayani apa pun tanpa isinya. Setiap join ditolak
		// dengan error ini, penghuni tidak pernah bertambah, dan room tersapu
		// setelah masa tenggangnya — percobaan berikutnya memuat ulang. Sembuh
		// sendiri tanpa logika retry.
		r.markBroken(fmt.Errorf("load document content: %w", err),
			"document content could not be loaded")
		return
	}

	content, err := design.Decode(stored.Content)
	if err != nil {
		// Isi di database cacat, atau memakai bentuk lama yang sudah tidak dikenal
		// skema tertutup. Memuat ulang tidak akan memperbaikinya; yang dibutuhkan
		// adalah menyetel ulang kolom content dokumen tersebut.
		r.markBroken(fmt.Errorf("parse document content: %w", err),
			"document content is malformed")
		return
	}

	r.content = content
	r.paper = stored.Paper
	r.version = stored.Version
	r.savedVersion = stored.Version

	if content.IsEmpty() {
		// Dokumen yang belum punya halaman diisi benih agar kanvas tidak hampa.
		//
		// version dinaikkan supaya benihnya ikut tersimpan pada penyimpanan
		// berikutnya. Tanpa itu ia disusun ulang setiap kali room lahir, dan
		// elemennya mendapat id baru setiap kali — membuat setiap penyuntingan
		// yang menunjuk id lama gagal.
		r.content = defaultDocumentContent(stored.Paper)
		r.version++
		r.logger.Info("seeded empty document design content",
			"document", r.token, "version", r.version)
	}
}

func (r *Room) handle(event roomEvent) {
	switch e := event.(type) {
	case syncEvent:
		// reply berkapasitas satu, jadi pengiriman apa pun di cabang ini tidak
		// pernah menahan orchestrator walau peminta sudah menyerah menunggu.
		if r.broken != nil {
			e.reply <- r.broken
			return
		}

		payload, err := r.encodeSnapshot()
		if err != nil {
			// Isi yang tidak dapat dikodekan juga tidak akan pernah dapat
			// disimpan, jadi room ini memang sudah tidak berguna.
			r.markBroken(err, "document content can no longer be encoded")
			e.reply <- r.broken
			return
		}

		// Pendaftaran anggota dan pengiriman snapshot terjadi pada langkah yang
		// sama. Itulah yang menjamin klien tidak pernah menerima delta untuk
		// keadaan yang belum ia punya.
		r.members[e.subscriber] = struct{}{}
		e.subscriber.Send(payload)
		e.reply <- nil
	case leaveEvent:
		delete(r.members, e.subscriber)
	case snapshotEvent:
		if r.broken != nil {
			e.reply <- snapshotResult{err: r.broken}
			return
		}

		content, err := r.content.Encode()
		if err != nil {
			r.markBroken(err, "document content can no longer be encoded")
			e.reply <- snapshotResult{err: r.broken}
			return
		}

		e.reply <- snapshotResult{
			content: content,
			version: r.version,
			paper:   r.paper,
		}
	}
}

// encodeSnapshot menyusun payload snapshot dari isi terkini.
//
// Hasil encode berupa byte baru, jadi klien tidak pernah memegang struktur yang
// masih dipakai orchestrator.
func (r *Room) encodeSnapshot() ([]byte, error) {
	content, err := r.content.Encode()
	if err != nil {
		return nil, err
	}

	return r.encoder.EncodeSnapshot(content, r.version)
}

// flush menyerahkan penulisan ke goroutine terpisah.
//
// Menulis di dalam orchestrator akan menghentikan seluruh penyuntingan dokumen
// ini selama query berlangsung — beberapa milidetik pada keadaan normal, tetapi
// bisa jauh lebih lama saat database tersendat.
//
// Paling banyak satu penulisan berjalan per room. Perubahan yang masuk selama
// penulisan otomatis membuat version melewati versi yang sedang ditulis,
// sehingga room tetap kotor dan ikut tertulis pada denyut berikutnya.
func (r *Room) flush(ctx context.Context) {
	if r.broken != nil || r.saving || r.version == r.savedVersion {
		return
	}

	content, err := r.content.Encode()
	if err != nil {
		r.markBroken(err, "document content can no longer be encoded")
		return
	}

	r.saving = true
	fromVersion, toVersion := r.savedVersion, r.version

	go func() {
		// Penyimpanan sengaja lepas dari pembatalan context aplikasi: memutus
		// penulisan di tengah jalan hanya membuang pekerjaan pengguna, sedangkan
		// tenggatnya sudah membatasi.
		saveCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), contentSaveTimeout)
		defer cancel()

		err := r.documents.SaveContent(saveCtx, r.token, content, fromVersion, toVersion)

		select {
		case r.saved <- saveResult{version: toVersion, err: err}:
		case <-r.done:
		}
	}()
}

func (r *Room) handleSaved(result saveResult) {
	r.saving = false

	if result.err == nil {
		r.savedVersion = result.version
		return
	}

	if isPermanentSaveFailure(result.err) {
		r.markBroken(fmt.Errorf("save document content: %w", result.err),
			"document content can no longer be saved")
		return
	}

	// Kegagalan sementara: biarkan tetap kotor dan coba lagi pada denyut
	// berikutnya. Tidak ada yang hilang selama room masih hidup.
	r.logger.Warn("save document design content failed, will retry",
		"document", r.token, "error", result.err)
}

// isPermanentSaveFailure memisahkan kegagalan yang tidak akan membaik dengan
// mencoba lagi dari gangguan sementara seperti koneksi database terputus.
//
// ErrNotFound berarti dokumennya sudah dihapus. ErrConflict berarti versinya
// bergeser, yang dengan satu instance seharusnya tidak pernah terjadi —
// kemunculannya adalah sinyal bahwa asumsi itu dilanggar, misalnya ada proses
// kedua atau UPDATE manual.
//
// Keduanya diperlakukan permanen, artinya klien diputus dan kehilangan delta
// yang belum tersimpan. Alternatifnya memuat ulang dari database, yang menahan
// sesinya tetapi tetap membuang delta yang sama. Memutus dipilih karena lebih
// jujur: menimpa penulis lain jauh lebih berbahaya daripada memaksa klien
// menyambung ulang dan melihat keadaan yang sebenarnya.
func isPermanentSaveFailure(err error) bool {
	return errors.Is(err, domain.ErrNotFound) || errors.Is(err, domain.ErrConflict)
}

// disconnectAll memutus seluruh penghuni. Keanggotaannya sendiri dibiarkan
// dibersihkan oleh leaveEvent masing-masing, supaya jalur keluarnya tetap satu.
func (r *Room) disconnectAll(reason string) {
	for member := range r.members {
		member.Disconnect(reason)
	}
}

// drain adalah kesempatan terakhir menyimpan sebelum orchestrator berhenti.
//
// Penulisan yang sedang berjalan ditunggu lebih dulu, kalau tidak compare-and-set
// terakhir akan bentrok dengan penulisan kita sendiri dan justru menandai room
// sebagai rusak.
func (r *Room) drain(ctx context.Context) {
	if r.saving {
		select {
		case result := <-r.saved:
			r.handleSaved(result)
		case <-time.After(drainSaveWait):
			// Hasil penulisan tidak diketahui, jadi compare-and-set berikutnya
			// tidak dapat memilih fromVersion yang benar. Menyerah lebih jujur
			// daripada menebak dan berisiko menimpa.
			r.logger.Warn("gave up waiting for in-flight save, tail changes may be lost",
				"document", r.token)
			return
		}
	}

	if r.broken != nil || r.version == r.savedVersion {
		return
	}

	content, err := r.content.Encode()
	if err != nil {
		r.logger.Error("final save on shutdown could not encode content",
			"document", r.token, "error", err)
		return
	}

	// Context aplikasi sudah dibatalkan saat titik ini tercapai, jadi penyimpanan
	// terakhir wajib memakai context yang lepas darinya.
	saveCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), contentSaveTimeout)
	defer cancel()

	if err := r.documents.SaveContent(saveCtx, r.token, content, r.savedVersion, r.version); err != nil {
		r.logger.Error("final save on shutdown failed", "document", r.token, "error", err)
		return
	}

	r.savedVersion = r.version
}

// markBroken menandai room tidak lagi dapat melayani, mencatat penyebabnya, dan
// memutus penghuninya.
//
// cause adalah penyebab sebenarnya dan hanya masuk log. reason adalah frasa yang
// disampaikan ke klien — baik lewat close frame bagi yang sedang terhubung,
// maupun sebagai error bagi yang baru mencoba menempel. Memisahkan keduanya
// menjaga detail internal tidak bocor sekaligus membuat kedua jalur menyampaikan
// hal yang sama.
//
// Sekali terisi tidak pernah ditimpa: penyebab pertama adalah yang paling
// menjelaskan, sedangkan kegagalan berikutnya hanyalah akibatnya.
func (r *Room) markBroken(cause error, reason string) {
	if r.broken != nil {
		return
	}

	r.broken = domain.NewError(domain.ErrUnavailable, reason)
	r.logger.Error("document design room stopped serving",
		"document", r.token, "reason", reason, "error", cause)
	r.disconnectAll(reason)
}

// sync mendaftarkan klien sebagai anggota lalu meminta orchestrator mengirimkan
// snapshot kepadanya.
//
// Ini satu-satunya operasi yang sinkron, dan yang ditunggu hanya konfirmasi
// berhasil atau tidak — snapshot-nya sendiri dikirim orchestrator langsung ke
// antrean keluar klien.
func (r *Room) sync(ctx context.Context, sub Subscriber) error {
	reply := make(chan error, 1)
	event := syncEvent{subscriber: sub, reply: reply}

	select {
	case r.inbox <- event:
	case <-r.done:
		return domain.NewError(domain.ErrUnavailable, "document design room is closed")
	case <-ctx.Done():
		return ctx.Err()
	}

	select {
	case err := <-reply:
		return err
	case <-r.done:
		return domain.NewError(domain.ErrUnavailable, "document design room is closed")
	case <-ctx.Done():
		return ctx.Err()
	}
}

// errRoomClosed menandai room yang sudah berhenti, dibedakan dari room yang
// rusak.
//
// Keduanya berakhir sebagai ErrUnavailable bagi klien WebSocket, dan karena
// domain.Error membandingkan dirinya lewat Kind saja, errors.Is tidak dapat
// memisahkan keduanya. Ekspor justru harus memisahkannya: room yang berhenti
// berarti database sudah mutakhir dan aman dibaca, sedangkan room yang rusak
// berarti keadaan sebenarnya tidak diketahui. Karena itu penanda tersendiri, dan
// sengaja tidak diekspor — hanya manager yang perlu mengenalinya.
var errRoomClosed = errors.New("document design room stopped")

// snapshot mengambil salinan isi terkini dari orchestrator.
func (r *Room) snapshot(ctx context.Context) (snapshotResult, error) {
	reply := make(chan snapshotResult, 1)

	select {
	case r.inbox <- snapshotEvent{reply: reply}:
	case <-r.done:
		return snapshotResult{}, errRoomClosed
	case <-ctx.Done():
		return snapshotResult{}, ctx.Err()
	}

	select {
	case result := <-reply:
		return result, result.err
	case <-r.done:
		return snapshotResult{}, errRoomClosed
	case <-ctx.Done():
		return snapshotResult{}, ctx.Err()
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
