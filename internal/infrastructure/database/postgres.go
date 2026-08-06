package database

import (
	"context"
	"database/sql"
	"time"

	"github.com/mohfakhria/api-widia-kencana/internal/infrastructure/config"

	_ "github.com/lib/pq"
)

const (
	// maxOpenConns membatasi jumlah koneksi ke Postgres.
	//
	// Tanpa batas, database/sql membuka koneksi sebanyak yang diminta. Itu tidak
	// terasa selama akses database hanya dipicu request, tetapi document design
	// menulis di latar belakang tiap dua detik untuk setiap dokumen yang sedang
	// dibuka — beban yang tumbuh mengikuti jumlah dokumen aktif, bukan jumlah
	// request. Melewati max_connections Postgres tidak hanya menggagalkan
	// penyimpanan dokumen, tetapi seluruh endpoint karena pool-nya sama.
	//
	// Dengan batas atas, kelebihan beban berubah jadi antrean sesaat alih-alih
	// kegagalan menyeluruh.
	maxOpenConns = 25

	// maxIdleConns dijaga cukup untuk melayani lonjakan pendek tanpa membuka
	// koneksi baru terus-menerus.
	maxIdleConns = 5

	// connMaxLifetime memaksa koneksi diperbarui berkala, supaya proxy atau
	// firewall yang memutus koneksi lama tidak meninggalkan koneksi mati di pool.
	connMaxLifetime = 30 * time.Minute
)

func NewPostgres(ctx context.Context, cfg config.Config) (*sql.DB, error) {
	db, err := sql.Open("postgres", cfg.PostgresDSN())
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(maxOpenConns)
	db.SetMaxIdleConns(maxIdleConns)
	db.SetConnMaxLifetime(connMaxLifetime)

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}

	return db, nil
}
