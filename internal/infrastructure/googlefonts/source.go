// Package googlefonts mengambil berkas TTF statik dari repositori google/fonts.
//
// Sumbernya repositori, BUKAN fonts.googleapis.com, dan itu keputusan yang
// dipaksa kenyataan: CSS API melayani woff2 kepada peramban modern dan URL tanpa
// ekstensi kepada UA lawas — keduanya tidak dapat disematkan ke PDF. Lebih dari
// itu, satu bobot dipecah menjadi beberapa berkas menurut unicode-range, jadi
// menanamkannya berarti menjahit potongan kembali menjadi font utuh.
//
// Berkas statik di repositori adalah TTF utuh, satu berkas per muka huruf, dan
// dapat langsung dipakai.
package googlefonts

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/mohfakhria/api-widia-kencana/internal/domain"
	"github.com/mohfakhria/api-widia-kencana/internal/domain/design"
)

const baseURL = "https://raw.githubusercontent.com/google/fonts/main"

// maxFontBytes membatasi berkas yang mau diterima.
//
// Ini masukan dari internet yang disimpan dan kelak dimuat ke memori saat
// ekspor. TTF keluarga latin berada di kisaran ratusan kilobyte; lima megabyte
// jauh melampauinya sekaligus menutup berkas yang keliru sepenuhnya.
const maxFontBytes = 5 << 20

// licenseDirs adalah direktori lisensi di repositori, dicoba berurutan.
//
// Tidak ada indeks yang memetakan nama keluarga ke direktorinya, jadi satu-
// satunya cara adalah mencoba. Ketiganya menutup hampir seluruh isi repositori,
// dan ofl didahulukan karena di situlah mayoritasnya.
var licenseDirs = []string{"ofl", "apache", "ufl"}

// weightNames memetakan bobot numerik ke penamaan berkas di repositori.
//
// Penamaannya konvensi, bukan aturan yang dijamin: sebagian keluarga hanya
// menyediakan font variabel dan tidak punya berkas statik sama sekali. Yang
// tidak ditemukan dilaporkan apa adanya, bukan diganti diam-diam dengan bobot
// lain — pengganti yang senyap di sini akan tersimpan sebagai bobot yang
// diminta, lalu berbohong selamanya.
var weightNames = map[int]string{
	100: "Thin",
	200: "ExtraLight",
	300: "Light",
	400: "Regular",
	500: "Medium",
	600: "SemiBold",
	700: "Bold",
	800: "ExtraBold",
	900: "Black",
}

type Source struct {
	http *http.Client
}

func New(timeout time.Duration) *Source {
	if timeout <= 0 {
		timeout = 20 * time.Second
	}

	return &Source{http: &http.Client{Timeout: timeout}}
}

func (s *Source) Fetch(ctx context.Context, family string, weight int, style string) ([]byte, error) {
	filename, err := fileName(family, weight, style)
	if err != nil {
		return nil, err
	}

	folder := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(family), " ", ""))

	var terakhir error
	for _, license := range licenseDirs {
		url := fmt.Sprintf("%s/%s/%s/%s", baseURL, license, folder, filename)

		data, err := s.get(ctx, url)
		if err == nil {
			return data, nil
		}
		terakhir = err
	}

	return nil, domain.NewError(domain.ErrNotFound, fmt.Sprintf(
		"font %s %d %s not found in the Google Fonts repository (%s): %v",
		family, weight, style, filename, terakhir))
}

func (s *Source) get(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := s.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}

	// Dibatasi saat membaca, bukan dipercaya dari Content-Length: header itu
	// datang dari pihak lain dan tidak mengikat berapa byte yang sungguh
	// dikirim.
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxFontBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxFontBytes {
		return nil, fmt.Errorf("font file is larger than %d bytes", maxFontBytes)
	}

	return data, nil
}

// fileName menyusun nama berkas menurut konvensi repositori: Family-Weight.ttf,
// dengan Italic menempel di belakang nama bobot — dan "Italic" sendirian untuk
// bobot 400, karena "RegularItalic" tidak dipakai di sana.
func fileName(family string, weight int, style string) (string, error) {
	name, ok := weightNames[weight]
	if !ok {
		return "", domain.NewError(domain.ErrInvalidInput, fmt.Sprintf(
			"font weight %d is not a static weight", weight))
	}

	base := strings.ReplaceAll(strings.TrimSpace(family), " ", "")
	if !strings.EqualFold(style, design.FontStyleItalic) {
		return fmt.Sprintf("%s-%s.ttf", base, name), nil
	}
	if weight == 400 {
		return fmt.Sprintf("%s-Italic.ttf", base), nil
	}

	return fmt.Sprintf("%s-%sItalic.ttf", base, name), nil
}
