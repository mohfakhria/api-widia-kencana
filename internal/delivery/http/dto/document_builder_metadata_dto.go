package dto

import "github.com/mohfakhria/api-widia-kencana/internal/domain/entity"

type DocumentBuilderMetadataDataResponse struct {
	DocumentBuilder DocumentBuilderMetadataResponse `json:"document_builder"`
}

type DocumentBuilderMetadataResponse struct {
	Papers   []DocumentPaperResponse   `json:"papers"`
	Elements []DocumentElementResponse `json:"elements"`
}

type DocumentPaperResponse struct {
	Token          string  `json:"token"`
	Name           string  `json:"name"`
	MediaType      string  `json:"media_type"`
	Width          float64 `json:"width"`
	Height         float64 `json:"height"`
	Unit           string  `json:"unit"`
	AllowPortrait  bool    `json:"allow_portrait"`
	AllowLandscape bool    `json:"allow_landscape"`
	Status         string  `json:"status"`
}

type DocumentElementResponse struct {
	Token        string                            `json:"token"`
	Code         string                            `json:"code"`
	Name         string                            `json:"name"`
	RendererType string                            `json:"renderer_type"`
	RendererTag  string                            `json:"renderer_tag"`
	ContentType  string                            `json:"content_type"`
	IsContainer  bool                              `json:"is_container"`
	Status       string                            `json:"status"`
	Properties   []DocumentElementPropertyResponse `json:"properties"`
}

type DocumentElementPropertyResponse struct {
	Token         string                           `json:"token"`
	PropertyToken string                           `json:"property_token"`
	Code          string                           `json:"code"`
	Name          string                           `json:"name"`
	DataType      string                           `json:"data_type"`
	InputType     string                           `json:"input_type"`
	DefaultValue  string                           `json:"default_value"`
	Unit          string                           `json:"unit"`
	Position      int                              `json:"position"`
	Options       []DocumentPropertyOptionResponse `json:"options"`
}

type DocumentPropertyOptionResponse struct {
	Token    string `json:"token"`
	Value    string `json:"value"`
	Label    string `json:"label"`
	Position int    `json:"position"`
}

func NewDocumentBuilderMetadataDataResponse(
	metadata *entity.DocumentBuilderMetadata,
) DocumentBuilderMetadataDataResponse {
	return DocumentBuilderMetadataDataResponse{
		DocumentBuilder: NewDocumentBuilderMetadataResponse(metadata),
	}
}

func NewDocumentBuilderMetadataResponse(
	metadata *entity.DocumentBuilderMetadata,
) DocumentBuilderMetadataResponse {
	response := DocumentBuilderMetadataResponse{
		Papers:   make([]DocumentPaperResponse, 0, len(metadata.Papers)),
		Elements: make([]DocumentElementResponse, 0, len(metadata.Elements)),
	}

	for _, paper := range metadata.Papers {
		response.Papers = append(response.Papers, NewDocumentPaperResponse(paper))
	}
	for _, element := range metadata.Elements {
		response.Elements = append(response.Elements, NewDocumentElementResponse(element))
	}

	return response
}

func NewDocumentPaperResponse(paper entity.DocumentPaper) DocumentPaperResponse {
	return DocumentPaperResponse{
		Token:          paper.Token,
		Name:           paper.Name,
		MediaType:      paper.MediaType,
		Width:          paper.Width,
		Height:         paper.Height,
		Unit:           paper.Unit,
		AllowPortrait:  paper.AllowPortrait,
		AllowLandscape: paper.AllowLandscape,
		Status:         paper.Status,
	}
}

func NewDocumentElementResponse(element entity.DocumentElement) DocumentElementResponse {
	response := DocumentElementResponse{
		Token:        element.Token,
		Code:         element.Code,
		Name:         element.Name,
		RendererType: element.RendererType,
		RendererTag:  element.RendererTag,
		ContentType:  element.ContentType,
		IsContainer:  element.IsContainer,
		Status:       element.Status,
		Properties:   make([]DocumentElementPropertyResponse, 0, len(element.Properties)),
	}

	for _, property := range element.Properties {
		response.Properties = append(response.Properties, NewDocumentElementPropertyResponse(property))
	}

	return response
}

func NewDocumentElementPropertyResponse(
	elementProperty entity.DocumentElementProperty,
) DocumentElementPropertyResponse {
	property := elementProperty.Property
	response := DocumentElementPropertyResponse{
		Token:         elementProperty.Token,
		PropertyToken: property.Token,
		Code:          property.Code,
		Name:          property.Name,
		DataType:      property.DataType,
		InputType:     property.InputType,
		DefaultValue:  elementProperty.DefaultValue,
		Unit:          property.Unit,
		Position:      elementProperty.Position,
		Options:       make([]DocumentPropertyOptionResponse, 0, len(property.Options)),
	}

	for _, option := range property.Options {
		response.Options = append(response.Options, NewDocumentPropertyOptionResponse(option))
	}

	return response
}

func NewDocumentPropertyOptionResponse(option entity.DocumentPropertyOption) DocumentPropertyOptionResponse {
	return DocumentPropertyOptionResponse{
		Token:    option.Token,
		Value:    option.Value,
		Label:    option.Label,
		Position: option.Position,
	}
}
