#!/usr/bin/env bash
#
# Deploy MCP. Satu perintah.
#
#   ./script/deploy_mcp.sh
#
# Ini PEMBUNGKUS tipis di atas script/deploy.sh, bukan salinannya. Seluruh
# pekerjaan sungguhan — pemeriksaan pohon kerja, build yang dapat diulang,
# pengiriman berikut pencocokan sidik jari, pemasangan atomik, restart, dan
# health check — ada di sana. Yang di bawah ini hanya menyebut layanan mana.
#
# Dibuat begini dengan sengaja: dua salinan skrip deploy pasti melenceng, dan
# yang melenceng adalah yang lebih jarang dijalankan — persis yang akan
# dipercayai buta ketika ada yang rusak.
#
# Seluruh variabel deploy.sh tetap berlaku dan dapat ditimpa, misalnya:
#
#   HOST=widia-server ./script/deploy_mcp.sh
#   ALLOW_DIRTY=1 ./script/deploy_mcp.sh
#
# PERBEDAAN PENTING dari deploy API: restart di sini TIDAK memutus sesi
# penyuntingan siapa pun. MCP tidak memegang koneksi pengguna; yang terputus
# hanya perintah agent yang kebetulan sedang berjalan. Ia juga login ulang
# sendiri setelahnya, sehingga tidak ada yang perlu disentuh manual.

set -euo pipefail

repo=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)

# Bawaan khusus MCP. Setiap satu tetap dapat ditimpa dari luar, karena bentuk
# ${VAR:-bawaan} di deploy.sh menghormati apa pun yang sudah tersetel.
export PACKAGE="${PACKAGE:-./cmd/mcp}"
export BINARY="${BINARY:-widia-mcp}"
export SERVICE="${SERVICE:-widia-mcp}"
export STAGING="${STAGING:-/root/deployment/widia-mcp}"
export TARGET="${TARGET:-/opt/widia-mcp/widia-mcp}"
export ENV_FILE="${ENV_FILE:-/etc/widia-mcp/mcp.env}"

# MCP_PORT, bukan APP_PORT. Health check deploy.sh membaca kunci ini dari
# ENV_FILE untuk tahu port mana yang harus ditanya; kunci yang salah membuatnya
# jatuh ke 8080 dan menguji layanan yang KELIRU — yang lebih buruk daripada
# tidak menguji sama sekali, karena hasilnya hijau.
export PORT_KEY="${PORT_KEY:-MCP_PORT}"

exec "$repo/script/deploy.sh"
