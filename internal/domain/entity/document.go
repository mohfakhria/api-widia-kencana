package entity

import "time"

type Document struct {
	ID              int64
	Token           string
	DocumentPaperID int64
	ParentID        *int64
	ParentToken     string
	Paper           DocumentPaper
	Name            string
	DocumentType    string
	Position        int
	Status          string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}
