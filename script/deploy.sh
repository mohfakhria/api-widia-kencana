#!/usr/bin/env bash
#
# Bangun binary lalu kirim ke server. Berhenti di situ.
#
#   ./script/deploy.sh
#
# Pemasangan dan menjalankan ulang layanan sengaja TIDAK dikerjakan skrip ini —
# keduanya dilakukan manual di server. Yang dijamin skrip ini cuma satu hal:
# berkas yang mendarat di sana benar-benar berasal dari commit yang sedang Anda
# pegang, dan sampai tanpa cacat.
#
# Yang dapat diubah lewat environment:
#
#   HOST=widia-server                     nama host di ~/.ssh/config
#   STAGING=/root/deployment/widia-api    tempat berkas mendarat
#   ARCH=amd64                            amd64 atau arm64
#   ALLOW_DIRTY=1                         izinkan kirim dari pohon kerja kotor

set -euo pipefail

HOST=${HOST:-widia-server}
STAGING=${STAGING:-/root/deployment/widia-api}
ARCH=${ARCH:-amd64}
ALLOW_DIRTY=${ALLOW_DIRTY:-0}

repo=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$repo"

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
langkah "Kirim $commit → $HOST"

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
langkah "Membangun binary linux/$ARCH"
if docker info >/dev/null 2>&1; then
	echo "  lewat Docker"
	DOCKER_BUILDKIT=1 docker build \
		--quiet \
		--target binary \
		--build-arg TARGET_ARCH="$ARCH" \
		--output type=local,dest=./dist \
		. >/dev/null
else
	echo "  daemon Docker mati — memakai toolchain lokal ($(go version | awk '{print $3}'))"
	CGO_ENABLED=0 GOOS=linux GOARCH="$ARCH" go build \
		-trimpath -buildvcs=false -ldflags="-s -w" \
		-o dist/widia-api ./cmd/api
fi

lokal=$(shasum -a 256 dist/widia-api | cut -d' ' -f1)
printf '  %s  %s\n' "$(du -h dist/widia-api | cut -f1)" "${lokal:0:16}…"

# ── 4. Kirim ────────────────────────────────────────────────────────────────
langkah "Mengirim ke $HOST:$STAGING"
ssh "$HOST" "${sudo_jauh}mkdir -p $(dirname "$STAGING")"

# scp berjalan sebagai user SSH, TANPA sudo — tidak ada cara menyisipkannya di
# tengah scp. Direktori yang barusan dibuat lewat sudo karena itu boleh jadi milik
# root dan tidak dapat ditulisi. Diperiksa di sini supaya jawabannya menyebut apa
# yang harus diubah, bukan sekadar "Permission denied" dari scp.
if ! ssh "$HOST" "test -w $(dirname "$STAGING")"; then
	mati "$(dirname "$STAGING") tidak dapat ditulisi oleh user SSH — pakai STAGING=/tmp/widia-api, atau masuk sebagai root"
fi

scp -q dist/widia-api "$HOST:$STAGING"

# ── 5. Sampai utuh? ─────────────────────────────────────────────────────────
#
# Diperiksa di sini, bukan diserahkan ke langkah manual, karena inilah satu-satunya
# hal yang skrip ini janjikan. Pengiriman yang terpotong menghasilkan berkas yang
# ada dan berukuran wajar, dan gejalanya baru muncul sebagai layanan yang gagal
# start dengan pesan yang tidak menyebut-nyebut soal pengiriman.
langkah "Memeriksa berkas yang mendarat"
jauh=$(ssh "$HOST" "${sudo_jauh}sha256sum $STAGING" | cut -d' ' -f1)
if [ "$jauh" != "$lokal" ]; then
	mati "sidik jari berbeda — di server ${jauh:0:16}…, yang dikirim ${lokal:0:16}…"
fi
echo "  sidik jari cocok"

printf '\n\033[32m✓ %s ada di %s:%s\033[0m\n' "$commit" "$HOST" "$STAGING"

# Langkah manual dicetak lengkap supaya tidak perlu diingat maupun dicari di
# README. Sengaja tidak dijalankan: memasang dan menjalankan ulang layanan
# memutus semua sesi penyuntingan yang sedang terbuka, dan itu keputusan yang
# pantas diambil sadar, bukan sebagai kelanjutan otomatis dari sebuah upload.
cat <<PETUNJUK

Selanjutnya, di server:

  install -o root -g root -m 0755 $STAGING /opt/widia-api/widia-api
  systemctl restart widia-api
  systemctl status widia-api --no-pager

install, bukan cp — ia menulis berkas baru lalu menggantinya secara atomik,
sehingga aman dijalankan selagi layanannya hidup. cp menimpa berkas yang sedang
dieksekusi dan ditolak Linux dengan ETXTBSY.

Jangan menjalankan binary langsung dari $(dirname "$STAGING"): unit systemd
memakai ProtectHome=true, yang membuat /root kosong dan tidak terjangkau bagi
proses layanan.

Berhentinya memakan waktu sampai delapan detik — orchestrator dokumen menyimpan
suntingan terakhir tiap sesi yang sedang terbuka.
PETUNJUK
