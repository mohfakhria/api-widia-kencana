package usecase

import (
	"context"
	"strings"

	"github.com/mohfakhria/api-widia-kencana/internal/domain"
	"github.com/mohfakhria/api-widia-kencana/internal/domain/entity"
	"github.com/mohfakhria/api-widia-kencana/internal/usecase/port/input"
	"github.com/mohfakhria/api-widia-kencana/internal/usecase/port/output"
)

// allowedCompanyStatuses adalah kosakata tertutup, sejalan dengan CHECK di
// migrasinya.
//
// 'suspended' sengaja tidak ada: selama tidak satu pun tempat memperlakukannya
// berbeda dari 'inactive', ia hanya kata kedua untuk satu keadaan.
var allowedCompanyStatuses = map[string]struct{}{
	"active":   {},
	"inactive": {},
}

const defaultCompanyStatus = "active"

// Bawaan yang hanya berarti bagi perusahaan PENERBIT dokumen. Pada baris
// pelanggan ketiganya terisi tetapi tidak dipakai apa pun — lihat catatannya di
// migration/companies.sql.
const (
	defaultCompanyTimezone = "Asia/Jakarta"
	defaultCompanyLocale   = "id-ID"
	defaultCompanyCurrency = "IDR"
)

type companyUseCase struct {
	repo output.CompanyRepository
}

func NewCompanyUseCase(repo output.CompanyRepository) input.CompanyUseCase {
	return &companyUseCase{repo: repo}
}

func (uc *companyUseCase) List(ctx context.Context, query input.ListCompanyQuery) ([]entity.Company, error) {
	query.Status = strings.ToLower(strings.TrimSpace(query.Status))
	if query.Status != "" {
		if _, ok := allowedCompanyStatuses[query.Status]; !ok {
			return nil, domain.NewError(domain.ErrInvalidInput, "invalid company status")
		}
	}
	query.Search = strings.TrimSpace(query.Search)

	return uc.repo.List(ctx, query)
}

func (uc *companyUseCase) Get(ctx context.Context, id string) (*entity.Company, error) {
	companyID, err := parseUUID(id, "company id")
	if err != nil {
		return nil, err
	}

	company, err := uc.repo.GetByID(ctx, companyID)
	if err != nil {
		return nil, err
	}

	contacts, err := uc.repo.ListContacts(ctx, companyID)
	if err != nil {
		return nil, err
	}
	company.Contacts = contacts

	return company, nil
}

func (uc *companyUseCase) Create(ctx context.Context, cmd input.CompanyCommand) (*entity.Company, error) {
	company := mapCompanyCommand(cmd)
	if err := validateCompany(company); err != nil {
		return nil, err
	}

	return uc.repo.Create(ctx, company)
}

func (uc *companyUseCase) Update(ctx context.Context, id string, cmd input.CompanyCommand) error {
	companyID, err := parseUUID(id, "company id")
	if err != nil {
		return err
	}

	company := mapCompanyCommand(cmd)
	if err := validateCompany(company); err != nil {
		return err
	}

	return uc.repo.Update(ctx, companyID, company)
}

func (uc *companyUseCase) Delete(ctx context.Context, id string) error {
	companyID, err := parseUUID(id, "company id")
	if err != nil {
		return err
	}

	// Kontaknya ikut terhapus lewat ON DELETE CASCADE, bukan dihapus di sini.
	// Kontak tidak punya hidup di luar perusahaannya, dan menghapusnya lewat dua
	// pernyataan terpisah membuka jendela ketika salah satunya sudah hilang.
	return uc.repo.Delete(ctx, companyID)
}

func (uc *companyUseCase) AddContact(ctx context.Context, companyID string, cmd input.CompanyContactCommand) (*entity.CompanyContact, error) {
	id, err := parseUUID(companyID, "company id")
	if err != nil {
		return nil, err
	}

	// Perusahaannya dipastikan ada lebih dulu supaya jawabannya "company not
	// found", bukan pelanggaran foreign key yang menyebut nama constraint.
	if _, err := uc.repo.GetByID(ctx, id); err != nil {
		return nil, err
	}

	contact := mapCompanyContactCommand(cmd)
	contact.CompanyID = id
	if err := validateCompanyContact(contact); err != nil {
		return nil, err
	}

	return uc.repo.CreateContact(ctx, contact)
}

func (uc *companyUseCase) UpdateContact(ctx context.Context, contactID string, cmd input.CompanyContactCommand) error {
	id, err := parseUUID(contactID, "contact id")
	if err != nil {
		return err
	}

	existing, err := uc.repo.GetContactByID(ctx, id)
	if err != nil {
		return err
	}

	contact := mapCompanyContactCommand(cmd)
	contact.CompanyID = existing.CompanyID
	if err := validateCompanyContact(contact); err != nil {
		return err
	}

	return uc.repo.UpdateContact(ctx, id, contact)
}

func (uc *companyUseCase) DeleteContact(ctx context.Context, contactID string) error {
	id, err := parseUUID(contactID, "contact id")
	if err != nil {
		return err
	}

	return uc.repo.DeleteContact(ctx, id)
}

func mapCompanyCommand(cmd input.CompanyCommand) *entity.Company {
	status := strings.ToLower(strings.TrimSpace(cmd.Status))
	if status == "" {
		status = defaultCompanyStatus
	}

	return &entity.Company{
		// Code TIDAK dihuruf-besarkan, hanya dipangkas. Yang menegakkan keunikan
		// adalah indeks pada UPPER(code), sehingga 'wiken' tetap bertabrakan
		// dengan 'WIKEN' — tetapi bentuk yang diketik orang disimpan apa adanya,
		// karena kode ini muncul di layar.
		Code:            strings.TrimSpace(cmd.Code),
		Name:            strings.TrimSpace(cmd.Name),
		LegalName:       strings.TrimSpace(cmd.LegalName),
		CompanyType:     strings.TrimSpace(cmd.CompanyType),
		Description:     strings.TrimSpace(cmd.Description),
		EstablishedDate: cmd.EstablishedDate,
		// Address TIDAK dipangkas per baris — ia dicetak apa adanya, dan
		// merapikannya di sini berarti backend diam-diam mengubah tata letak yang
		// sudah dilihat orang saat mengetiknya.
		Address:      strings.Trim(cmd.Address, " \t\n"),
		Email:        strings.TrimSpace(cmd.Email),
		Phone:        strings.TrimSpace(cmd.Phone),
		Fax:          strings.TrimSpace(cmd.Fax),
		Timezone:     firstNonEmpty(strings.TrimSpace(cmd.Timezone), defaultCompanyTimezone),
		Locale:       firstNonEmpty(strings.TrimSpace(cmd.Locale), defaultCompanyLocale),
		CurrencyCode: strings.ToUpper(firstNonEmpty(strings.TrimSpace(cmd.CurrencyCode), defaultCompanyCurrency)),
		Status:       status,
	}
}

func validateCompany(company *entity.Company) error {
	if company.Code == "" {
		return domain.NewError(domain.ErrInvalidInput, "company code cannot be empty")
	}
	if company.Name == "" {
		return domain.NewError(domain.ErrInvalidInput, "company name cannot be empty")
	}
	if company.LegalName == "" {
		return domain.NewError(domain.ErrInvalidInput, "company legal name cannot be empty")
	}
	if _, ok := allowedCompanyStatuses[company.Status]; !ok {
		return domain.NewError(domain.ErrInvalidInput, "invalid company status")
	}
	// Tiga huruf, sesuai ISO 4217 dan sesuai CHAR(3) di kolomnya. Yang lebih
	// panjang akan dipotong diam-diam oleh database, dan potongan itu tidak
	// pernah muncul sebagai galat.
	if len(company.CurrencyCode) != 3 {
		return domain.NewError(domain.ErrInvalidInput, "company currency code must be 3 letters")
	}

	return nil
}

func mapCompanyContactCommand(cmd input.CompanyContactCommand) *entity.CompanyContact {
	status := strings.ToLower(strings.TrimSpace(cmd.Status))
	if status == "" {
		status = defaultCompanyStatus
	}

	return &entity.CompanyContact{
		Salutation: strings.TrimSpace(cmd.Salutation),
		Name:       strings.TrimSpace(cmd.Name),
		Position:   strings.TrimSpace(cmd.Position),
		Department: strings.TrimSpace(cmd.Department),
		Email:      strings.TrimSpace(cmd.Email),
		Phone:      strings.TrimSpace(cmd.Phone),
		Mobile:     strings.TrimSpace(cmd.Mobile),
		IsPrimary:  cmd.IsPrimary,
		Status:     status,
	}
}

func validateCompanyContact(contact *entity.CompanyContact) error {
	if contact.Name == "" {
		return domain.NewError(domain.ErrInvalidInput, "contact name cannot be empty")
	}
	if _, ok := allowedCompanyStatuses[contact.Status]; !ok {
		return domain.NewError(domain.ErrInvalidInput, "invalid contact status")
	}

	// Kontak utama yang tidak aktif adalah dua pernyataan yang saling
	// meniadakan, dan indeks uniknya pun hanya menghitung yang aktif — sehingga
	// membiarkannya berarti perusahaan itu tampak punya kontak utama padahal
	// tidak ada yang akan tercetak.
	if contact.IsPrimary && contact.Status != "active" {
		return domain.NewError(domain.ErrInvalidInput, "primary contact must be active")
	}

	return nil
}

func firstNonEmpty(value, fallback string) string {
	if value == "" {
		return fallback
	}

	return value
}
