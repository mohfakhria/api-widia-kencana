package pdf

import (
	"github.com/mohfakhria/api-widia-kencana/internal/domain/design"
)

// drawTable menggambar tabel: isian, teks per sel, lalu grid.
//
// Urutannya mengikat. Isian menutupi apa pun di bawahnya, jadi ia harus
// mendahului teks; grid digambar TERAKHIR supaya garisnya tidak tertutup isian
// baris berikutnya — pada 0,75pt, separuh garis yang tertutup terlihat sebagai
// grid yang tebalnya tidak rata.
func (c *canvas) drawTable(element *design.Element) error {
	if element.W <= 0 || element.H <= 0 || len(element.Columns) == 0 {
		return nil
	}

	edges := columnEdges(element)
	tops := rowTops(element)

	c.fillRows(element, tops)

	if err := c.drawCells(element, edges, tops); err != nil {
		return err
	}

	c.drawGrid(element, edges, tops)

	return nil
}

// columnEdges mengembalikan tepi kiri setiap kolom beserta tepi kanan tabel.
//
// Dijumlahkan berjalan, bukan dihitung sendiri-sendiri sebagai share × W.
// Pembulatan float pada enam kolom cukup menyisakan celah setengah titik di
// antara isian dua kolom bersebelahan — celah yang terlihat sebagai garis putih
// tipis di kertas.
func columnEdges(element *design.Element) []float64 {
	edges := make([]float64, len(element.Columns)+1)
	edges[0] = element.X

	berjalan := 0.0
	for index, column := range element.Columns {
		berjalan += column.Share
		edges[index+1] = element.X + berjalan*element.W
	}

	// Tepi terakhir dipatok ke lebar tabel. Jumlah share diizinkan meleset
	// sepersekian ribu oleh shareTolerance, dan tanpa patokan ini selisih itu
	// muncul sebagai kolom terakhir yang tidak rata dengan tepi kanan.
	edges[len(edges)-1] = element.X + element.W

	return edges
}

// rowTops mengembalikan tepi atas setiap baris beserta tepi bawah baris terakhir.
//
// Tinggi dipakai APA ADANYA dari muatan. Frontend sudah mengukurnya dari teks
// yang dibungkus, dan menghitung ulang di sini berarti dua pengukur yang akan
// berselisih tepat pada baris yang teksnya membungkus.
func rowTops(element *design.Element) []float64 {
	tops := make([]float64, len(element.Rows)+1)
	tops[0] = element.Y

	for index, row := range element.Rows {
		tops[index+1] = tops[index] + row.Height
	}

	return tops
}

// fillRows mengecat latar tiap baris.
//
// Baris body BERSELANG-SELING dihitung dari baris pertama DI BAWAH header, jadi
// ada-tidaknya header tidak menggeser baris mana yang berwarna. Kosong berarti
// tidak dicat, bukan putih — konvensi yang sama dengan fill pada rect.
func (c *canvas) fillRows(element *design.Element, tops []float64) {
	for index, row := range element.Rows {
		warna := element.BodyFill
		if element.HeaderRow && index == 0 {
			warna = element.HeaderFill
		} else if element.StripeFill != "" {
			body := index
			if element.HeaderRow {
				body--
			}
			if body >= 0 && body%2 == 1 {
				warna = element.StripeFill
			}
		}

		if warna == "" || row.Height <= 0 {
			continue
		}

		red, green, blue, _ := design.ParseColor(warna)
		c.pdf.SetFillColor(red, green, blue)
		c.pdf.Rect(element.X, tops[index], element.W, row.Height, "F")
	}
}

// drawCells menggambar teks setiap sel.
func (c *canvas) drawCells(element *design.Element, edges, tops []float64) error {
	for rowIndex, row := range element.Rows {
		if row.Height <= 0 {
			continue
		}

		for cellIndex, cell := range row.Cells {
			if cell.Text == "" {
				continue
			}

			if err := c.drawCell(element, cell, cellBox{
				x:      edges[cellIndex],
				y:      tops[rowIndex],
				width:  edges[cellIndex+1] - edges[cellIndex],
				height: row.Height,
				align:  element.Columns[cellIndex].Align,
				header: element.HeaderRow && rowIndex == 0,
			}); err != nil {
				return err
			}
		}
	}

	return nil
}

// cellBox adalah kotak satu sel beserta yang membedakannya dari sel lain.
type cellBox struct {
	x, y, width, height float64
	align               string
	header              bool
}

// drawCell menggambar isi satu sel lewat mesin teks yang SAMA dengan elemen teks.
//
// Sel disusun sebagai design.Element bertipe teks, lalu diserahkan ke
// layoutText, baseline, dan drawTextLine yang dipakai kotak teks biasa. Itu
// disengaja dan merupakan inti paritas fitur ini: frontend melakukan hal yang
// persis sama lewat TextElement sementara, sehingga sel dan kotak teks tidak
// dapat berselisih soal di mana sebuah garis putus.
//
// Membangun satu Element per sel memang mengalokasi, tetapi ekspor bukan jalur
// panas — dan satu implementasi pembungkusan jauh lebih berharga daripada
// alokasi yang tidak pernah terasa.
func (c *canvas) drawCell(element *design.Element, cell design.TableCell, box cellBox) error {
	inner := box.width - 2*element.CellPadding
	if inner <= 0 {
		return nil
	}

	warna := element.ResolvedTextColor()
	if box.header && element.HeaderColor != "" {
		warna = element.HeaderColor
	}

	// Kotak isi, bukan kotak sel: pembungkusan memakai lebar setelah dikurangi
	// sisi dalam di kedua sisi, dan perataan tegak memakai tinggi yang sama
	// perlakuannya.
	teks := design.Element{
		ID:            element.ID,
		Type:          design.ElementText,
		X:             box.x + element.CellPadding,
		Y:             box.y + element.CellPadding,
		W:             inner,
		H:             box.height - 2*element.CellPadding,
		Text:          cell.Text,
		FontFamily:    element.FontFamily,
		FontSize:      element.FontSize,
		FontWeight:    cell.ResolvedCellWeight(),
		Color:         warna,
		Align:         box.align,
		LineHeight:    element.LineHeight,
		VerticalAlign: element.VerticalAlign,
	}
	if teks.H <= 0 {
		return nil
	}

	lines, font, err := c.layoutText(&teks)
	if err != nil {
		return err
	}

	red, green, blue, _ := design.ParseColor(teks.ResolvedColor())
	c.pdf.SetTextColor(red, green, blue)
	c.pdf.SetFillColor(red, green, blue)

	baseline, step := c.baseline(font, &teks)
	baseline += verticalOffset(teks.ResolvedVerticalAlign(), teks.H, float64(len(lines))*step)

	// Diklip ke kotak SEL, bukan ke kotak isi. Teks yang melebihi tinggi baris
	// dipotong alih-alih meluber ke baris berikutnya: meluber menghasilkan tabel
	// yang jelas rusak, sedangkan memotong kehilangan satu garis dengan cara yang
	// terkurung dan terlihat. Ini seharusnya tidak pernah terpicu — tinggi baris
	// diturunkan dari sel tertinggi — dan bila sering terpicu, itu pertanda
	// pengukur kedua sisi berselisih.
	c.pdf.ClipRect(box.x, box.y, box.width, box.height, false)
	for index, line := range lines {
		c.drawTextLine(&teks, line, teks.X, baseline+float64(index)*step,
			inner, box.align, 0, index == len(lines)-1)
	}
	c.pdf.ClipEnd()

	return nil
}

// drawGrid menggambar garis tabel.
//
// Hanya sisi ATAS dan KIRI setiap sel, ditutup tepi kanan dan bawah tabel.
// Menggambar keempat sisi tiap sel menggandakan setiap garis dalam, dan pada
// 0,75pt penggandaan itu terlihat di kertas sebagai grid yang tebalnya tidak
// rata. Tinggi elemen sudah memperhitungkan tepi penutup itu.
//
// borderWidth nol berarti tanpa grid sama sekali — salah satu preset, bukan
// kelalaian.
func (c *canvas) drawGrid(element *design.Element, edges, tops []float64) {
	if element.BorderWidth <= 0 || len(element.Rows) == 0 {
		return
	}

	red, green, blue, _ := design.ParseColor(element.BorderColor)
	c.pdf.SetDrawColor(red, green, blue)
	c.pdf.SetLineWidth(element.BorderWidth)

	bawah := tops[len(tops)-1]
	kanan := edges[len(edges)-1]

	// Garis mendatar: atas setiap baris, ditambah penutup bawah.
	for _, y := range tops {
		c.pdf.Line(element.X, y, kanan, y)
	}

	// Garis tegak: kiri setiap kolom, ditambah penutup kanan.
	for _, x := range edges {
		c.pdf.Line(x, element.Y, x, bawah)
	}
}
