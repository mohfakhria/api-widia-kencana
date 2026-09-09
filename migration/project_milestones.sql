CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- Menuntut projects sudah ada — lihat foreign key di bawah.

-- Linimasa sebuah proyek: tonggak apa saja yang sudah tercapai, dan kapan.
--
-- SATU BARIS HANYA KETIKA TERCAPAI. Tidak ada penyemaian tonggak kosong saat
-- proyek dibuat, dan tidak ada kolom "sudah/belum" — ketiadaan barisnya ITULAH
-- "belum". Yang didapat dari bentuk ini: proyek baru tidak menanggung tujuh
-- baris kosong, dan tidak ada keadaan setengah jadi ketika kosakatanya berubah.

CREATE TABLE IF NOT EXISTS project_milestones (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    project_id BIGINT NOT NULL,

    milestone VARCHAR(60) NOT NULL,
    -- TEKS BEBAS, dan itu keputusan yang diambil sadar — berbeda dari role,
    -- kind, group, dan document_type yang seluruhnya berkosakata tertutup.
    --
    -- Alasannya: alur tiap proyek tidak sama, dan kosakata tertutup memaksa
    -- setiap pekerjaan yang tidak lazim menunggu ALTER TABLE. Yang ditukar
    -- dengan keluwesan itu HARUS diketahui:
    --
    --   1. PERSENTASE KEHILANGAN PENYEBUTNYA. "4 dari 6" hanya berarti bila
    --      himpunannya tetap. Progress karenanya adalah LINIMASA, bukan angka
    --      persen; yang ingin bilah kemajuan menghitungnya terhadap daftar
    --      rekomendasi di bawah, dan tonggak karangan sendiri tidak ikut
    --      menggerakkannya.
    --
    --   2. EJAAN BERCABANG. 'BAST Ditandatangani', 'bast ditandatangani', dan
    --      'BAST_DITANDATANGANI' menjadi tiga tonggak berbeda, lalu saringan
    --      "proyek yang sudah BAST" mengembalikan sepertiganya tanpa ada yang
    --      menyadari dua pertiganya hilang.
    --
    -- Yang menahan nomor 2 adalah PENORMALAN DI USECASE, bukan CHECK di sini:
    -- huruf dikecilkan, spasi/titik/garis-bawah menjadi hubung, hubung ganda
    -- dirapatkan — pola yang sama persis dengan sanitizeAssetKey.
    --
    -- SEBERAPA JAUH ia menolong, diperiksa langsung: ketiga contoh di atas
    -- menyatu, tetapi 'B.A.S.T' menjadi 'b-a-s-t' dan TETAP terpisah dari
    -- 'bast' — titik di antara huruf tunggal adalah pemisah, bukan hiasan. Yang
    -- ditutup percabangan karena besar-kecil huruf dan tanda baca biasa; nama
    -- yang memang berbeda tetap berbeda. Karena itu daftar saran di bawah
    -- penting: ia yang membuat orang mengetik nama yang sama.
    --
    -- REKOMENDASI yang disodorkan ke frontend, urut sesuai alur pekerjaan:
    --
    --   penawaran-terkirim    penawaran sampai ke pelanggan, bukan draft
    --   po-diterima           PO pelanggan diterima
    --   pekerjaan-dimulai     produksi atau pekerjaan lapangan berjalan
    --   barang-diserahkan     serah terima fisik di lokasi
    --   bast-ditandatangani   pelanggan menerima secara resmi
    --   faktur-terbit         tagihan dikirim
    --   lunas                 pembayaran diterima penuh
    --
    -- faktur-terbit sengaja terpisah dari lunas: jarak antara keduanya yang
    -- paling sering dikejar orang, dan digabung ia tidak dapat direkonstruksi.
    --
    -- Daftar ini SARAN, bukan aturan. Ia hidup di kode supaya dapat berubah
    -- tanpa migrasi, dan disajikan lewat endpoint supaya frontend tidak
    -- menyalinnya lalu ketinggalan.

    reached_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    -- Boleh diisi mundur: tonggak sering dicatat beberapa hari setelah
    -- kejadiannya, dan memaksanya NOW() berarti linimasa yang rapi tetapi salah.

    note TEXT,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- Yang setelah dipangkas tidak menyisakan apa pun bukan tonggak.
    CONSTRAINT project_milestones_not_empty_chk
        CHECK (BTRIM(milestone) <> ''),

    CONSTRAINT fk_project_milestones_project
        FOREIGN KEY (project_id) REFERENCES projects (id)
        ON UPDATE CASCADE ON DELETE CASCADE
);

-- Satu tonggak sekali per proyek. Mencapai 'bast-ditandatangani' dua kali tidak
-- punya arti, dan yang benar-benar terjadi dua kali — revisi, pengiriman kedua —
-- adalah tonggak dengan nama berbeda.
CREATE UNIQUE INDEX IF NOT EXISTS project_milestones_uq_idx
    ON project_milestones (project_id, milestone);

-- Linimasa satu proyek, terlama lebih dulu: inilah urutan yang dibaca orang.
-- Berbeda dari lampiran dan dokumen yang terbaru di atas — di sana yang dicari
-- yang paling akhir masuk, di sini yang dicari jalan ceritanya.
CREATE INDEX IF NOT EXISTS project_milestones_project_idx
    ON project_milestones (project_id, reached_at);

-- "Proyek mana saja yang sudah BAST tetapi belum lunas" — pertanyaan yang
-- menyisir LINTAS proyek, sehingga penyaringnya milestone lebih dulu.
CREATE INDEX IF NOT EXISTS project_milestones_milestone_idx
    ON project_milestones (milestone, reached_at);

-- Cap waktu dijaga TRIGGER, bukan aplikasi — alasan lengkapnya di users.sql.
CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS project_milestones_set_updated_at_trg ON project_milestones;

CREATE TRIGGER project_milestones_set_updated_at_trg
BEFORE UPDATE ON project_milestones
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();
