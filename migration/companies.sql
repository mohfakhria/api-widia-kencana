CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS companies (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    -- UUID sebagai primary key, BUKAN pola id BIGINT + token UUID yang dipakai
    -- documents, assets, dan document_papers. Akibatnya kolom ini yang muncul di
    -- URL dan di balasan API — jadi frontend menyebut `id` untuk perusahaan
    -- sementara menyebut `token` untuk dokumen dan aset. Keduanya bekerja; yang
    -- perlu diketahui hanya bahwa perbedaan itu disengaja, bukan terlewat.

    code VARCHAR(30) NOT NULL,
    -- Kode internal, contoh: WIKEN. Unik — lihat indeksnya di bawah.
    --
    -- WAJIB, dan itu keputusan yang diambil sadar dengan satu akibat: setiap
    -- perusahaan baru menuntut seseorang mengarang kode unik tepat pada saat ia
    -- sedang mengetik penawaran. Frontend sebaiknya mengusulkannya dari nama,
    -- karena kode yang dikarang terburu-buru melahirkan DIZKAA, DIZKAA1, dan
    -- PTDIZKAA untuk perusahaan yang sama.

    name VARCHAR(200) NOT NULL,
    -- Nama pendek tanpa bentuk badan usaha, contoh: Widia Kencana.
    -- Dipakai daftar dan pencarian, BUKAN untuk dicetak.

    legal_name VARCHAR(255) NOT NULL,
    -- Nama legal lengkap, contoh: PT Widia Kencana.
    --
    -- INI yang dicetak di dokumen, apa adanya. Tidak dirakit dari company_type +
    -- name: perakitan harus memutuskan "PT" atau "PT.", spasi, dan perlakuan
    -- untuk CV maupun Yayasan — dan kekeliruannya muncul di kop surat pelanggan.

    company_type VARCHAR(30),
    -- Bentuk badan usaha: PT, CV, UD, Yayasan. Keterangan saja; TIDAK dipakai
    -- merakit legal_name.

    description TEXT,

    established_date DATE,

    address TEXT,
    -- Alamat lengkap, boleh multi-baris, DICETAK APA ADANYA.
    --
    -- Sengaja tidak dipecah menjadi jalan/kota/provinsi/kodepos: elemen dokumen
    -- menyimpannya sebagai satu teks dengan pergantian baris di tengah, dan
    -- memecahnya berarti setiap template merakitnya kembali sedikit berbeda.

    email VARCHAR(150),
    phone VARCHAR(30),
    fax VARCHAR(30),
    -- Ketiganya milik PERUSAHAAN, bukan orangnya. Yang melekat pada orang ada di
    -- company_contacts. Empat kolom serupa tanpa pemisahan yang terlihat adalah
    -- undangan untuk diisi tertukar.

    timezone VARCHAR(50) NOT NULL DEFAULT 'Asia/Jakarta',
    locale VARCHAR(10) NOT NULL DEFAULT 'id-ID',
    currency_code CHAR(3) NOT NULL DEFAULT 'IDR',
    -- Ketiganya hanya berarti bagi perusahaan PENERBIT dokumen — yang menentukan
    -- format tanggal dan mata uang di kertas adalah penerbitnya, bukan penerima.
    -- Pada baris pelanggan dan pemasok, ketiganya akan berisi bawaan yang tidak
    -- dipakai apa pun.

    status VARCHAR(20) NOT NULL DEFAULT 'active',
    -- Values: active, inactive
    --
    -- BUKAN soft delete. Menghapus perusahaan menghapus barisnya, sejalan dengan
    -- seluruh tabel lain di repo ini. 'inactive' berarti "ada dan pernah dipakai,
    -- hanya tidak ditawarkan lagi di pemilih" — keadaan bisnis, bukan penanda
    -- kematian.

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT companies_code_not_empty_chk
        CHECK (BTRIM(code) <> ''),

    CONSTRAINT companies_legal_name_not_empty_chk
        CHECK (BTRIM(legal_name) <> ''),

    CONSTRAINT companies_status_chk
        CHECK (status IN ('active', 'inactive'))
);

CREATE UNIQUE INDEX IF NOT EXISTS companies_code_uq_idx
    ON companies (UPPER(code));
-- UPPER, supaya 'wiken' dan 'WIKEN' tidak dapat hidup berdampingan. Kode
-- dimaksudkan dibaca manusia, dan manusia tidak membedakan keduanya.

CREATE INDEX IF NOT EXISTS companies_status_idx
    ON companies (status);

CREATE INDEX IF NOT EXISTS companies_name_idx
    ON companies (LOWER(name));

-- Cap waktu dijaga TRIGGER, bukan aplikasi — alasan lengkapnya di users.sql.
--
-- Fungsinya diulang di sini dengan CREATE OR REPLACE, sama seperti users.sql dan
-- assets.sql: berkas migration dijalankan manual satu per satu, dan berkas yang
-- hanya berhasil bila berkas lain sudah dijalankan adalah jebakan.
CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS companies_set_updated_at_trg ON companies;

CREATE TRIGGER companies_set_updated_at_trg
BEFORE UPDATE ON companies
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();


-- Benih perusahaan sendiri. Dijalankan berulang tanpa akibat.
--
-- Seluruh isinya diambil dari yang sudah tercetak di dokumen — lapisan master
-- pada template quotation memuat nama, alamat, dan surelnya. Tidak ada satu pun
-- yang dikarang: nomor telepon, faks, dan tanggal berdiri DIBIARKAN KOSONG
-- karena tidak ada sumbernya, dan kolom yang diisi tebakan lebih merugikan
-- daripada kolom yang kosong — yang kosong terlihat, yang salah tidak.
--
-- legal_name memakai titik setelah PT, persis seperti yang tercetak di kop.
-- Kolom inilah yang disalin ke dokumen, jadi bentuknya harus sama sampai ke
-- tanda bacanya.
--
-- ON CONFLICT DO NOTHING, bukan DO UPDATE. Menjalankan ulang berkas ini tidak
-- boleh mengembalikan data yang sudah disunting orangnya lewat aplikasi — benih
-- adalah keadaan AWAL, bukan keadaan yang dipaksakan terus-menerus.
--
-- Sasarannya UPPER(code), mengikuti indeks uniknya. Menyebut (code) saja tidak
-- cocok dengan indeks ekspresi itu dan Postgres akan menolaknya.
--
-- Baris kedua adalah PELANGGAN, bukan perusahaan sendiri, dan keduanya duduk di
-- tabel yang sama. Akibatnya segera terasa begitu frontend membuat pemilih
-- "KEPADA YTH.": PT. Widia Kencana akan muncul di daftar pelanggannya sendiri,
-- dan tidak ada satu pun kolom yang dapat menyaringnya keluar. Itulah yang
-- disediakan is_customer/is_supplier, yang belum ada di tabel ini.
--
-- Sumber baris UTAC: halaman kantor resminya di utacgroup.com. Nama dan alamat
-- disalin dari sana; TELEPON, FAKS, DAN SUREL TIDAK DICANTUMKAN di halaman itu,
-- jadi ketiganya dibiarkan kosong. Deskripsinya pun kosong: halaman itu tidak
-- menjelaskan pabrik ini secara khusus, dan menyalin keterangan induk grupnya
-- berarti menaruh keterangan yang belum tentu benar tentang lokasi ini.
--
-- Alamatnya dipatah menjadi dua baris mengikuti cara blok "KEPADA YTH." dicetak,
-- dan ", Indonesia" ditanggalkan — dokumen ini dalam negeri, dan kop pelanggan
-- lain pun tidak mencantumkan negara. Kode 'UTAC' saya yang menetapkan; ganti
-- bila ada kode internal yang sudah dipakai.
--
-- Perhatikan legal_name keduanya berbeda dalam hal titik: 'PT. Widia Kencana'
-- mengikuti kop Anda, 'PT UTAC ...' mengikuti tulisan resmi mereka. Itu BUKAN
-- kelalaian — kolom ini dicetak apa adanya, jadi masing-masing membawa ejaannya
-- sendiri.
INSERT INTO companies (
    code, name, legal_name, company_type, description, address, email, established_date
) VALUES
    (
        'WIKEN',
        'Widia Kencana',
        'PT. Widia Kencana',
        'PT',
        'Services & Trading for Industry — Control & Monitoring System, General Supply',
        'Jl. Cihanjuang No. 180A, Cimahi 40513',
        'widia.kencana@yahoo.com',
        NULL
    ),
    (
        'UTAC',
        'UTAC Manufacturing Services Indonesia',
        'PT UTAC Manufacturing Services Indonesia',
        'PT',
        NULL,
        E'Jl. Maligi I Lot.A1-4, Kawasan Industri KIIC Sukaluyu\nTeluk Jambe Timur, Karawang 41361, Jawa Barat',
        NULL,
        NULL
    ),
    -- Alamat baris ini diambil dari PENAWARAN YANG SUDAH ANDA KIRIM, bukan dari
    -- profil publiknya. Keduanya berbeda, dan perbedaannya bukan kekeliruan:
    -- indokontraktor.com mencatat "Jalan Perum MGT, Cluster Bellagio, Blok M.5,
    -- NO.2, Kota Bekasi" — kemungkinan besar domisili terdaftar menurut NIB —
    -- sementara yang tercetak di dokumen adalah alamat surat-menyurat yang
    -- benar-benar dipakai.
    --
    -- Yang dipilih alamat dokumen, karena kolom ini DICETAK dan sudah terbukti
    -- sampai. Alamat terdaftar ditulis di komentar ini supaya tidak hilang; ia
    -- baru butuh kolom sendiri kalau invoice atau faktur pajak menuntut domisili
    -- resmi, dan saat itu tiba tabelnya perlu alamat kedua, bukan penggantian.
    --
    -- legal_name memakai bentuk yang tercetak di penawaran — "PT. Dizkaa Putra
    -- Anugerah". Profil publiknya menulis tanpa "PT", tetapi yang disalin ke
    -- dokumen adalah kolom ini, dan yang sudah dikirim ke pelanggan memakai
    -- bentuk itu.
    --
    -- established_date bersumber dari profil publik: 16 November 2023.
    (
        'DIZKAA',
        'Dizkaa Putra Anugerah',
        'PT. Dizkaa Putra Anugerah',
        'PT',
        'Perusahaan jasa konstruksi',
        E'Ruko New Westfield, Jl. Western Boulevard No. 32 Blok ER7\nMustika Jaya, Bekasi 17158',
        NULL,
        DATE '2023-11-16'
    )
ON CONFLICT (UPPER(code)) DO NOTHING;
