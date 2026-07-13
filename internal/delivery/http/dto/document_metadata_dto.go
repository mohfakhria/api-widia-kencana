package dto

import "github.com/mohfakhria/api-widia-kencana/internal/domain/entity"

type DocumentPapersDataResponse struct {
	Papers []DocumentPaperResponse `json:"papers"`
}

type DocumentElementsDataResponse struct {
	Elements []DocumentElementResponse `json:"elements"`
}

type DocumentPropertiesDataResponse struct {
	Properties []DocumentPropertyResponse `json:"properties"`
}

type DocumentPropertyOptionsDataResponse struct {
	PropertyOptions []DocumentPropertyOptionDetailResponse `json:"property_options"`
}

type DocumentElementPropertiesDataResponse struct {
	ElementProperties []DocumentElementPropertyDetailResponse `json:"element_properties"`
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

type DocumentPropertyResponse struct {
	Token        string `json:"token"`
	Code         string `json:"code"`
	Name         string `json:"name"`
	DataType     string `json:"data_type"`
	InputType    string `json:"input_type"`
	DefaultValue string `json:"default_value"`
	Unit         string `json:"unit"`
	Status       string `json:"status"`
}

type DocumentPropertyOptionResponse struct {
	Token    string `json:"token"`
	Value    string `json:"value"`
	Label    string `json:"label"`
	Position int    `json:"position"`
}

type DocumentPropertyOptionDetailResponse struct {
	Token         string `json:"token"`
	PropertyToken string `json:"property_token"`
	PropertyCode  string `json:"property_code"`
	Value         string `json:"value"`
	Label         string `json:"label"`
	Position      int    `json:"position"`
	Status        string `json:"status"`
}

type DocumentElementPropertyDetailResponse struct {
	Token         string                   `json:"token"`
	ElementToken  string                   `json:"element_token"`
	ElementCode   string                   `json:"element_code"`
	PropertyToken string                   `json:"property_token"`
	PropertyCode  string                   `json:"property_code"`
	DefaultValue  string                   `json:"default_value"`
	Position      int                      `json:"position"`
	Status        string                   `json:"status"`
	Property      DocumentPropertyResponse `json:"property"`
}

func NewDocumentPapersDataResponse(papers []entity.DocumentPaper) DocumentPapersDataResponse {
	response := DocumentPapersDataResponse{
		Papers: make([]DocumentPaperResponse, 0, len(papers)),
	}
	for _, paper := range papers {
		response.Papers = append(response.Papers, NewDocumentPaperResponse(paper))
	}

	return response
}

func NewDocumentElementsDataResponse(elements []entity.DocumentElement) DocumentElementsDataResponse {
	response := DocumentElementsDataResponse{
		Elements: make([]DocumentElementResponse, 0, len(elements)),
	}
	for _, element := range elements {
		response.Elements = append(response.Elements, NewDocumentElementResponse(element))
	}

	return response
}

func NewDocumentPropertiesDataResponse(properties []entity.DocumentProperty) DocumentPropertiesDataResponse {
	response := DocumentPropertiesDataResponse{
		Properties: make([]DocumentPropertyResponse, 0, len(properties)),
	}
	for _, property := range properties {
		response.Properties = append(response.Properties, NewDocumentPropertyResponse(property))
	}

	return response
}

func NewDocumentPropertyOptionsDataResponse(
	options []entity.DocumentPropertyOption,
) DocumentPropertyOptionsDataResponse {
	response := DocumentPropertyOptionsDataResponse{
		PropertyOptions: make([]DocumentPropertyOptionDetailResponse, 0, len(options)),
	}
	for _, option := range options {
		response.PropertyOptions = append(response.PropertyOptions, NewDocumentPropertyOptionDetailResponse(option))
	}

	return response
}

func NewDocumentElementPropertiesDataResponse(
	elementProperties []entity.DocumentElementProperty,
) DocumentElementPropertiesDataResponse {
	response := DocumentElementPropertiesDataResponse{
		ElementProperties: make([]DocumentElementPropertyDetailResponse, 0, len(elementProperties)),
	}
	for _, elementProperty := range elementProperties {
		response.ElementProperties = append(
			response.ElementProperties,
			NewDocumentElementPropertyDetailResponse(elementProperty),
		)
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

func NewDocumentPropertyResponse(property entity.DocumentProperty) DocumentPropertyResponse {
	return DocumentPropertyResponse{
		Token:        property.Token,
		Code:         property.Code,
		Name:         property.Name,
		DataType:     property.DataType,
		InputType:    property.InputType,
		DefaultValue: property.DefaultValue,
		Unit:         property.Unit,
		Status:       property.Status,
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

func NewDocumentPropertyOptionDetailResponse(
	option entity.DocumentPropertyOption,
) DocumentPropertyOptionDetailResponse {
	return DocumentPropertyOptionDetailResponse{
		Token:         option.Token,
		PropertyToken: option.PropertyToken,
		PropertyCode:  option.PropertyCode,
		Value:         option.Value,
		Label:         option.Label,
		Position:      option.Position,
		Status:        option.Status,
	}
}

func NewDocumentElementPropertyDetailResponse(
	elementProperty entity.DocumentElementProperty,
) DocumentElementPropertyDetailResponse {
	property := elementProperty.Property
	return DocumentElementPropertyDetailResponse{
		Token:         elementProperty.Token,
		ElementToken:  elementProperty.ElementToken,
		ElementCode:   elementProperty.ElementCode,
		PropertyToken: property.Token,
		PropertyCode:  property.Code,
		DefaultValue:  elementProperty.DefaultValue,
		Position:      elementProperty.Position,
		Status:        elementProperty.Status,
		Property:      NewDocumentPropertyResponse(property),
	}
}

func NewDocumentPropertyOptionResponse(option entity.DocumentPropertyOption) DocumentPropertyOptionResponse {
	return DocumentPropertyOptionResponse{
		Token:    option.Token,
		Value:    option.Value,
		Label:    option.Label,
		Position: option.Position,
	}
}
