package dto

import (
	"github.com/mohfakhria/api-widia-kencana/internal/usecase/port/input"
)

// FontFaceResponse melaporkan satu muka huruf.
//
// Reason hanya muncul bila stored bernilai false — bentuk yang membuat balasan
// sukses sebagian tetap terbaca sekali lihat: yang gagal membawa sebabnya, yang
// berhasil tidak membawa field kosong yang mengganggu.
type FontFaceResponse struct {
	// Entry selalu ada; sisanya menyusul sejauh berkasnya terbaca. Baris yang
	// dilewati karena bukan font hanya membawa entry dan reason.
	Entry      string `json:"entry"`
	Family     string `json:"family,omitempty"`
	Weight     int    `json:"weight,omitempty"`
	Style      string `json:"style,omitempty"`
	ObjectName string `json:"object_name,omitempty"`
	Size       int64  `json:"size,omitempty"`
	Stored     bool   `json:"stored"`
	Reason     string `json:"reason,omitempty"`
}

type FontRegisterResponse struct {
	Faces  []FontFaceResponse `json:"faces"`
	Stored int                `json:"stored"`
	Failed int                `json:"failed"`
}

func NewFontRegisterResponse(faces []input.FontFaceResult) FontRegisterResponse {
	out := FontRegisterResponse{Faces: make([]FontFaceResponse, 0, len(faces))}
	for _, face := range faces {
		out.Faces = append(out.Faces, FontFaceResponse{
			Entry:      face.Entry,
			Family:     face.Family,
			Weight:     face.Weight,
			Style:      face.Style,
			ObjectName: face.ObjectName,
			Size:       face.Size,
			Stored:     face.Stored,
			Reason:     face.Reason,
		})
		if face.Stored {
			out.Stored++
			continue
		}
		out.Failed++
	}

	return out
}

type FontFaceListResponse struct {
	Weight      int    `json:"weight"`
	Style       string `json:"style"`
	Size        int64  `json:"size"`
	ContentPath string `json:"content_path"`
}

type FontFamilyListResponse struct {
	Family string                 `json:"family"`
	Faces  []FontFaceListResponse `json:"faces"`
}

type FontListResponse struct {
	Families []FontFamilyListResponse `json:"families"`
}

func NewFontListResponse(families []input.FontFamilyListing) FontListResponse {
	// Dibuat kosong, bukan dibiarkan nil: nil menjadi null di JSON, dan klien
	// yang melakukan iterasi atasnya gagal justru saat belum ada font terpasang
	// — keadaan yang paling wajar di hari pertama.
	out := FontListResponse{Families: make([]FontFamilyListResponse, 0, len(families))}
	for _, family := range families {
		faces := make([]FontFaceListResponse, 0, len(family.Faces))
		for _, face := range family.Faces {
			faces = append(faces, FontFaceListResponse{
				Weight:      face.Weight,
				Style:       face.Style,
				Size:        face.Size,
				ContentPath: face.ContentPath,
			})
		}
		out.Families = append(out.Families, FontFamilyListResponse{Family: family.Family, Faces: faces})
	}

	return out
}
