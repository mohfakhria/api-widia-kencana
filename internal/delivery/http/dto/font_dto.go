package dto

import (
	"strings"

	"github.com/mohfakhria/api-widia-kencana/internal/usecase/port/input"
)

type FontRegisterRequest struct {
	Family  string   `json:"family"`
	Weights []int    `json:"weights"`
	Styles  []string `json:"styles"`
}

func (r FontRegisterRequest) ToRegisterFontCommand() input.RegisterFontCommand {
	return input.RegisterFontCommand{
		Family:  strings.TrimSpace(r.Family),
		Weights: r.Weights,
		Styles:  r.Styles,
	}
}

// FontFaceResponse melaporkan satu muka huruf.
//
// Reason hanya muncul bila stored bernilai false — bentuk yang membuat balasan
// sukses sebagian tetap terbaca sekali lihat: yang gagal membawa sebabnya, yang
// berhasil tidak membawa field kosong yang mengganggu.
type FontFaceResponse struct {
	Family     string `json:"family"`
	Weight     int    `json:"weight"`
	Style      string `json:"style"`
	ObjectName string `json:"object_name"`
	Size       int64  `json:"size"`
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
