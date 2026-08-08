package http

import (
	"context"
	"encoding/json"

	"github.com/mohfakhria/api-widia-kencana/internal/delivery/http/dto"
	"github.com/mohfakhria/api-widia-kencana/internal/domain/design"
)

// Pengubahan pesan penyuntingan menjadi tindakan.
//
// Berbeda dari cursor.move yang dijatuhkan diam-diam, muatan penyuntingan yang
// cacat DIBALAS. Suntingan datang dari tindakan sadar pengguna dan sudah
// tergambar optimistis di layarnya; membiarkannya tanpa kabar berarti layar dan
// dokumen berbeda selamanya tanpa ada yang tahu. Lajunya pun jauh lebih rendah
// daripada kursor, jadi membalas tidak membanjiri siapa pun.

func (h *DocumentDesignHandler) createElement(ctx context.Context, documentToken string, payload []byte, subscriber *designSubscriber) {
	var message dto.DesignElementCreate
	if err := json.Unmarshal(payload, &message); err != nil {
		subscriber.sendError("malformed_message", "element.create payload is not valid")
		return
	}
	if message.Page == "" {
		subscriber.sendError("malformed_message", "element.create requires a page")
		return
	}

	element, err := h.decodeElement(message.Element, subscriber)
	if err != nil {
		return
	}

	// Satu-satunya penyuntingan yang menunggu jawaban orchestrator, karena satu-
	// satunya yang dapat ditolak di sana: halaman tidak ada, atau id sudah dipakai.
	if err := h.service.CreateElement(ctx, documentToken, subscriber, message.Page, element); err != nil {
		h.logger.Warn("create document design element",
			"document", documentToken, "element", element.ID, "error", err)
		// Pesan dari lapisan usecase memang disusun sebagai frasa yang aman
		// disampaikan ke klien; detail internalnya berhenti di log.
		subscriber.sendError("element_rejected", err.Error())
	}
}

func (h *DocumentDesignHandler) updateElement(documentToken string, payload []byte, subscriber *designSubscriber) {
	var message dto.DesignElementUpdate
	if err := json.Unmarshal(payload, &message); err != nil {
		subscriber.sendError("malformed_message", "element.update payload is not valid")
		return
	}

	element, err := h.decodeElement(message.Element, subscriber)
	if err != nil {
		return
	}

	h.service.UpdateElement(documentToken, subscriber, element)
}

func (h *DocumentDesignHandler) deleteElement(documentToken string, payload []byte, subscriber *designSubscriber) {
	var message dto.DesignElementDelete
	if err := json.Unmarshal(payload, &message); err != nil {
		subscriber.sendError("malformed_message", "element.delete payload is not valid")
		return
	}
	if message.ID == "" {
		subscriber.sendError("malformed_message", "element.delete requires an id")
		return
	}

	h.service.DeleteElement(documentToken, subscriber, message.ID)
}

func (h *DocumentDesignHandler) reorderElement(documentToken string, payload []byte, subscriber *designSubscriber) {
	var message dto.DesignElementReorder
	if err := json.Unmarshal(payload, &message); err != nil {
		subscriber.sendError("malformed_message", "element.reorder payload is not valid")
		return
	}
	if message.ID == "" {
		subscriber.sendError("malformed_message", "element.reorder requires an id")
		return
	}

	// Index di luar batas sengaja TIDAK ditolak di sini. Batasnya milik
	// orchestrator dan berubah setiap kali ada yang menambah atau menghapus
	// elemen; memeriksanya di lapisan ini berarti memutuskan berdasarkan keadaan
	// yang tidak kita pegang.
	h.service.ReorderElement(documentToken, subscriber, message.ID, message.Index)
}

// decodeElement mengurai elemen lewat domain, bukan di sini.
//
// Bentuk elemen beserta aturannya dimiliki domain/design, yang menolak properti
// tak dikenal lewat DisallowUnknownFields. Menguraikannya di lapisan delivery
// akan menyalin aturan itu ke tempat yang tidak berwenang, dan salinan seperti
// itu cepat atau lambat melenceng dari aslinya.
//
// Ditolak di sini, sebelum menyentuh orchestrator, supaya muatan cacat tidak
// pernah membebani goroutine yang melayani seluruh penyunting dokumen ini.
func (h *DocumentDesignHandler) decodeElement(raw json.RawMessage, subscriber *designSubscriber) (design.Element, error) {
	element, err := design.DecodeElement(raw)
	if err != nil {
		subscriber.sendError("element_rejected", err.Error())
		return design.Element{}, err
	}

	return element, nil
}
