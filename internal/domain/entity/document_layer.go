package entity

import "time"

type DocumentLayer struct {
	ID                int64
	Token             string
	DocumentID        int64
	DocumentToken     string
	ParentID          *int64
	ParentToken       string
	DocumentElementID int64
	Element           DocumentElement
	Region            string
	Name              string
	Content           map[string]any
	Properties        map[string]any
	Position          int
	Status            string
	CreatedAt         time.Time
	UpdatedAt         time.Time
	Children          []DocumentLayer
}
