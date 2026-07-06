package entity

import "time"

type Workflow struct {
	ID        int64
	Name      string
	Status    string
	CreatedAt time.Time
	UpdatedAt time.Time
}
