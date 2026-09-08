CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- Menuntut companies sudah ada — lihat foreign key di bawah. Jalankan
-- companies.sql lebih dulu.

CREATE TABLE IF NOT EXISTS company_contacts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    company_id UUID NOT NULL,

    salutation VARCHAR(30),
    -- Bapak, Ibu, Mr., Ms., Dr.

    name VARCHAR(150) NOT NULL,
    -- Nama lengkap TANPA sapaan, contoh: Fahmi Ardiyanto.

    position VARCHAR(100),
    department VARCHAR(100),
    -- Ketiganya yang menyusun baris "Attn:" di dokumen —
    -- "Bapak Wahyudi — Owner" adalah salutation + name + position.

    email VARCHAR(150),
    phone VARCHAR(30),
    mobile VARCHAR(30),
    -- Nomor WhatsApp sengaja tidak punya kolom sendiri: di Indonesia ia hampir
    -- selalu nomor yang sama dengan mobile, dan kolom kedua yang isinya sama
    -- hanya menambah tempat untuk berselisih.

    is_primary BOOLEAN NOT NULL DEFAULT FALSE,

    status VARCHAR(20) NOT NULL DEFAULT 'active',

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT company_contacts_name_not_empty_chk
        CHECK (BTRIM(name) <> ''),

    CONSTRAINT company_contacts_status_chk
        CHECK (status IN ('active', 'inactive')),

    CONSTRAINT fk_company_contacts_company
        FOREIGN KEY (company_id)
        REFERENCES companies (id)
        ON UPDATE CASCADE
        ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS company_contacts_company_idx
    ON company_contacts (company_id);

CREATE UNIQUE INDEX IF NOT EXISTS company_contacts_primary_uq_idx
    ON company_contacts (company_id)
    WHERE is_primary AND status = 'active';
-- Satu kontak utama per perusahaan, ditegakkan database.
--
-- Tanpa ini, "jadikan kontak utama" yang lupa menurunkan yang lama menghasilkan
-- dua baris is_primary — dan yang muncul di dokumen menjadi bergantung pada
-- urutan baris, yang di Postgres tidak dijanjikan apa pun. Aplikasi tetap harus
-- menurunkan yang lama lebih dulu; indeks ini yang memastikan kelalaian itu
-- ditolak, bukan didiamkan.
--
-- Hanya yang aktif, sehingga kontak lama yang dinonaktifkan boleh tetap
-- membawa penanda utamanya sebagai jejak.

-- Cap waktu dijaga TRIGGER, bukan aplikasi — alasan lengkapnya di users.sql.
--
-- Fungsinya diulang di sini dengan CREATE OR REPLACE seperti berkas migration
-- lain: yang mengulang tidak berakibat apa pun, sedangkan yang mengandalkan
-- berkas lain sudah dijalankan adalah jebakan.
CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS company_contacts_set_updated_at_trg ON company_contacts;

CREATE TRIGGER company_contacts_set_updated_at_trg
BEFORE UPDATE ON company_contacts
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();


-- Benih kontak PT. Widia Kencana. Dijalankan berulang tanpa akibat.
--
-- Namanya diambil dari baris users yang sudah ada di repo, beserta surel
-- perusahaannya. JABATAN SENGAJA KOSONG: blok tanda tangan di template masih
-- berbunyi "Nama Penanggung Jawab / Direktur" sebagai penampung, dan menebak
-- siapa yang Direktur berarti menaruh jabatan karangan pada dokumen yang dikirim
-- ke pelanggan. Isi lewat aplikasi setelah tabel ini punya CRUD.
--
-- is_primary jatuh ke Fahmi semata-mata karena ia baris pertama di users.sql.
-- Itu pilihan sembarang yang terlihat, bukan kesimpulan — membaliknya satu
-- UPDATE.
--
-- WHERE NOT EXISTS, bukan ON CONFLICT: tabel ini tidak punya kunci alami untuk
-- dijadikan sasaran konflik, dan menambah indeks unik hanya demi kenyamanan
-- benih adalah alasan yang terbalik.
INSERT INTO company_contacts (company_id, salutation, name, email, is_primary)
SELECT c.id, 'Bapak', 'Fahmi Ardiyanto', 'fahmi@widiakencana.com', TRUE
FROM companies c
WHERE c.code = 'WIKEN'
  AND NOT EXISTS (
      SELECT 1 FROM company_contacts x
      WHERE x.company_id = c.id AND LOWER(x.name) = LOWER('Fahmi Ardiyanto')
  );

INSERT INTO company_contacts (company_id, salutation, name, email, is_primary)
SELECT c.id, 'Bapak', 'Fakhri', 'fakhri@widiakencana.com', FALSE
FROM companies c
WHERE c.code = 'WIKEN'
  AND NOT EXISTS (
      SELECT 1 FROM company_contacts x
      WHERE x.company_id = c.id AND LOWER(x.name) = LOWER('Fakhri')
  );


-- Kontak PT. Dizkaa Putra Anugerah, disalin dari baris "Attn:" pada penawaran
-- yang sudah dikirim: "Bapak Wahyudi — Owner". Ketiga kolomnya memetakan tepat —
-- salutation, name, position — dan itu memang bentuk yang dirakit kembali saat
-- dicetak.
--
-- Surel dan nomornya tidak ada di dokumen maupun di profil publiknya, jadi
-- dibiarkan kosong.
INSERT INTO company_contacts (company_id, salutation, name, position, is_primary)
SELECT c.id, 'Bapak', 'Wahyudi', 'Owner', TRUE
FROM companies c
WHERE c.code = 'DIZKAA'
  AND NOT EXISTS (
      SELECT 1 FROM company_contacts x
      WHERE x.company_id = c.id AND LOWER(x.name) = LOWER('Wahyudi')
  );
