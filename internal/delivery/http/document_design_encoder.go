package http

import (
	"encoding/json"

	"github.com/mohfakhria/api-widia-kencana/internal/delivery/http/dto"
	"github.com/mohfakhria/api-widia-kencana/internal/domain/design"
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

func (DesignMessageEncoder) EncodeElementCreated(version int64, page string, element design.Element) ([]byte, error) {
	return dto.NewDesignElementCreatedMessage(version, page, element)
}

func (DesignMessageEncoder) EncodeElementUpdated(version int64, element design.Element) ([]byte, error) {
	return dto.NewDesignElementUpdatedMessage(version, element)
}

func (DesignMessageEncoder) EncodeElementDeleted(version int64, id string) ([]byte, error) {
	return dto.NewDesignElementDeletedMessage(version, id)
}

func (DesignMessageEncoder) EncodeElementReordered(version int64, id string, index int) ([]byte, error) {
	return dto.NewDesignElementReorderedMessage(version, id, index)
}

func (DesignMessageEncoder) EncodePageCreated(version int64, id string, index int) ([]byte, error) {
	return dto.NewDesignPageCreatedMessage(version, id, index)
}

func (DesignMessageEncoder) EncodePageUpdated(version int64, id string, hidden, locked bool) ([]byte, error) {
	return dto.NewDesignPageUpdatedMessage(version, id, hidden, locked)
}

func (DesignMessageEncoder) EncodePageDeleted(version int64, id string) ([]byte, error) {
	return dto.NewDesignPageDeletedMessage(version, id)
}

func (DesignMessageEncoder) EncodePageReordered(version int64, id string, index int) ([]byte, error) {
	return dto.NewDesignPageReorderedMessage(version, id, index)
}

func (DesignMessageEncoder) EncodePresence(users []documentdesign.PresenceUser) ([]byte, error) {
	payload := make([]dto.DesignPresenceUser, 0, len(users))
	for _, user := range users {
		payload = append(payload, dto.DesignPresenceUser{ID: user.ID, Name: user.Name})
	}

	return dto.NewDesignPresenceMessage(payload)
}
