package entity

import "time"

type Asset struct {
	ID                 int64
	Token              string
	Bucket             string
	Scope              string
	ObjectName         string
	OriginalFilename   string
	StoredFilename     string
	MimeType           string
	Extension          string
	Size               int64
	ETag               string
	ChecksumSHA256     *string
	Status             string
	UploadMethod       string
	IsPrivate          bool
	UploadedBy         *int64
	PresignedExpiresAt *time.Time
	UploadedAt         *time.Time
	FailedAt           *time.Time
	DeletedAt          *time.Time
	FailureCode        *string
	FailureMessage     *string
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

type PurchaseOrderAsset struct {
	ID              int64
	PurchaseOrderID int64
	AssetID         int64
	Category        string
	Asset           *Asset
	CreatedAt       time.Time
}
