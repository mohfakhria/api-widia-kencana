package input

import (
	"context"
	"time"

	"github.com/mohfakhria/api-widia-kencana/internal/domain/entity"
)

type CompanyUseCase interface {
	List(ctx context.Context, query ListCompanyQuery) ([]entity.Company, error)

	// Get menyertakan kontaknya; List tidak. Lihat alasannya di entity.Company.
	Get(ctx context.Context, id string) (*entity.Company, error)
	Create(ctx context.Context, cmd CompanyCommand) (*entity.Company, error)
	Update(ctx context.Context, id string, cmd CompanyCommand) error
	Delete(ctx context.Context, id string) error

	AddContact(ctx context.Context, companyID string, cmd CompanyContactCommand) (*entity.CompanyContact, error)
	UpdateContact(ctx context.Context, contactID string, cmd CompanyContactCommand) error
	DeleteContact(ctx context.Context, contactID string) error
}

type ListCompanyQuery struct {
	Status string
	// Search mencocokkan code, name, dan legal_name sekaligus. Satu kotak
	// pencarian, karena orang yang mencari pelanggan tidak lebih dulu memutuskan
	// sedang mengetik kode atau nama.
	Search string
}

// CompanyCommand dipakai membuat MAUPUN memperbarui.
//
// Satu bentuk untuk keduanya, dan itu disengaja: pembaruan di sini mengganti
// seluruh isi barisnya, bukan menambal sebagian. Master data disunting lewat
// satu formulir yang memuat semuanya, jadi "tidak disebut" memang berarti
// dikosongkan — berbeda dari page.update pada dokumen, yang ditambal
// per-properti oleh banyak penyunting sekaligus.
type CompanyCommand struct {
	Code            string
	Name            string
	LegalName       string
	CompanyType     string
	Description     string
	EstablishedDate *time.Time
	Address         string
	Email           string
	Phone           string
	Fax             string
	Timezone        string
	Locale          string
	CurrencyCode    string
	Status          string
}

type CompanyContactCommand struct {
	Salutation string
	Name       string
	Position   string
	Department string
	Email      string
	Phone      string
	Mobile     string
	IsPrimary  bool
	Status     string
}
