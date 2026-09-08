package dto

import (
	"time"

	"github.com/mohfakhria/api-widia-kencana/internal/domain/entity"
	"github.com/mohfakhria/api-widia-kencana/internal/usecase/port/input"
)

// CompanyRequest dipakai membuat MAUPUN memperbarui, dan pembaruan mengganti
// seluruh isinya — field yang tidak dikirim menjadi kosong.
//
// Berbeda dari page.update pada dokumen, yang menolak field yang hilang: di sana
// halaman ditambal per-properti oleh banyak penyunting sekaligus. Master data
// disunting satu orang lewat satu formulir yang memuat semuanya.
type CompanyRequest struct {
	Code            string  `json:"code"`
	Name            string  `json:"name"`
	LegalName       string  `json:"legal_name"`
	CompanyType     string  `json:"company_type"`
	Description     string  `json:"description"`
	EstablishedDate *string `json:"established_date"`
	Address         string  `json:"address"`
	Email           string  `json:"email"`
	Phone           string  `json:"phone"`
	Fax             string  `json:"fax"`
	Timezone        string  `json:"timezone"`
	Locale          string  `json:"locale"`
	CurrencyCode    string  `json:"currency_code"`
	Status          string  `json:"status"`
}

type CompanyListFilterRequest struct {
	Status string `form:"status"`
	Search string `form:"search"`
}

type CompanyContactRequest struct {
	Salutation string `json:"salutation"`
	Name       string `json:"name"`
	Position   string `json:"position"`
	Department string `json:"department"`
	Email      string `json:"email"`
	Phone      string `json:"phone"`
	Mobile     string `json:"mobile"`
	IsPrimary  bool   `json:"is_primary"`
	Status     string `json:"status"`
}

type CompanyContactResponse struct {
	ID         string    `json:"id"`
	CompanyID  string    `json:"company_id"`
	Salutation string    `json:"salutation"`
	Name       string    `json:"name"`
	Position   string    `json:"position"`
	Department string    `json:"department"`
	Email      string    `json:"email"`
	Phone      string    `json:"phone"`
	Mobile     string    `json:"mobile"`
	IsPrimary  bool      `json:"is_primary"`
	Status     string    `json:"status"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type CompanyResponse struct {
	ID              string  `json:"id"`
	Code            string  `json:"code"`
	Name            string  `json:"name"`
	LegalName       string  `json:"legal_name"`
	CompanyType     string  `json:"company_type"`
	Description     string  `json:"description"`
	EstablishedDate *string `json:"established_date"`
	Address         string  `json:"address"`
	Email           string  `json:"email"`
	Phone           string  `json:"phone"`
	Fax             string  `json:"fax"`
	Timezone        string  `json:"timezone"`
	Locale          string  `json:"locale"`
	CurrencyCode    string  `json:"currency_code"`
	Status          string  `json:"status"`
	// Contacts hanya muncul pada company-detail. Pada daftar ia dihilangkan,
	// bukan dikirim kosong — larik kosong akan terbaca sebagai "perusahaan ini
	// memang tidak punya kontak".
	Contacts  []CompanyContactResponse `json:"contacts,omitempty"`
	CreatedAt time.Time                `json:"created_at"`
	UpdatedAt time.Time                `json:"updated_at"`
}

type CompanyDataResponse struct {
	Company CompanyResponse `json:"company"`
}

type CompanyListResponse struct {
	Companies []CompanyResponse `json:"companies"`
}

type CompanyContactDataResponse struct {
	Contact CompanyContactResponse `json:"contact"`
}

// dateLayout adalah bentuk established_date di JSON: tanggal saja, tanpa jam.
//
// Kolomnya DATE, dan mengirimkannya sebagai timestamp lengkap membuat klien
// harus memutuskan zona waktu untuk sesuatu yang tidak punya jam — lalu tanggal
// berdirinya bergeser sehari bagi orang yang membukanya dari zona lain.
const dateLayout = "2006-01-02"

func (r CompanyRequest) ToCompanyCommand() (input.CompanyCommand, error) {
	cmd := input.CompanyCommand{
		Code:         r.Code,
		Name:         r.Name,
		LegalName:    r.LegalName,
		CompanyType:  r.CompanyType,
		Description:  r.Description,
		Address:      r.Address,
		Email:        r.Email,
		Phone:        r.Phone,
		Fax:          r.Fax,
		Timezone:     r.Timezone,
		Locale:       r.Locale,
		CurrencyCode: r.CurrencyCode,
		Status:       r.Status,
	}

	if r.EstablishedDate != nil && *r.EstablishedDate != "" {
		parsed, err := time.Parse(dateLayout, *r.EstablishedDate)
		if err != nil {
			return cmd, err
		}
		cmd.EstablishedDate = &parsed
	}

	return cmd, nil
}

func (r CompanyContactRequest) ToCompanyContactCommand() input.CompanyContactCommand {
	return input.CompanyContactCommand{
		Salutation: r.Salutation,
		Name:       r.Name,
		Position:   r.Position,
		Department: r.Department,
		Email:      r.Email,
		Phone:      r.Phone,
		Mobile:     r.Mobile,
		IsPrimary:  r.IsPrimary,
		Status:     r.Status,
	}
}

func (r CompanyListFilterRequest) ToListCompanyQuery() input.ListCompanyQuery {
	return input.ListCompanyQuery{Status: r.Status, Search: r.Search}
}

func NewCompanyContactResponse(contact *entity.CompanyContact) CompanyContactResponse {
	return CompanyContactResponse{
		ID:         contact.ID,
		CompanyID:  contact.CompanyID,
		Salutation: contact.Salutation,
		Name:       contact.Name,
		Position:   contact.Position,
		Department: contact.Department,
		Email:      contact.Email,
		Phone:      contact.Phone,
		Mobile:     contact.Mobile,
		IsPrimary:  contact.IsPrimary,
		Status:     contact.Status,
		CreatedAt:  contact.CreatedAt,
		UpdatedAt:  contact.UpdatedAt,
	}
}

func NewCompanyResponse(company *entity.Company) CompanyResponse {
	response := CompanyResponse{
		ID:           company.ID,
		Code:         company.Code,
		Name:         company.Name,
		LegalName:    company.LegalName,
		CompanyType:  company.CompanyType,
		Description:  company.Description,
		Address:      company.Address,
		Email:        company.Email,
		Phone:        company.Phone,
		Fax:          company.Fax,
		Timezone:     company.Timezone,
		Locale:       company.Locale,
		CurrencyCode: company.CurrencyCode,
		Status:       company.Status,
		CreatedAt:    company.CreatedAt,
		UpdatedAt:    company.UpdatedAt,
	}

	if company.EstablishedDate != nil {
		tanggal := company.EstablishedDate.Format(dateLayout)
		response.EstablishedDate = &tanggal
	}

	for index := range company.Contacts {
		response.Contacts = append(response.Contacts, NewCompanyContactResponse(&company.Contacts[index]))
	}

	return response
}

func NewCompanyDataResponse(company *entity.Company) CompanyDataResponse {
	return CompanyDataResponse{Company: NewCompanyResponse(company)}
}

func NewCompanyContactDataResponse(contact *entity.CompanyContact) CompanyContactDataResponse {
	return CompanyContactDataResponse{Contact: NewCompanyContactResponse(contact)}
}

func NewCompanyListResponse(companies []entity.Company) CompanyListResponse {
	// Dibuat kosong, bukan nil: nil menjadi null di JSON, dan klien yang
	// melakukan iterasi atasnya gagal justru saat belum ada satu perusahaan pun.
	response := CompanyListResponse{Companies: make([]CompanyResponse, 0, len(companies))}
	for index := range companies {
		response.Companies = append(response.Companies, NewCompanyResponse(&companies[index]))
	}

	return response
}
