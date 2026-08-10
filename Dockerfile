# syntax=docker/dockerfile:1.7

# Docker di sini adalah KOTAK BUILD, bukan cara menjalankan aplikasi.
#
# Hasil akhirnya satu berkas binary yang dijalankan systemd langsung di host —
# lihat deploy/widia-api.service. Gunanya Docker cuma satu: memastikan binary
# yang mendarat di server dibangun oleh toolchain yang sama persis setiap kali,
# tanpa bergantung pada Go yang terpasang di laptop siapa pun.
#
#   docker build --target binary --output type=local,dest=./dist .
#
# Tahap terakhir memakai scratch dan hanya berisi binary-nya, sehingga --output
# menyalinnya apa adanya ke ./dist. Tidak ada image yang perlu disimpan maupun
# dijalankan.

ARG GO_VERSION=1.24.4

FROM golang:${GO_VERSION}-alpine AS builder

# git dipasang untuk berjaga bila ada dependensi yang diambil lewat VCS, bukan
# lewat module proxy. Tahap ini dibuang seluruhnya, jadi ongkosnya nol.
RUN apk add --no-cache git

WORKDIR /src

# Manifes disalin lebih dulu supaya unduhan dependensi tetap terpakai dari cache
# selama go.mod dan go.sum belum berubah. Menyalin seluruh sumber lebih dulu
# membuat setiap perubahan satu baris kode mengunduh ulang semuanya.
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .

# amd64 untuk server x86 kebanyakan; arm64 untuk Graviton, Ampere, dan sejenisnya.
ARG TARGET_ARCH=amd64

# CGO_ENABLED=0 menghasilkan ELF yang benar-benar statis — tidak menuntut glibc,
# musl, maupun pustaka apa pun di server. Binary yang sama berjalan di Debian,
# Ubuntu, maupun Alpine, dan tidak ikut rusak saat distro menaikkan versi glibc.
#
# -trimpath membuang jalur absolut mesin pembangun dari binary; -s -w membuang
# tabel simbol dan DWARF, memangkas ukurannya sekitar sepertiga. Keduanya membuat
# hasil build dapat diulang dan tidak membocorkan struktur direktori pembangunnya.
#
# -buildvcs=false disetel eksplisit karena .git sengaja tidak ikut masuk konteks
# build. Tanpa ini, Go dapat menolak membangun ketika ia menemukan repo tetapi
# tidak dapat menjalankan git di dalamnya.
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux GOARCH=${TARGET_ARCH} \
    go build \
        -trimpath \
        -buildvcs=false \
        -ldflags="-s -w" \
        -o /out/widia-api \
        ./cmd/api

# Tahap yang diekspor. Kosong selain binary-nya.
FROM scratch AS binary
COPY --from=builder /out/widia-api /widia-api
