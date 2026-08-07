package documentdesign

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/mohfakhria/api-widia-kencana/internal/domain"
)

const (
	// flushInterval adalah jarak antar penyimpanan ke database. Kerugian maksimal
	// saat proses mati mendadak sebesar ini, selalu.
	flushInterval = 2 * time.Second

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
