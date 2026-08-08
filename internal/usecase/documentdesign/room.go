package documentdesign

import (
	"context"
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
	// room. Pengirim yang menemukannya penuh akan menunggu — dan penantian itu
	// berujung pada koneksi klien yang diakhiri, jadi kelonggarannya dibuat besar.
	//
	// Ongkosnya 256 KB PER ROOM, dialokasikan di muka karena channel memesan
	// seluruh slotnya sekaligus. Rantai kegagalan lengkapnya ada di
	// document-design-architecture.md.
	roomInboxSize = 16384

	contentLoadTimeout = 5 * time.Second
)

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
	// keduanya bertentangan. Bentuk aslinya dipertahankan untuk ekspor, yang juga
	// melayani dokumen tanpa room dan karena itu mengonversi sendiri.
	paper entity.DocumentPaperSize
	// page adalah ukuran yang sama dalam titik. Dikonversi sekali saat memuat,
	// bukan tiap snapshot, dan satuan yang tidak dikenal ditolak di sana — sehingga
	// jalur snapshot maupun benih tidak perlu lagi memikirkan kegagalan konversi.
	page PageSize
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
	// supaya ia tidak pernah menerima delta untuk keadaan yang belum ia punya —
	// dan supaya ia belum terhitung hadir, karena ia memang belum melihat apa pun.
	members map[Subscriber]Member
	// cursors dikunci id orang, isinya menang-terakhir. Disimpan, bukan sekadar
	// diteruskan, supaya orang yang baru bergabung dapat langsung diberi seluruh
	// kursor yang sudah ada — dan supaya kursor milik orang yang pergi dapat
	// dibuang.
	cursors map[string]Cursor
	// cursorsDirty menandai ada yang berubah sejak siaran terakhir. Tanpa ini
	// denyut akan mengirim ulang posisi yang sama dua puluh kali per detik walau
	// tidak ada satu pun yang bergerak.
	cursorsDirty bool
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
		members:   make(map[Subscriber]Member),
		cursors:   make(map[string]Cursor),
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

	cursorTicker := time.NewTicker(cursorTickInterval)
	defer cursorTicker.Stop()

	for {
		select {
		case <-r.stop:
			// Menerapkan yang terlanjur mengantre HARUS mendahului penyimpanan
			// terakhir, kalau tidak perubahan yang sudah diterima klien sebagai
			// berhasil justru tidak ikut tertulis.
			r.consume()
			r.drain(ctx)
			return
		case event := <-r.inbox:
			r.handle(event)
		case result := <-r.saved:
			r.handleSaved(result)
		case <-ticker.C:
			r.flush(ctx)
		case <-cursorTicker.C:
			r.broadcastCursors()
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

	width, height, ok := design.PaperPoints(stored.Paper.Width, stored.Paper.Height, stored.Paper.Unit)
	if !ok {
		// Tanpa ukuran halaman, frontend tidak dapat menggambar apa pun dan ekspor
		// tidak dapat menentukan ukuran kertas. Menolak di sini lebih jujur daripada
		// melayani sesi yang hasilnya pasti salah.
		r.markBroken(fmt.Errorf("document paper unit %q is not supported", stored.Paper.Unit),
			"document paper uses an unsupported unit")
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
	r.page = PageSize{Width: width, Height: height}
	r.version = stored.Version
	r.savedVersion = stored.Version

	if content.IsEmpty() {
		// Dokumen yang belum punya halaman diisi benih agar kanvas tidak hampa.
		//
		// version dinaikkan supaya benihnya ikut tersimpan pada penyimpanan
		// berikutnya. Tanpa itu ia disusun ulang setiap kali room lahir, dan
		// elemennya mendapat id baru setiap kali — membuat setiap penyuntingan
		// yang menunjuk id lama gagal.
		r.content = defaultDocumentContent(r.page)
		r.version++
		r.logger.Info("seeded empty document design content",
			"document", r.token, "version", r.version)
	}
}

// consume menghabiskan kejadian yang masih mengantre saat room diminta berhenti.
//
// Tanpa ini isi inbox dibuang begitu saja. Hari ini isinya hanya kursor sehingga
// tidak ada yang hilang, tetapi begitu penyuntingan masuk, suntingan yang tiba
// tepat sebelum shutdown akan lenyap tanpa jejak — klien sudah menganggapnya
// berhasil karena memang tidak ada yang memberitahunya sebaliknya.
//
// Kejadian yang datang SETELAH ini tetap hilang, terutama selama drain yang bisa
// memakan beberapa detik. Itulah alasan urutannya begini: dikuras sedekat mungkin
// dengan penyimpanan terakhir, bukan di awal jalur berhenti.
func (r *Room) consume() {
	for {
		select {
		case event := <-r.inbox:
			if e, ok := event.(syncEvent); ok {
				// Bergabung sengaja DITOLAK, tidak diteruskan ke handle. Menerimanya
				// berarti menjadikan seseorang anggota room yang sedang mati: ia tidak
				// akan pernah menerima siaran apa pun dan tidak punya cara mengetahui
				// itu. Ditolak, ia menyambung ulang dan mendarat di room berikutnya.
				e.reply <- domain.NewError(domain.ErrUnavailable, "document design room is closed")
				continue
			}

			r.handle(event)
		default:
			return
		}
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

		// Diperiksa sebelum didaftarkan: yang menentukan perlu-tidaknya siaran
		// adalah apakah ORANGNYA sudah hadir, bukan koneksinya.
		newcomer := !r.hasUser(e.member.UserID)

		// Pendaftaran anggota dan pengiriman snapshot terjadi pada langkah yang
		// sama. Itulah yang menjamin klien tidak pernah menerima delta untuk
		// keadaan yang belum ia punya.
		r.members[e.member.Subscriber] = e.member
		e.member.Subscriber.Send(payload)

		// Snapshot lebih dulu, baru daftar orangnya. Keduanya masuk antrean pada
		// langkah yang sama, jadi urutan itu terjamin sampai ke klien.
		if newcomer {
			r.broadcastPresence()
		} else {
			// Tab kedua milik orang yang sama, atau document.get yang diulang
			// sebagai jalur pemulihan. Daftar tidak berubah bagi yang lain, tetapi
			// peminta tetap perlu menerimanya.
			r.sendPresence(e.member.Subscriber)
		}

		// Kursor yang sudah ada dikirim langsung, tidak menunggu denyut. Denyut
		// hanya menyiarkan saat ada yang berubah, jadi pendatang yang bergabung
		// ketika semua orang sedang diam tidak akan pernah melihat satu kursor pun.
		r.sendCursors(e.member.Subscriber)

		// Balasan paling akhir, supaya kembalinya Sync berarti seluruh pesan untuk
		// bergabung sudah masuk antrean — bukan sebagian. Ongkosnya hanya beberapa
		// penambahan ke antrean, dan sebagai gantinya perilakunya dapat ditalar
		// tanpa memikirkan apa yang masih tertinggal di belakang.
		e.reply <- nil
	case leaveEvent:
		member, joined := r.members[e.subscriber]
		delete(r.members, e.subscriber)

		// Orang yang masih memegang tab lain belum benar-benar pergi, jadi tidak
		// ada yang berubah untuk disiarkan.
		if joined && !r.hasUser(member.UserID) {
			// Orang yang benar-benar pergi tidak boleh meninggalkan kursornya
			// menggantung di layar orang lain. Siarannya diserahkan ke denyut;
			// kehadiran tidak, karena ia tidak punya denyut sendiri.
			if _, had := r.cursors[member.UserID]; had {
				delete(r.cursors, member.UserID)
				r.cursorsDirty = true
			}
			r.broadcastPresence()
		}
	case cursorMoveEvent:
		// Kursor hanya milik anggota. Koneksi yang sudah terbuka tetapi belum
		// meminta dokumen belum hadir bagi siapa pun — namanya tidak ada di
		// presence — sehingga kursornya akan muncul di layar orang lain sebagai id
		// yang tidak dapat dipetakan ke siapa pun.
		if !r.hasUser(e.cursor.UserID) {
			return
		}

		// Seluruh kerja untuk satu gerakan cuma dua baris ini. Penyandian dan
		// siarannya menunggu denyut.
		r.cursors[e.cursor.UserID] = e.cursor
		r.cursorsDirty = true
	case elementCreateEvent:
		r.applyCreate(e)
	case elementUpdateEvent:
		r.applyUpdate(e)
	case elementDeleteEvent:
		r.applyDelete(e)
	case elementReorderEvent:
		r.applyReorder(e)
	case pageCreateEvent:
		r.applyPageCreate(e)
	case pageUpdateEvent:
		r.applyPageUpdate(e)
	case pageDeleteEvent:
		r.applyPageDelete(e)
	case pageReorderEvent:
		r.applyPageReorder(e)
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

	return r.encoder.EncodeSnapshot(content, r.version, r.page)
}

// hasUser menjawab apakah seseorang masih memegang setidaknya satu koneksi ke
// dokumen ini.
func (r *Room) hasUser(userID string) bool {
	for _, member := range r.members {
		if member.UserID == userID {
			return true
		}
	}

	return false
}

// disconnectAll memutus seluruh penghuni. Keanggotaannya sendiri dibiarkan
// dibersihkan oleh leaveEvent masing-masing, supaya jalur keluarnya tetap satu.
func (r *Room) disconnectAll(reason string) {
	for member := range r.members {
		member.Disconnect(reason)
	}
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
func (r *Room) sync(ctx context.Context, member Member) error {
	reply := make(chan error, 1)
	event := syncEvent{member: member, reply: reply}

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

// moveCursor tidak menunggu hasil, sama seperti leave.
func (r *Room) moveCursor(cursor Cursor) {
	select {
	case r.inbox <- cursorMoveEvent{cursor: cursor}:
	case <-r.done:
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

// createElement menunggu hasil, berbeda dari ketiga penyuntingan lain.
//
// Penambahan dapat ditolak orchestrator, dan penolakan itu wajib sampai ke
// pengirimnya — elemen yang sudah tergambar optimistis di layarnya tidak akan
// pernah ada di dokumen bila ia tidak diberi tahu.
func (r *Room) createElement(ctx context.Context, sub Subscriber, page string, element design.Element) error {
	reply := make(chan error, 1)
	event := elementCreateEvent{subscriber: sub, page: page, element: element, reply: reply}

	return r.await(ctx, event, reply)
}

// Ketiga sisanya tidak menunggu apa pun. Yang mungkin terjadi hanyalah
// perubahannya tidak berlaku karena sasarannya sudah lenyap, dan itu menyatu
// dengan sendirinya lewat siaran yang sedang menuju pengirimnya.

func (r *Room) updateElement(sub Subscriber, element design.Element) {
	select {
	case r.inbox <- elementUpdateEvent{subscriber: sub, element: element}:
	case <-r.done:
	}
}

func (r *Room) deleteElement(sub Subscriber, id string) {
	select {
	case r.inbox <- elementDeleteEvent{subscriber: sub, id: id}:
	case <-r.done:
	}
}

func (r *Room) reorderElement(sub Subscriber, id string, index int) {
	select {
	case r.inbox <- elementReorderEvent{subscriber: sub, id: id, index: index}:
	case <-r.done:
	}
}

// createPage dan deletePage menunggu hasil; reorderPage tidak. Keduanya yang
// menunggu punya jalur penolakan — id kembar atau batas halaman pada yang
// pertama, halaman terakhir pada yang kedua.
func (r *Room) createPage(ctx context.Context, sub Subscriber, id string, index *int) error {
	reply := make(chan error, 1)
	event := pageCreateEvent{subscriber: sub, id: id, index: index, reply: reply}

	return r.await(ctx, event, reply)
}

func (r *Room) deletePage(ctx context.Context, sub Subscriber, id string) error {
	reply := make(chan error, 1)
	event := pageDeleteEvent{subscriber: sub, id: id, reply: reply}

	return r.await(ctx, event, reply)
}

// updatePage tidak menunggu hasil, sama seperti updateElement.
func (r *Room) updatePage(sub Subscriber, id, title string, hidden, locked bool) {
	select {
	case r.inbox <- pageUpdateEvent{subscriber: sub, id: id, title: title, hidden: hidden, locked: locked}:
	case <-r.done:
	}
}

func (r *Room) reorderPage(sub Subscriber, id string, index int) {
	select {
	case r.inbox <- pageReorderEvent{subscriber: sub, id: id, index: index}:
	case <-r.done:
	}
}

// await menaruh kejadian lalu menunggu balasannya, dengan kedua jalan keluar yang
// sama di kedua penantian: room yang berhenti, dan permintaan yang dibatalkan.
//
// Ada karena ketiga penyuntingan yang membalas menuliskan rangkaian select yang
// sama persis, dan rangkaian itu punya satu jebakan yang layak ditulis sekali
// saja: jalan keluar done WAJIB ada di penantian kedua juga. Tanpanya, room yang
// berhenti tepat setelah kejadian masuk membuat pemanggilnya menggantung selamanya
// pada balasan yang tidak akan pernah dikirim.
func (r *Room) await(ctx context.Context, event roomEvent, reply <-chan error) error {
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
