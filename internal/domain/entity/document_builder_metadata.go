package entity

import "time"

type DocumentBuilderMetadata struct {
	Papers   []DocumentPaper
	Elements []DocumentElement
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

type DocumentElement struct {
	ID           int64
	Token        string
	Code         string
	Name         string
	RendererType string
	RendererTag  string
	ContentType  string
	IsContainer  bool
	Status       string
	CreatedAt    time.Time
	UpdatedAt    time.Time
	Properties   []DocumentElementProperty
}

type DocumentProperty struct {
	ID           int64
	Token        string
	Code         string
	Name         string
	DataType     string
	InputType    string
	DefaultValue string
	Unit         string
	Status       string
	CreatedAt    time.Time
	UpdatedAt    time.Time
	Options      []DocumentPropertyOption
}

type DocumentPropertyOption struct {
	ID                 int64
	Token              string
	DocumentPropertyID int64
	Value              string
	Label              string
	Position           int
	Status             string
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

type DocumentElementProperty struct {
	ID                 int64
	Token              string
	DocumentElementID  int64
	DocumentPropertyID int64
	DefaultValue       string
	Position           int
	Status             string
	CreatedAt          time.Time
	UpdatedAt          time.Time
	Property           DocumentProperty
}
