package http

import (
	"encoding/json"
	"strconv"

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
			ID:   strconv.FormatInt(cursor.UserID, 10),
			Page: cursor.Page,
			X:    cursor.X,
			Y:    cursor.Y,
		})
	}

	return dto.NewDesignCursorMessage(entries)
}

func (DesignMessageEncoder) EncodeSelections(selections []documentdesign.Selection) ([]byte, error) {
	entries := make([]dto.DesignSelectionEntry, 0, len(selections))
	for _, selection := range selections {
		entries = append(entries, dto.DesignSelectionEntry{
			ID:  strconv.FormatInt(selection.UserID, 10),
			IDs: selection.ElementIDs,
		})
	}

	return dto.NewDesignSelectionMessage(entries)
}

func (DesignMessageEncoder) EncodeElementCreated(version int64, origin documentdesign.Origin, page string, element design.Element) ([]byte, error) {
	return dto.NewDesignElementCreatedMessage(version, string(origin), page, element)
}

func (DesignMessageEncoder) EncodeElementUpdated(version int64, origin documentdesign.Origin, element design.Element) ([]byte, error) {
	return dto.NewDesignElementUpdatedMessage(version, string(origin), element)
}

func (DesignMessageEncoder) EncodeElementDeleted(version int64, origin documentdesign.Origin, id string) ([]byte, error) {
	return dto.NewDesignElementDeletedMessage(version, string(origin), id)
}

func (DesignMessageEncoder) EncodeElementReordered(version int64, origin documentdesign.Origin, id string, index int) ([]byte, error) {
	return dto.NewDesignElementReorderedMessage(version, string(origin), id, index)
}

func (DesignMessageEncoder) EncodePageCreated(version int64, origin documentdesign.Origin, id string, index int) ([]byte, error) {
	return dto.NewDesignPageCreatedMessage(version, string(origin), id, index)
}

func (DesignMessageEncoder) EncodePageUpdated(version int64, origin documentdesign.Origin, id string, props design.PageProps) ([]byte, error) {
	return dto.NewDesignPageUpdatedMessage(version, string(origin), id, props)
}

func (DesignMessageEncoder) EncodePageDeleted(version int64, origin documentdesign.Origin, id string) ([]byte, error) {
	return dto.NewDesignPageDeletedMessage(version, string(origin), id)
}

func (DesignMessageEncoder) EncodePageReordered(version int64, origin documentdesign.Origin, id string, index int) ([]byte, error) {
	return dto.NewDesignPageReorderedMessage(version, string(origin), id, index)
}

func (DesignMessageEncoder) EncodePresence(users []documentdesign.PresenceUser) ([]byte, error) {
	payload := make([]dto.DesignPresenceUser, 0, len(users))
	for _, user := range users {
		payload = append(payload, dto.DesignPresenceUser{ID: strconv.FormatInt(user.ID, 10), Name: user.Name})
	}

	return dto.NewDesignPresenceMessage(payload)
}
