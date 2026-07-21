package entity

const (
	DocumentOrientationPortrait = "portrait"
	DocumentUnitAuto            = "auto"
	DocumentUnitPX              = "px"
)

func DefaultDocumentSettings() map[string]any {
	return map[string]any{
		"page": map[string]any{
			"orientation": DocumentOrientationPortrait,
			"margin": map[string]any{
				"top":    24,
				"right":  24,
				"bottom": 24,
				"left":   24,
				"unit":   DocumentUnitPX,
			},
		},
		"regions": map[string]any{
			"header": DefaultDocumentRegionSettings(96, DocumentUnitPX),
			"body":   DefaultDocumentRegionSettings(nil, DocumentUnitAuto),
			"footer": DefaultDocumentRegionSettings(72, DocumentUnitPX),
		},
	}
}

func DefaultDocumentRegionSettings(height any, unit string) map[string]any {
	return map[string]any{
		"height":    height,
		"unit":      unit,
		"watermark": nil,
	}
}
