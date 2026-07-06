package entity

import "time"

type WorkflowStep struct {
	ID              int64
	WorkflowStageID int64
	Name            string
	Position        int
	Status          string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}
