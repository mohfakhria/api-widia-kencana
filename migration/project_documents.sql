CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- Menuntut projects DAN documents sudah ada — lihat foreign key di bawah.

-- Tabel ini menjawab satu pertanyaan yang sampai sekarang tidak dapat dijawab:
-- DOKUMEN APA SAJA YANG MILIK SEBUAH PROYEK.
--
-- Sengaja tabel penghubung, BUKAN kolom project_id di documents. Dokumen dibuat
-- lebih dulu dan sering tanpa proyek sama sekali — templat, penawaran yang tidak
-- jadi, dokumen percobaan — sehingga kolom di sana akan kosong pada sebagian
-- besar baris, dan setiap kueri dokumen ikut menanggung kolom yang tidak
-- dipakainya. Keterkaitannya juga lahir belakangan: orang membuat penawaran
-- dulu, proyeknya menyusul ketika PO datang.
--
-- BEDA TAJAM dengan project_attachments, dan perbedaannya harus dipegang siapa
-- pun yang menulis CRUD-nya. Lampiran adalah berkas yang DITERIMA — scan PO,
-- foto lapangan — yang tidak punya hidup di luar proyeknya, sehingga menghapus
-- lampiran ikut menghapus berkasnya. Dokumen DIBUAT sendiri di editor, punya
-- riwayat, induk, dan isinya sendiri. Melepasnya dari proyek TIDAK boleh
-- menghapus dokumennya.

CREATE TABLE IF NOT EXISTS project_documents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    project_id BIGINT NOT NULL,
    document_id BIGINT NOT NULL,

    note TEXT,
    -- Kenapa dokumen ini ada di proyek ini, bila tidak jelas dari jenisnya.

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- Menghapus proyek hanya memutus kaitannya. Dokumennya tetap hidup — ia
    -- pekerjaan yang dibuat orang, bukan lampiran yang lahir untuk proyek itu.
    CONSTRAINT fk_project_documents_project
        FOREIGN KEY (project_id) REFERENCES projects (id)
        ON UPDATE CASCADE ON DELETE CASCADE,

    -- Menghapus dokumen membawa serta kaitannya. Baris yang menunjuk dokumen
    -- yang sudah tidak ada tidak dapat dibuka siapa pun.
    CONSTRAINT fk_project_documents_document
        FOREIGN KEY (document_id) REFERENCES documents (id)
        ON UPDATE CASCADE ON DELETE CASCADE
);

-- SATU DOKUMEN MILIK SATU PROYEK. Bukan sekadar tidak boleh didaftarkan dua kali
-- pada proyek yang sama — tidak boleh ada di dua proyek sama sekali.
--
-- Itu yang membuat nilai proyek punya satu jawaban: penawaran yang sama tidak
-- dapat terhitung pada dua proyek sekaligus. Kalau kelak ternyata perlu
-- dilonggarkan, indeks ini diganti menjadi (project_id, document_id) — dan arah
-- itu memang yang mudah. Sebaliknya tidak: mengetatkan menuntut data yang sudah
-- terlanjur ganda dirapikan lebih dulu.
CREATE UNIQUE INDEX IF NOT EXISTS project_documents_document_uq_idx
    ON project_documents (document_id);

CREATE INDEX IF NOT EXISTS project_documents_project_idx
    ON project_documents (project_id, created_at DESC);

-- BELUM DIPUTUSKAN: DOKUMEN MANA YANG MENJADI NILAI PROYEK.
--
-- Tabel ini menyediakan jalannya, bukan aturannya. Menjumlahkan seluruh
-- grand_total dokumen yang tertaut SALAH dan akan menghasilkan angka dua sampai
-- tiga kali lipat: satu pekerjaan lazimnya punya penawaran, PO, dan faktur yang
-- menyebut nilai yang sama persis.
--
-- Tiga arah yang masuk akal, dan seluruhnya masih murah selama tabel ini kosong:
--
--   1. Jenis paling berwenang yang ada. Faktur mengalahkan PO, PO mengalahkan
--      penawaran. Tanpa kolom tambahan, tetapi diam-diam berubah ketika dokumen
--      baru masuk — nilai proyek bergerak tanpa ada yang menyentuhnya.
--   2. Kolom penanda, satu per proyek, ditegakkan indeks unik parsial. Eksplisit
--      dan tidak berubah sendiri, tetapi menuntut orang memilihnya.
--   3. Nilai proyek diketik sendiri di projects, dan dokumen hanya rujukan.
--      Paling sederhana, tetapi mengembalikan persoalan yang tabel ini
--      diadakan untuk menghapus.
--
-- Ketiganya satu ALTER dari sini. Jangan menjumlahkan apa pun sebelum salah
-- satunya dipilih.

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

DROP TRIGGER IF EXISTS project_documents_set_updated_at_trg ON project_documents;

CREATE TRIGGER project_documents_set_updated_at_trg
BEFORE UPDATE ON project_documents
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();
