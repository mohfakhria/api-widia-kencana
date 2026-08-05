package entity

import "time"

type Document struct {
	ID              int64
	Token           string
	DocumentPaperID int64
	Paper           DocumentPaper
	ParentID        *int64
	ParentToken     string
	Name            string
	DocumentType    string
	Status          string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type DocumentPaper struct {
	ID             int64
	Token          string
	Name           string
	MediaType      string
	Width          float64
	Height         float64
	Unit           string
	AllowPortrait  bool
	AllowLandscape bool
	Status         string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}
