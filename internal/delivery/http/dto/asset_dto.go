package dto

import (
	"strings"
	"time"

	"github.com/mohfakhria/api-widia-kencana/internal/domain/entity"
	"github.com/mohfakhria/api-widia-kencana/internal/usecase/port/input"
)

type AssetUploadRequest struct {
	OriginalFilename string `json:"original_filename"`
	MimeType         string `json:"mime_type"`
	Size             int64  `json:"size"`
	Scope            string `json:"scope"`
}

type AssetListFilterRequest struct {
	Status    string `form:"status"`
	Scope     string `form:"scope"`
	MimeType  string `form:"mime_type"`
	Extension string `form:"extension"`
}

type AssetResponse struct {
	Token              string     `json:"token"`
	Scope              string     `json:"scope"`
	ObjectName         string     `json:"object_name"`
	OriginalFilename   string     `json:"original_filename"`
	StoredFilename     string     `json:"stored_filename"`
	MimeType           string     `json:"mime_type"`
	Extension          string     `json:"extension"`
	Size               int64      `json:"size"`
	ETag               string     `json:"etag"`
	Status             string     `json:"status"`
	UploadMethod       string     `json:"upload_method"`
	IsPrivate          bool       `json:"is_private"`
	PresignedExpiresAt *time.Time `json:"presigned_expires_at,omitempty"`
	UploadedAt         *time.Time `json:"uploaded_at,omitempty"`
	FailedAt           *time.Time `json:"failed_at,omitempty"`
	DeletedAt          *time.Time `json:"deleted_at,omitempty"`
	FailureCode        *string    `json:"failure_code,omitempty"`
	FailureMessage     *string    `json:"failure_message,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

type AssetDataResponse struct {
	Asset AssetResponse `json:"asset"`
}

type AssetListResponse struct {
	Assets []AssetResponse `json:"assets"`
}

type AssetUploadRequestResponse struct {
	Asset     AssetResponse `json:"asset"`
	UploadURL string        `json:"upload_url"`
	ExpiresAt time.Time     `json:"expires_at"`
}

type AssetPresignGetResponse struct {
	Asset     AssetResponse `json:"asset"`
	URL       string        `json:"url"`
	ExpiresIn int64         `json:"expires_in"`
}

func (r AssetUploadRequest) ToRequestAssetUploadCommand(uploadedBy *int64) input.RequestAssetUploadCommand {
	return input.RequestAssetUploadCommand{
		OriginalFilename: r.OriginalFilename,
		MimeType:         r.MimeType,
		Size:             r.Size,
		Scope:            r.Scope,
		UploadedBy:       uploadedBy,
	}
}

func (r AssetListFilterRequest) ToListAssetQuery() input.ListAssetQuery {
	return input.ListAssetQuery{
		Status:    strings.TrimSpace(r.Status),
		Scope:     strings.TrimSpace(r.Scope),
		MimeType:  strings.TrimSpace(r.MimeType),
		Extension: strings.TrimSpace(r.Extension),
	}
}

func NewAssetResponse(asset *entity.Asset) AssetResponse {
	return AssetResponse{
		Token:              asset.Token,
		Scope:              asset.Scope,
		ObjectName:         asset.ObjectName,
		OriginalFilename:   asset.OriginalFilename,
		StoredFilename:     asset.StoredFilename,
		MimeType:           asset.MimeType,
		Extension:          asset.Extension,
		Size:               asset.Size,
		ETag:               asset.ETag,
		Status:             asset.Status,
		UploadMethod:       asset.UploadMethod,
		IsPrivate:          asset.IsPrivate,
		PresignedExpiresAt: asset.PresignedExpiresAt,
		UploadedAt:         asset.UploadedAt,
		FailedAt:           asset.FailedAt,
		DeletedAt:          asset.DeletedAt,
		FailureCode:        asset.FailureCode,
		FailureMessage:     asset.FailureMessage,
		CreatedAt:          asset.CreatedAt,
		UpdatedAt:          asset.UpdatedAt,
	}
}

func NewAssetDataResponse(asset *entity.Asset) AssetDataResponse {
	return AssetDataResponse{Asset: NewAssetResponse(asset)}
}

func NewAssetListResponse(assets []entity.Asset) AssetListResponse {
	response := AssetListResponse{
		Assets: make([]AssetResponse, 0, len(assets)),
	}
	for _, asset := range assets {
		response.Assets = append(response.Assets, NewAssetResponse(&asset))
	}

	return response
}

func NewAssetUploadRequestResponse(result *input.AssetUploadRequestResult) AssetUploadRequestResponse {
	return AssetUploadRequestResponse{
		Asset:     NewAssetResponse(result.Asset),
		UploadURL: result.UploadURL,
		ExpiresAt: result.ExpiresAt,
	}
}

func NewAssetPresignGetResponse(result *input.AssetPresignGetResult) AssetPresignGetResponse {
	return AssetPresignGetResponse{
		Asset:     NewAssetResponse(result.Asset),
		URL:       result.URL,
		ExpiresIn: result.ExpiresIn,
	}
}
