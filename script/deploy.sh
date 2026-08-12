#!/usr/bin/env bash
#
# Bangun, kirim, pasang, jalankan ulang. Satu perintah.
#
#   ./script/deploy.sh
#
# PERLU DIINGAT: langkah restart MEMUTUS SEMUA SESI PENYUNTINGAN yang sedang
# terbuka. Suntingannya tidak hilang — orchestrator dokumen menyimpan yang
# terakhir sebelum berhenti, dan itu alasan TimeoutStopSec=30s pada unit
# systemd — tetapi orang yang sedang menggambar akan melihat editornya terputus
# lalu menyambung ulang. Pilih waktunya.
#
# Yang dapat diubah lewat environment:
#
#   HOST=widia-server                     nama host di ~/.ssh/config
#   STAGING=/root/deployment/widia-api    tempat berkas mendarat
#   TARGET=/opt/widia-api/widia-api       tempat berkas dijalankan
#   SERVICE=widia-api                     nama unit systemd
#   ENV_FILE=/etc/widia-api/api.env       dibaca hanya untuk tahu APP_PORT
#   ARCH=amd64                            amd64 atau arm64
#   ALLOW_DIRTY=1                         izinkan deploy dari pohon kerja kotor
#   PACKAGE=./cmd/api                     paket Go yang dibangun
#   BINARY=widia-api                      nama berkas hasil build
#   PORT_KEY=APP_PORT                     kunci di ENV_FILE untuk health check
#
# Keempat variabel terakhir ada supaya skrip ini melayani LEBIH DARI SATU
# layanan — lihat script/deploy_mcp.sh, yang hanya menyetelnya lalu memanggil
# skrip ini. Menyalin skrip ini menjadi versi kedua berarti dua salinan yang
# pasti melenceng, dan yang melenceng adalah yang lebih jarang dijalankan.
#
# STAGING dan TARGET sengaja berbeda, dan bukan sekadar kebiasaan. Unit systemd
# memakai ProtectHome=true, yang membuat /root kosong dan tidak terjangkau bagi
# proses layanan — binary yang dijalankan LANGSUNG dari sana tidak akan pernah
# ditemukan, dan systemd menjawabnya dengan 203/EXEC yang tidak menyebut sama
# sekali bahwa penyebabnya pengerasan.

set -euo pipefail

HOST=${HOST:-widia-server}
STAGING=${STAGING:-/root/deployment/widia-api}
TARGET=${TARGET:-/opt/widia-api/widia-api}
SERVICE=${SERVICE:-widia-api}
ENV_FILE=${ENV_FILE:-/etc/widia-api/api.env}
ARCH=${ARCH:-amd64}
ALLOW_DIRTY=${ALLOW_DIRTY:-0}
PACKAGE=${PACKAGE:-./cmd/api}
BINARY=${BINARY:-widia-api}
PORT_KEY=${PORT_KEY:-APP_PORT}

repo=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$repo"

# ssh dan scp dibungkus supaya penerusan port dari ~/.ssh/config tidak ikut
# terbawa. Host yang punya LocalForward — misalnya untuk menjangkau Postgres dari
# laptop — membuat setiap koneksi baru mencoba mengikat port lokal yang sama, dan
# yang kedua seterusnya gagal dengan "Address already in use". Perintahnya tetap
# berjalan, tetapi keluarannya dipenuhi galat yang tidak ada hubungannya dengan
# deploy — dan galat sungguhan menjadi tidak terlihat di antaranya.
#
# Dibungkus sebagai fungsi, bukan ditambahkan di tiap pemanggilan, supaya tidak
# ada satu pun yang terlewat. command menghindari fungsi ini memanggil dirinya
# sendiri, dan tetap menghormati PATH.
ssh() { command ssh -o ClearAllForwardings=yes "$@"; }
scp() { command scp -o ClearAllForwardings=yes "$@"; }

langkah() { printf '\n\033[1m▸ %s\033[0m\n' "$1"; }
mati() { printf '\n\033[31m✗ %s\033[0m\n' "$1" >&2; exit 1; }

# ── 1. Apa yang sebenarnya akan dikirim ─────────────────────────────────────
#
# Pohon kerja yang kotor membuat binary tidak dapat ditelusuri kembali ke commit
# mana pun, dan itu menghapus satu-satunya cara membuktikan apa yang berjalan di
# server berasal dari sumber yang mana.
if [ -n "$(git status --porcelain)" ] && [ "$ALLOW_DIRTY" != "1" ]; then
	git status --short
	mati "pohon kerja kotor — commit dulu, atau jalankan ulang dengan ALLOW_DIRTY=1"
fi

commit=$(git rev-parse --short HEAD)
langkah "Deploy $commit → $HOST"

# ── 2. Server terjangkau? ───────────────────────────────────────────────────
#
# Ditanyakan SEBELUM membangun. Selain menghemat satu menit ketika servernya
# memang tidak terjangkau, ini menutup kegagalan yang lebih buruk: tanpa
# pemeriksaan ini, ssh yang gagal di dalam $(...) hanya menghasilkan string
# kosong, skrip menyimpulkan "berarti bukan root", lalu berjalan terus dan gagal
# di langkah berikutnya dengan pesan yang tidak menyebut penyebab sebenarnya.
if ! uid=$(ssh -o BatchMode=yes -o ConnectTimeout=10 "$HOST" id -u 2>&1); then
	printf '%s\n' "$uid" >&2
	mati "tidak dapat menyambung ke $HOST"
fi

if [ "$uid" = "0" ]; then
	sudo_jauh=""
else
	sudo_jauh="sudo "
	echo "  masuk sebagai uid $uid — perintah di server memakai sudo"
fi

# ── 3. Bangun ───────────────────────────────────────────────────────────────
#
# Selalu dibangun ulang, tidak pernah memakai dist/ apa adanya. Binary basi yang
# terlanjur terkirim adalah kegagalan yang paling sulit disadari: semuanya
# berhasil, layanannya hidup, dan yang berjalan bukan yang Anda kira.
#
# Docker didahulukan karena ia mematok versi Go lewat Dockerfile. Bila daemonnya
# mati, build lokal dipakai sebagai gantinya — dengan bendera yang sama persis,
# dan hasilnya terbukti identik byte per byte karena -trimpath dan -buildvcs=false
# membuat build ini dapat diulang.
langkah "Membangun $BINARY linux/$ARCH"
if docker info >/dev/null 2>&1; then
	echo "  lewat Docker"
	DOCKER_BUILDKIT=1 docker build \
		--quiet \
		--target binary \
		--build-arg TARGET_ARCH="$ARCH" \
		--build-arg PACKAGE="$PACKAGE" \
		--build-arg BINARY="$BINARY" \
		--output type=local,dest=./dist \
		. >/dev/null
else
	echo "  daemon Docker mati — memakai toolchain lokal ($(go version | awk '{print $3}'))"
	CGO_ENABLED=0 GOOS=linux GOARCH="$ARCH" go build \
		-trimpath -buildvcs=false -ldflags="-s -w" \
		-o "dist/$BINARY" "$PACKAGE"
fi

lokal=$(shasum -a 256 "dist/$BINARY" | cut -d' ' -f1)
printf '  %s  %s\n' "$(du -h "dist/$BINARY" | cut -f1)" "${lokal:0:16}…"

# ── 4. Kirim ────────────────────────────────────────────────────────────────
langkah "Mengirim ke $HOST:$STAGING"
ssh "$HOST" "${sudo_jauh}mkdir -p $(dirname "$STAGING")"

# STAGING harus berupa BERKAS, bukan direktori. Bila direktori dengan nama itu
# sudah ada, scp akan menaruh berkasnya DI DALAMNYA tanpa mengeluh, dan sisa
# skrip ini bekerja pada jalur yang isinya bukan yang dikira. Sudah pernah
# terjadi, dan gejalanya muncul jauh kemudian sebagai "install: omitting
# directory".
if ssh "$HOST" "test -d $STAGING"; then
	mati "$STAGING sudah ada sebagai DIREKTORI — hapus dulu: ssh $HOST 'rmdir $STAGING'"
fi

# scp berjalan sebagai user SSH, TANPA sudo — tidak ada cara menyisipkannya di
# tengah scp. Direktori yang barusan dibuat lewat sudo karena itu boleh jadi milik
# root dan tidak dapat ditulisi. Diperiksa di sini supaya jawabannya menyebut apa
# yang harus diubah, bukan sekadar "Permission denied" dari scp.
if ! ssh "$HOST" "test -w $(dirname "$STAGING")"; then
	mati "$(dirname "$STAGING") tidak dapat ditulisi oleh user SSH — pakai STAGING=/tmp/widia-api, atau masuk sebagai root"
fi

scp -q "dist/$BINARY" "$HOST:$STAGING"

jauh=$(ssh "$HOST" "${sudo_jauh}sha256sum $STAGING" | cut -d' ' -f1)
if [ "$jauh" != "$lokal" ]; then
	mati "pengiriman tidak utuh — di server ${jauh:0:16}…, yang dikirim ${lokal:0:16}…"
fi
echo "  sampai utuh"

# ── 5. Pasang lalu jalankan ulang ───────────────────────────────────────────
#
# install, bukan cp: ia menulis berkas baru lalu menggantinya secara atomik,
# sehingga aman dijalankan selagi layanannya hidup. cp menimpa berkas yang sedang
# dieksekusi, dan Linux menolaknya dengan ETXTBSY.
#
# install -d lebih dulu karena install BIASA tidak membuat direktori tujuan, dan
# pada pemasangan pertama /opt/widia-api memang belum ada.
langkah "Memasang ke $TARGET"
ssh "$HOST" "${sudo_jauh}install -d $(dirname "$TARGET") && ${sudo_jauh}install -o root -g root -m 0755 $STAGING $TARGET"

terpasang=$(ssh "$HOST" "${sudo_jauh}sha256sum $TARGET" | cut -d' ' -f1)
if [ "$terpasang" != "$lokal" ]; then
	mati "yang terpasang bukan yang dikirim — ${terpasang:0:16}… vs ${lokal:0:16}…"
fi
echo "  sidik jari cocok"

langkah "Menjalankan ulang $SERVICE"
echo "  sesi penyuntingan yang sedang terbuka akan terputus"
ssh "$HOST" "${sudo_jauh}systemctl restart $SERVICE"

# ── 6. Benarkah ia hidup ────────────────────────────────────────────────────
#
# Ditunggu, bukan ditanya sekali. Berhentinya sendiri dapat memakan delapan
# detik — orchestrator dokumen menyimpan suntingan terakhir tiap sesi — dan
# layanan yang gagal start berulang menghabiskan RestartSec di antara
# percobaannya, sehingga satu pertanyaan tepat setelah restart hampir selalu
# menjawab "activating" apa pun keadaan sebenarnya.
langkah "Memeriksa hasilnya"
hidup=0
for _ in $(seq 1 10); do
	if ssh "$HOST" "systemctl is-active --quiet $SERVICE"; then
		hidup=1
		break
	fi
	sleep 2
done

if [ "$hidup" != "1" ]; then
	ssh "$HOST" "${sudo_jauh}journalctl -u $SERVICE -n 30 --no-pager" || true
	mati "$SERVICE tidak hidup setelah dijalankan ulang"
fi
echo "  unit aktif"

# Health check adalah satu-satunya bukti aplikasinya benar-benar melayani, bukan
# sekadar prosesnya ada. Portnya dibaca dari berkas konfigurasi di server supaya
# tidak ada angka yang perlu diulang di dua tempat.
port=$(ssh "$HOST" "${sudo_jauh}grep -oE \"^$PORT_KEY=.*\" $ENV_FILE 2>/dev/null | cut -d= -f2" || true)
port=${port:-8080}

if ssh "$HOST" "command -v curl >/dev/null 2>&1"; then
	kode=$(ssh "$HOST" "curl -sS -o /dev/null -w '%{http_code}' --max-time 10 http://127.0.0.1:$port/health" || true)
	if [ "$kode" != "200" ]; then
		ssh "$HOST" "${sudo_jauh}journalctl -u $SERVICE -n 30 --no-pager" || true
		mati "health menjawab ${kode:-tidak ada} pada port $port, bukan 200"
	fi
	echo "  health 200 di port $port"
else
	echo "  curl tidak ada di server — health check dilewati"
fi

printf '\n\033[32m✓ %s berjalan di %s pada commit %s\033[0m\n' "$SERVICE" "$HOST" "$commit"
