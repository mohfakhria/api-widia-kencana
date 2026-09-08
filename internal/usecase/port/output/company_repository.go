package output

import (
	"context"

	"github.com/mohfakhria/api-widia-kencana/internal/domain/entity"
	"github.com/mohfakhria/api-widia-kencana/internal/usecase/port/input"
)

type CompanyRepository interface {
	List(ctx context.Context, query input.ListCompanyQuery) ([]entity.Company, error)
	GetByID(ctx context.Context, id string) (*entity.Company, error)
	Create(ctx context.Context, company *entity.Company) (*entity.Company, error)
	Update(ctx context.Context, id string, company *entity.Company) error
	Delete(ctx context.Context, id string) error

	ListContacts(ctx context.Context, companyID string) ([]entity.CompanyContact, error)

	// CreateContact dan UpdateContact menurunkan kontak utama yang lama SENDIRI,
	// di dalam satu transaksi bersama penulisannya.
	//
	// Indeks unik parsial di database menolak kontak utama kedua, jadi menaikkan
	// yang baru tanpa menurunkan yang lama akan gagal — dan menjadikannya dua
	// panggilan terpisah berarti ada saat ketika perusahaan itu TIDAK punya
	// kontak utama sama sekali, atau punya dua bila yang kedua gagal.
	CreateContact(ctx context.Context, contact *entity.CompanyContact) (*entity.CompanyContact, error)
	UpdateContact(ctx context.Context, id string, contact *entity.CompanyContact) error
	DeleteContact(ctx context.Context, id string) error
	GetContactByID(ctx context.Context, id string) (*entity.CompanyContact, error)
}
