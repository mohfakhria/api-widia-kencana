package entity

import "time"

type Project struct {
	ID        int64
	Name      string
	Status    string
	CreatedAt time.Time
	UpdatedAt time.Time
}
