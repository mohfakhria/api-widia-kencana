package usecase

import (
	"path/filepath"
	"strings"
	"unicode"

	"github.com/google/uuid"
)

func buildStoredAssetFilename(originalFilename string) string {
	return buildStoredAssetFilenameFromBase(filepath.Base(originalFilename))
}

func buildStoredAssetFilenameFromBase(filename string) string {
	filename = sanitizeAssetFilename(filename)
	if filename == "" || filename == "." {
		filename = "asset"
	}

	return uuid.NewString() + "-" + filename
}

func sanitizeAssetFilename(filename string) string {
	var builder strings.Builder
	for _, r := range filename {
		switch {
		case unicode.IsLetter(r), unicode.IsDigit(r):
			builder.WriteRune(r)
		case r == '.', r == '-', r == '_':
			builder.WriteRune(r)
		case unicode.IsSpace(r):
			builder.WriteRune('-')
		}
	}

	return builder.String()
}
