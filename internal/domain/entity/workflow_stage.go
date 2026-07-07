package entity

import "time"

type WorkflowStage struct {
	ID         int64
	WorkflowID int64
	Name       string
	Position   int
	Status     string
	CreatedAt  time.Time
	UpdatedAt  time.Time
	Steps      []WorkflowStep
}
