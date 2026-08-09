package usecase

import (
	"context"
	"log/slog"
	"time"

	"github.com/mohfakhria/api-widia-kencana/internal/usecase/port/output"
)

const (
	// assetSweepInterval adalah jarak antar sapuan.
	//
	// Tidak perlu rapat: yang dibersihkan adalah unggahan yang sudah pasti tidak
	// dapat diselesaikan, dan menundanya beberapa menit tidak merugikan siapa pun.
	// Dengan tenggat presigned 15 menit dan tenggang di bawah, satu objek yatim
	// hidup paling lama sekitar 25 menit.
	assetSweepInterval = 5 * time.Minute

	// assetSweepGrace adalah jarak aman setelah tenggat, sebelum sesuatu disapu.
	//
	// Tanpa ini ada lomba yang menghasilkan pesan menyesatkan: permintaan
	// asset-upload-complete yang membaca asetnya sesaat sebelum tenggat, lalu
	// memanggil Stat sesudahnya, akan menemukan objeknya sudah dihapus penyapu
	// dan melaporkan "object not found" — padahal sebabnya kedaluwarsa.
	//
	// Aset yang sudah lewat tenggat memang tidak dapat diselesaikan, jadi
	// menundanya tidak menghidupkan kembali apa pun.
	assetSweepGrace = 5 * time.Minute

	// assetSweepBatch membatasi berapa aset yang diurus dalam satu denyut.
	//
	// Satu sapuan pertama pada database yang sudah lama menumpuk dapat menemukan
	// puluhan ribu tunggakan; mengurusnya sekaligus berarti puluhan ribu
	// permintaan ke object storage dalam satu putaran. Yang tertua diambil lebih
	// dulu, sehingga tunggakan tetap habis walau dicicil.
	assetSweepBatch = 100
)

// AssetSweeper membuang unggahan yang tidak pernah selesai.
//
// Alur unggah terdiri dari tiga langkah — minta presigned, unggah ke object
// storage, lapor selesai — dan langkah ketiga sepenuhnya bergantung pada klien.
// Tab yang ditutup di tengah jalan meninggalkan baris berstatus pending, dan
// kadang juga objek yang sudah telanjur mendarat. Tanpa penyapu, keduanya
// menetap selamanya, dan satu-satunya yang bertambah seiring waktu adalah
// tagihan penyimpanan.
type AssetSweeper struct {
	repo    output.AssetRepository
	storage output.ObjectStorage
	logger  *slog.Logger
}

func NewAssetSweeper(repo output.AssetRepository, storage output.ObjectStorage, logger *slog.Logger) *AssetSweeper {
	if logger == nil {
		logger = slog.Default()
	}

	return &AssetSweeper{repo: repo, storage: storage, logger: logger}
}

func (s *AssetSweeper) Name() string {
	return "asset-sweeper"
}

func (s *AssetSweeper) Run(ctx context.Context) error {
	ticker := time.NewTicker(assetSweepInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case now := <-ticker.C:
			s.sweep(ctx, now)
		}
	}
}

// sweep mengurus satu batch aset kedaluwarsa.
//
// Kegagalan satu aset tidak menghentikan yang lain: tiap aset berdiri sendiri,
// dan satu objek yang tidak dapat dihapus tidak boleh menahan sisanya.
func (s *AssetSweeper) sweep(ctx context.Context, now time.Time) {
	expired, err := s.repo.FindExpired(ctx, now.Add(-assetSweepGrace), assetSweepBatch)
	if err != nil {
		if ctx.Err() != nil {
			// Aplikasi sedang berhenti. Tunggakannya masih ada saat menyala lagi.
			return
		}
		s.logger.Error("find expired assets", "error", err)
		return
	}
	if len(expired) == 0 {
		return
	}

	disapu := 0
	for i := range expired {
		if ctx.Err() != nil {
			break
		}
		if s.discard(ctx, expired[i].Token, expired[i].ObjectName) {
			disapu++
		}
	}

	s.logger.Info("swept expired asset uploads",
		"found", len(expired), "swept", disapu)
}

// discard membuang objeknya lebih dulu, baru menandai barisnya.
//
// URUTAN INI YANG MENENTUKAN BENAR-TIDAKNYA. Baris yang ditandai gagal tidak
// lagi cocok dengan pencarian di atas, sehingga ia tidak akan pernah disapu
// lagi. Menandai lebih dulu lalu gagal menghapus objeknya berarti objek itu
// yatim selamanya — tanpa satu pun baris yang menunjuk ke sana, jadi tidak ada
// yang bisa menemukannya kembali.
//
// Terbalik begini, kegagalan menandai hanya berarti aset yang sama diurus lagi
// pada denyut berikutnya. Penghapusan objek bersifat idempoten pada object
// storage — objek yang memang tidak pernah terunggah, yaitu kasus yang paling
// umum, dilaporkan berhasil.
func (s *AssetSweeper) discard(ctx context.Context, token, objectName string) bool {
	if err := s.storage.Delete(ctx, objectName); err != nil {
		if ctx.Err() == nil {
			s.logger.Warn("delete expired asset object",
				"asset", token, "object", objectName, "error", err)
		}
		return false
	}

	// Kode kegagalannya sama dengan yang dipakai CompleteUpload ketika menolak
	// unggahan yang lewat tenggat, supaya keduanya terbaca sebagai satu sebab.
	if err := s.repo.MarkFailed(ctx, token, "upload_expired", "asset upload was never completed"); err != nil {
		if ctx.Err() == nil {
			s.logger.Warn("mark expired asset failed", "asset", token, "error", err)
		}
		return false
	}

	return true
}
