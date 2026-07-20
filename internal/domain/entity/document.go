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
	Settings        map[string]any
	Position        int
	Status          string
	CreatedAt       time.Time
	UpdatedAt       time.Time
	WithLayers      bool
	Layers          DocumentLayerRegions
}

type DocumentLayerRegions struct {
	Header []DocumentLayer
	Body   []DocumentLayer
	Footer []DocumentLayer
}
