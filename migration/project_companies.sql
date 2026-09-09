CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- Menuntut projects DAN companies sudah ada — lihat foreign key di bawah.
-- Jalankan projects.sql dan companies.sql lebih dulu.

CREATE TABLE IF NOT EXISTS project_companies (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    project_id BIGINT NOT NULL,
    company_id UUID NOT NULL,
    -- Tipe keduanya sengaja berbeda karena tabel asalnya memang berbeda:
    -- projects memakai id BIGINT, companies memakai id UUID. Bukan kelalaian.

    role VARCHAR(30) NOT NULL,
    -- Kosakata TERTUTUP — lihat CHECK di bawah.
    --
    -- Ketiga namanya diambil dari membaca dokumen, BUKAN dari istilah yang
    -- dipakai sehari-hari di kantor: penawaran ditujukan kepada satu pihak
    -- (customer), pekerjaannya berlangsung di lokasi milik pihak lain
    -- (end-user, "Factory D"), dan barangnya datang dari pihak ketiga
    -- (supplier). Ganti bila istilahnya berbeda — tabel ini masih kosong,
    -- sehingga menggantinya cuma satu ALTER pada CHECK-nya. Sesudah terisi,
    -- menggantinya berarti memperbarui setiap barisnya.

    note TEXT,
    -- Kenapa perusahaan ini terlibat, bila tidak jelas dari perannya sendiri.

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT project_companies_role_chk
        CHECK (role IN ('customer', 'end-user', 'supplier')),

    CONSTRAINT fk_project_companies_project
        FOREIGN KEY (project_id) REFERENCES projects (id)
        ON UPDATE CASCADE ON DELETE CASCADE,

    -- RESTRICT, bukan CASCADE, dan perbedaannya disengaja. Menghapus proyek
    -- memang membuang keterlibatannya; menghapus PERUSAHAAN yang masih terlibat
    -- di sebuah proyek adalah kekeliruan yang harus ditolak, bukan diteruskan
    -- diam-diam sampai proyeknya kehilangan pelanggannya.
    --
    -- Di situlah status 'inactive' pada companies berperan: pelanggan lama
    -- dipensiunkan, bukan dihapus.
    CONSTRAINT fk_project_companies_company
        FOREIGN KEY (company_id) REFERENCES companies (id)
        ON UPDATE CASCADE ON DELETE RESTRICT
);

-- Satu perusahaan boleh memegang DUA peran dalam satu proyek — pelanggan yang
-- sekaligus memasok sebagian barangnya bukan hal aneh — tetapi tidak boleh
-- didaftarkan dua kali pada peran yang sama.
CREATE UNIQUE INDEX IF NOT EXISTS project_companies_uq_idx
    ON project_companies (project_id, company_id, role);

-- BANYAK perusahaan per peran diizinkan; dua pemasok dalam satu proyek adalah
-- hal biasa. Bila kelak diputuskan satu saja per peran, indeks di atas diganti
-- menjadi (project_id, role) — tetapi arah itu lebih sulit: ia menuntut data
-- yang sudah ada dirapikan lebih dulu, sedangkan melonggarkannya tidak.
CREATE INDEX IF NOT EXISTS project_companies_project_idx
    ON project_companies (project_id);

CREATE INDEX IF NOT EXISTS project_companies_company_idx
    ON project_companies (company_id);

-- Cap waktu dijaga TRIGGER, bukan aplikasi — alasan lengkapnya di users.sql.
-- Fungsinya diulang dengan CREATE OR REPLACE seperti berkas migration lain:
-- berkas yang hanya berhasil bila berkas lain sudah dijalankan adalah jebakan.
CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS project_companies_set_updated_at_trg ON project_companies;

CREATE TRIGGER project_companies_set_updated_at_trg
BEFORE UPDATE ON project_companies
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();
