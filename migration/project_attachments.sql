CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- Menuntut projects, assets, DAN companies sudah ada — lihat foreign key di
-- bawah. Jalankan ketiganya lebih dulu.

CREATE TABLE IF NOT EXISTS project_attachments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    project_id BIGINT NOT NULL,

    asset_id BIGINT NOT NULL,
    -- Berkasnya sendiri tinggal di object storage lewat alur aset yang sudah
    -- ada: asset-upload-request dengan group 'documents/<jenis>', PUT ke
    -- presigned URL, lalu asset-upload-complete. Tabel ini hanya menautkannya.
    --
    -- Kelompok documents/* memang diadakan untuk ini dan sampai sekarang tidak
    -- punya satu pun produsen.

    kind VARCHAR(30) NOT NULL,
    -- Kosakata TERTUTUP — lihat CHECK di bawah.
    --
    -- Enam yang pertama sama persis dengan document_type, karena berkas yang
    -- diterima adalah jenis dokumen yang sama dengan yang dibuat sendiri: PO
    -- yang datang dari pelanggan tetap sebuah purchase order. Tiga sisanya khas
    -- unggahan dan tidak pernah dibuat di editor.
    --
    -- Tertutup dengan alasan yang sama seperti group pada aset: teks bebas
    -- melahirkan "PO", "P.O.", dan "purchase order" sebagai tiga jenis berbeda,
    -- lalu penyaring "tampilkan semua PO" mengembalikan sepertiganya tanpa ada
    -- yang menyadari dua pertiganya hilang.

    company_id UUID,
    -- "Punya siapa" — PO milik pelanggan, faktur pajak milik kita, sertifikat
    -- barang milik pemasok. NULL untuk yang tidak milik siapa-siapa, seperti
    -- foto lapangan.
    --
    -- TIDAK dijamin database bahwa ia peserta proyek ini. Menjaminnya menuntut
    -- foreign key gabungan ke project_companies, dan itu menuntut (project_id,
    -- company_id) unik di sana — yang berarti satu perusahaan tidak lagi boleh
    -- memegang dua peran. Aturannya karenanya ditegakkan usecase, dan itu harus
    -- ditulis saat CRUD-nya dibuat; database di sini hanya memastikan
    -- perusahaannya ada.

    note TEXT,
    -- Yang menjawab "kenapa ada dua PO di sini" enam bulan kemudian.

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- TIDAK ADA uploaded_by. Aset sudah menyimpannya, dan menyalinnya ke sini
    -- berarti dua tempat yang suatu hari berselisih.

    CONSTRAINT project_attachments_kind_chk
        CHECK (kind IN (
            'quotation', 'purchase-order', 'bast',
            'delivery-note', 'service-report', 'invoice',
            'contract', 'site-photo', 'tax-invoice'
        )),

    CONSTRAINT fk_project_attachments_project
        FOREIGN KEY (project_id) REFERENCES projects (id)
        ON UPDATE CASCADE ON DELETE CASCADE,

    -- CASCADE: aset yang dihapus lewat asset-delete membawa serta tautannya.
    -- Lampiran yang menunjuk berkas yang sudah tidak ada tidak dapat dibuka
    -- siapa pun, dan menyimpannya hanya menghasilkan baris yang selalu gagal.
    CONSTRAINT fk_project_attachments_asset
        FOREIGN KEY (asset_id) REFERENCES assets (id)
        ON UPDATE CASCADE ON DELETE CASCADE,

    CONSTRAINT fk_project_attachments_company
        FOREIGN KEY (company_id) REFERENCES companies (id)
        ON UPDATE CASCADE ON DELETE RESTRICT
);

-- SATU BERKAS MILIK SATU PROYEK, dan itu yang membuat penghapusannya sederhana:
-- menghapus lampiran boleh langsung menghapus berkasnya, tanpa menghitung siapa
-- lagi yang memakainya. Scan PO memang milik satu proyek.
CREATE UNIQUE INDEX IF NOT EXISTS project_attachments_asset_uq_idx
    ON project_attachments (asset_id);

CREATE INDEX IF NOT EXISTS project_attachments_project_idx
    ON project_attachments (project_id, created_at DESC);

CREATE INDEX IF NOT EXISTS project_attachments_kind_idx
    ON project_attachments (kind);

-- PERINGATAN untuk yang menulis CRUD-nya nanti.
--
-- ON DELETE CASCADE di atas membuang BARIS lampiran ketika proyeknya dihapus —
-- tetapi database tidak dapat menghapus objek di MinIO. Menghapus proyek tanpa
-- membuang asetnya lebih dulu meninggalkan berkas yatim: objeknya ada,
-- barisnya di assets ada, tetapi tidak ada satu pun yang menunjuk ke sana.
--
-- Urutannya harus di usecase, dan harus objek dulu baru baris — sama seperti
-- AssetSweeper: yang terbalik meninggalkan objek tanpa satu pun baris yang
-- dapat menemukannya kembali.

-- Cap waktu dijaga TRIGGER, bukan aplikasi — alasan lengkapnya di users.sql.
CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS project_attachments_set_updated_at_trg ON project_attachments;

CREATE TRIGGER project_attachments_set_updated_at_trg
BEFORE UPDATE ON project_attachments
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();
