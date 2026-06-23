package entity

import "time"

type Asset struct {
	ID               int64
	Bucket           string
	ObjectName       string
	OriginalFilename string
	StoredFilename   string
	MimeType         string
	Extension        string
	Size             int64
	ETag             string
	IsPrivate        bool
	UploadedBy       *int64
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type PurchaseOrderAsset struct {
	ID              int64
	PurchaseOrderID int64
	AssetID         int64
	Category        string
	Asset           *Asset
	CreatedAt       time.Time
}
