package documentdesign

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
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
	// antrean tanpa batas.
	//
	// Kelonggarannya besar karena rantai kegagalannya panjang dan ujungnya mahal:
	// pengirim yang tertahan di sini menahan dispatch, lalu antrean masuk klien
	// terisi, lalu readLoop keluar — dan koneksi orang itu diakhiri. Lonjakan
	// sesaat, terutama dari kursor yang datang jauh lebih deras daripada kejadian
	// lain, sebaiknya tidak pernah sampai ke sana.
	//
	// Ongkosnya dua lapis, dan hanya yang pertama selalu dibayar:
	//
	//	256 KB  array slot, dialokasikan di muka begitu room lahir
	//	768 KB  isinya, hanya bila antrean sungguh terisi penuh
	//
	// Channel mengalokasikan seluruh slotnya di muka, dan satu roomEvent adalah
	// interface selebar dua kata. Angkanya PER ROOM, jadi puncaknya sebanding
	// dengan jumlah dokumen yang dibuka serentak.
	roomInboxSize = 16384

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

	// cursorTickInterval adalah jarak antar siaran kursor — dua puluh kali per
	// detik.
	//
	// Gerakan tidak lagi disiarkan saat diterima melainkan dipadatkan: pesan masuk
	// cuma menimpa satu entri peta, lalu denyut ini menyiarkan seluruh peta sekali.
	// Laju keluar karenanya terbatas pada denyut, berapa pun derasnya masukan.
	//
	// Lima puluh milidetik berarti tunda rata-rata 25 ms — di bawah ambang "terasa
	// seketika", dan tenggelam di dalam interpolasi yang dilakukan frontend.
	// Menaikkannya ke 30 Hz cukup mengganti tetapan ini saja.
	cursorTickInterval = 50 * time.Millisecond
)

// Subscriber adalah satu koneksi yang menerima siaran dari room.
//
// Send wajib tidak memblokir. Orchestrator memanggilnya saat menyiarkan, dan
// implementasi yang menunggu I/O akan menghentikan seluruh penyuntingan dokumen
// itu hanya karena satu klien lambat. Disconnect tunduk pada aturan yang sama.
type Subscriber interface {
	// Send untuk pesan yang wajib sampai. Antrean yang penuh berarti klien
	// tertinggal terlalu jauh, dan koneksinya diakhiri.
	Send(payload []byte)

	// SendEphemeral untuk pesan yang hanya berarti nilai terkininya.
	//
	// stream menandai pesan mana yang saling menggantikan: payload baru menimpa
	// pesan sejenis yang masih mengantre, alih-alih menumpuk di belakangnya.
	// Klien yang tersendat karenanya menerima posisi terkini saat ia menyusul,
	// bukan tumpukan posisi basi.
	//
	// Pemisahan dari Send bukan kerapian melainkan keharusan. Kursor yang memakai
	// Send akan MENJATUHKAN sesi penyuntingan begitu klien tersendat sesaat —
	// jeda GC atau satu render berat sudah cukup menumpuk lebih dari kapasitas
	// antrean.
	SendEphemeral(stream string, payload []byte)

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
	EncodeSnapshot(content json.RawMessage, version int64, page PageSize) ([]byte, error)
	EncodePresence(users []PresenceUser) ([]byte, error)
	EncodeCursors(cursors []Cursor) ([]byte, error)
}

// cursorStream menandai seluruh siaran kursor sebagai satu aliran
// nilai-terakhir. Siaran baru menggantikan siaran kursor yang masih mengantre di
// klien, karena letak kursor lima puluh milidetik lalu sudah tidak berarti apa
// pun begitu ada yang lebih baru.
const cursorStream = "cursor"

// Cursor adalah letak kursor satu orang di atas dokumen.
//
// Dikunci per orang, bukan per koneksi — sama seperti kehadiran. Satu orang
// dengan beberapa tab punya satu kursor, yaitu posisi dari tab yang terakhir
// bergerak; dua kursor berlabel nama yang sama justru membingungkan pembacanya.
type Cursor struct {
	UserID string
	Page   string
	X      float64
	Y      float64
}

// Member adalah satu koneksi beserta pemiliknya.
//
// Identitas dibawa dari tiket, bukan dicari saat koneksi terjadi. Membaca tabel
// user di dalam orchestrator berarti satu query lambat membekukan penyuntingan
// seluruh dokumen itu; tiket sudah diterbitkan lewat endpoint terautentikasi di
// luar jalur realtime, dan umurnya cuma tiga puluh detik sehingga namanya tidak
// mungkin basi.
type Member struct {
	Subscriber Subscriber
	UserID     string
	UserName   string
}

// PresenceUser adalah satu orang yang sedang membuka dokumen.
//
// Yang dihitung orang, bukan koneksi: satu orang dengan tiga tab tetap satu
// entri, dan ia baru hilang ketika tab terakhirnya tertutup. Menghitung koneksi
// akan menampilkan satu orang sebagai beberapa penyunting — dan itu bukan
// kemungkinan teoretis, karena frontend membuka lebih dari satu koneksi tiap kali
// halaman dimuat.
type PresenceUser struct {
	ID   string
	Name string
}

// PageSize adalah ukuran halaman dalam titik, ikut dikirim bersama snapshot.
//
// Tanpa ini frontend tidak punya cara menentukan ukuran kanvas: isi dokumen
// sengaja tidak memuat ukuran halaman, dan satu-satunya sumber lain — endpoint
// detail dokumen — mengembalikan ukuran dalam satuan asli kertas. Menyerahkan
// konversinya ke frontend berarti membuka satu kelas kesalahan baru untuk
// pertanyaan yang jawabannya sudah dipegang backend.
type PageSize struct {
	Width  float64
	Height float64
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
	member Member
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

// cursorMoveEvent tidak punya reply. Pengirimnya tidak menunggu apa pun, dan
// tidak ada yang berguna untuk dikembalikan — kursor adalah keadaan sesaat.
type cursorMoveEvent struct {
	cursor Cursor
}

func (syncEvent) isRoomEvent()       {}
func (leaveEvent) isRoomEvent()      {}
func (snapshotEvent) isRoomEvent()   {}
func (cursorMoveEvent) isRoomEvent() {}

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

// presentUsers menyusun daftar orang yang sedang membuka dokumen, tanpa
// pengulangan.
//
// Urutannya dibuat pasti — menurut nama, lalu id sebagai pemutus seri — karena
// iterasi peta di Go berurutan acak. Tanpa pengurutan, tumpukan avatar di
// frontend akan berganti susunan setiap kali ada yang datang atau pergi.
func (r *Room) presentUsers() []PresenceUser {
	seen := make(map[string]struct{}, len(r.members))
	users := make([]PresenceUser, 0, len(r.members))

	for _, member := range r.members {
		if _, exists := seen[member.UserID]; exists {
			continue
		}
		seen[member.UserID] = struct{}{}
		users = append(users, PresenceUser{ID: member.UserID, Name: member.UserName})
	}

	slices.SortFunc(users, func(a, b PresenceUser) int {
		if order := strings.Compare(a.Name, b.Name); order != 0 {
			return order
		}

		return strings.Compare(a.ID, b.ID)
	})

	return users
}

// broadcastPresence memberi tahu seluruh penghuni siapa saja yang sedang membuka
// dokumen ini.
//
// Kegagalan menyusun payload hanya dicatat, tidak menandai room rusak. Daftar
// kehadiran adalah hiasan di sekitar pekerjaan yang sebenarnya; menghentikan
// penyuntingan karena ia gagal disusun jauh lebih merugikan daripada tumpukan
// avatar yang tidak diperbarui.
func (r *Room) broadcastPresence() {
	payload, err := r.encodePresence()
	if err != nil {
		return
	}

	for subscriber := range r.members {
		subscriber.Send(payload)
	}
}

func (r *Room) sendPresence(subscriber Subscriber) {
	payload, err := r.encodePresence()
	if err != nil {
		return
	}

	subscriber.Send(payload)
}

// broadcastCursors mengirim letak seluruh kursor ke semua penghuni.
//
// Kegagalan menyusun payload hanya dicatat, tidak menandai room rusak. Kursor
// adalah hiasan di sekitar pekerjaan yang sebenarnya; menghentikan penyuntingan
// karena ia gagal disusun jauh lebih merugikan daripada kursor yang tidak
// bergerak.
func (r *Room) broadcastCursors() {
	// Penanda kotor sengaja TIDAK dibersihkan saat penghuninya kurang dari dua.
	// Dengan begitu orang kedua yang bergabung langsung menerima kursor yang sudah
	// ada pada denyut berikutnya, bukan menunggu ada yang menggerakkannya lagi.
	if !r.cursorsDirty || len(r.members) < 2 {
		return
	}
	r.cursorsDirty = false

	payload, err := r.encodeCursors()
	if err != nil {
		return
	}

	for subscriber := range r.members {
		subscriber.SendEphemeral(cursorStream, payload)
	}
}

// sendCursors mengirim kursor yang sudah ada kepada satu orang saja, dipakai saat
// ia baru bergabung.
//
// Dilewati bila belum ada kursor sama sekali: daftar kosong tidak memberi tahu
// apa pun kepada pendatang yang memang belum punya kursor untuk digambar.
func (r *Room) sendCursors(subscriber Subscriber) {
	if len(r.cursors) == 0 {
		return
	}

	payload, err := r.encodeCursors()
	if err != nil {
		return
	}

	subscriber.SendEphemeral(cursorStream, payload)
}

func (r *Room) encodeCursors() ([]byte, error) {
	payload, err := r.encoder.EncodeCursors(r.presentCursors())
	if err != nil {
		r.logger.Error("encode document design cursors", "document", r.token, "error", err)
		return nil, err
	}

	return payload, nil
}

// presentCursors menyusun muatan siaran.
//
// Diurutkan menurut id semata-mata supaya isinya dapat diulang: iterasi peta di
// Go berurutan acak, dan tanpa pengurutan dua siaran untuk keadaan yang sama akan
// menghasilkan byte yang berbeda-beda.
func (r *Room) presentCursors() []Cursor {
	cursors := make([]Cursor, 0, len(r.cursors))
	for _, cursor := range r.cursors {
		cursors = append(cursors, cursor)
	}

	slices.SortFunc(cursors, func(a, b Cursor) int {
		return strings.Compare(a.UserID, b.UserID)
	})

	return cursors
}

func (r *Room) encodePresence() ([]byte, error) {
	payload, err := r.encoder.EncodePresence(r.presentUsers())
	if err != nil {
		r.logger.Error("encode document design presence", "document", r.token, "error", err)
		return nil, err
	}

	return payload, nil
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
