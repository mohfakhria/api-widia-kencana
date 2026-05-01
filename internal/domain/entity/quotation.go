package entity

import "time"

type Quotation struct {
	ID            int64
	QuotationNo   string
	ClientName    string
	AttnName      string
	AttnPosition  string
	Address       string
	Project       string
	DiscountType  string
	DiscountValue float64
	SubTotal      float64
	Total         float64
	Notes         []string
	CreatedAt     time.Time
	UpdatedAt     time.Time
	Sections      []QuotationSection
}

type QuotationSection struct {
	ID          int64
	QuotationID int64
	Title       string
	Position    int
	Items       []QuotationItem
	Details     []QuotationDetail
}

type QuotationItem struct {
	ID        int64
	SectionID int64
	Name      string
	Qty       float64
	Unit      string
	Price     float64
	Total     float64
}

type QuotationDetail struct {
	ID          int64
	SectionID   int64
	Description string
	Position    int
}
