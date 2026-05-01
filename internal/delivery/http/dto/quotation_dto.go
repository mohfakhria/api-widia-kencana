package dto

type QuotationRequest struct {
	ClientName    string                    `json:"client_name"`
	AttnName      string                    `json:"attn_name"`
	AttnPosition  string                    `json:"attn_position"`
	Address       string                    `json:"address"`
	Project       string                    `json:"project"`
	DiscountType  string                    `json:"discount_type"`
	DiscountValue float64                   `json:"discount_value"`
	SubTotal      float64                   `json:"subtotal"`
	Total         float64                   `json:"total"`
	Notes         []string                  `json:"notes"`
	Sections      []QuotationSectionRequest `json:"sections"`
}

type QuotationSectionRequest struct {
	Title    string                   `json:"title"`
	Position int                      `json:"position"`
	Items    []QuotationItemRequest   `json:"items"`
	Details  []QuotationDetailRequest `json:"details"`
}

type QuotationItemRequest struct {
	Name  string  `json:"name"`
	Qty   float64 `json:"qty"`
	Unit  string  `json:"unit"`
	Price float64 `json:"price"`
}

type QuotationDetailRequest struct {
	Description string `json:"description"`
	Position    int    `json:"position"`
}
