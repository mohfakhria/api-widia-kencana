package dto

import "encoding/json"

// Jenis pesan yang dikirim server lewat WebSocket.
const (
	DesignMessageSnapshot = "snapshot"
	DesignMessageError    = "error"
)

type DocumentDesignTicketResponse struct {
	DesignTicket DesignTicketPayload `json:"design_ticket"`
}

type DesignTicketPayload struct {
	Ticket    string `json:"ticket"`
	ExpiresIn int64  `json:"expires_in"`
}

func NewDocumentDesignTicketResponse(ticket string, expiresIn int64) DocumentDesignTicketResponse {
	return DocumentDesignTicketResponse{
		DesignTicket: DesignTicketPayload{
			Ticket:    ticket,
			ExpiresIn: expiresIn,
		},
	}
}

// DesignInbound hanya membaca amplop pesan. Isi spesifik tiap jenis perintah
// baru diurai ketika penerapan perubahan dibangun.
type DesignInbound struct {
	Type string `json:"type"`
	Seq  int64  `json:"seq"`
}

type DesignSnapshotMessage struct {
	Type    string          `json:"type"`
	Version int64           `json:"version"`
	Content json.RawMessage `json:"content"`
}

type DesignErrorMessage struct {
	Type    string `json:"type"`
	Seq     int64  `json:"seq,omitempty"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

func NewDesignSnapshotMessage(content json.RawMessage, version int64) ([]byte, error) {
	return json.Marshal(DesignSnapshotMessage{
		Type:    DesignMessageSnapshot,
		Version: version,
		Content: content,
	})
}

func NewDesignErrorMessage(seq int64, code, message string) ([]byte, error) {
	return json.Marshal(DesignErrorMessage{
		Type:    DesignMessageError,
		Seq:     seq,
		Code:    code,
		Message: message,
	})
}
