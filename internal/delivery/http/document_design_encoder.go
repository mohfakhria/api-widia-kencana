package http

import (
	"encoding/json"

	"github.com/mohfakhria/api-widia-kencana/internal/delivery/http/dto"
	"github.com/mohfakhria/api-widia-kencana/internal/usecase/documentdesign"
)

// Bentuk kawat pesan yang dikirim room ke klien. Implementasi port
// documentdesign.MessageEncoder.

// DesignMessageEncoder menyusun payload yang dikirim room ke klien.
//
// Implementasi dari port documentdesign.MessageEncoder. Bentuk kawatnya milik
// lapisan ini; room hanya menyerahkan isi dan nomor versinya.
type DesignMessageEncoder struct{}

func (DesignMessageEncoder) EncodeSnapshot(content json.RawMessage, version int64, page documentdesign.PageSize) ([]byte, error) {
	return dto.NewDesignSnapshotMessage(content, version, page.Width, page.Height)
}

func (DesignMessageEncoder) EncodeCursors(cursors []documentdesign.Cursor) ([]byte, error) {
	entries := make([]dto.DesignCursorEntry, 0, len(cursors))
	for _, cursor := range cursors {
		entries = append(entries, dto.DesignCursorEntry{
			ID:   cursor.UserID,
			Page: cursor.Page,
			X:    cursor.X,
			Y:    cursor.Y,
		})
	}

	return dto.NewDesignCursorMessage(entries)
}

func (DesignMessageEncoder) EncodePresence(users []documentdesign.PresenceUser) ([]byte, error) {
	payload := make([]dto.DesignPresenceUser, 0, len(users))
	for _, user := range users {
		payload = append(payload, dto.DesignPresenceUser{ID: user.ID, Name: user.Name})
	}

	return dto.NewDesignPresenceMessage(payload)
}
