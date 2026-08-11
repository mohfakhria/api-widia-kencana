package http

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/mohfakhria/api-widia-kencana/internal/delivery/http/dto"
	"github.com/mohfakhria/api-widia-kencana/internal/domain/design"
)

// Pengubahan pesan penyuntingan menjadi tindakan — elemen dan halaman.
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
		if !isEditRejection(err) {
			return
		}

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

func (h *DocumentDesignHandler) createPage(ctx context.Context, documentToken string, payload []byte, subscriber *designSubscriber) {
	var message dto.DesignPageCreate
	if err := json.Unmarshal(payload, &message); err != nil {
		subscriber.sendError("malformed_message", "page.create payload is not valid")
		return
	}
	if message.ID == "" {
		subscriber.sendError("malformed_message", "page.create requires an id")
		return
	}

	// Index diteruskan apa adanya, termasuk ketika nil. Nil berarti "di akhir",
	// dan lapisan ini tidak tahu berapa jumlah halaman dokumen itu — hanya
	// orchestrator yang tahu, dan hanya ia yang boleh memutuskannya.
	if err := h.service.CreatePage(ctx, documentToken, subscriber, message.ID, message.Index); err != nil {
		if !isEditRejection(err) {
			return
		}

		h.logger.Warn("create document design page",
			"document", documentToken, "page", message.ID, "error", err)
		subscriber.sendError("page_rejected", err.Error())
	}
}

func (h *DocumentDesignHandler) updatePage(documentToken string, payload []byte, subscriber *designSubscriber) {
	var message dto.DesignPageUpdate
	if err := json.Unmarshal(payload, &message); err != nil {
		subscriber.sendError("malformed_message", "page.update payload is not valid")
		return
	}
	if message.ID == "" {
		subscriber.sendError("malformed_message", "page.update requires an id")
		return
	}

	// KEEMPAT field WAJIB ada. Yang hilang tidak diperlakukan sebagai nilai kosong:
	// klien yang lupa menyertakan locked akan membuka kunci halaman diam-diam, dan
	// yang lupa menyertakan title akan menghapus judulnya — keduanya tersiar ke
	// semua orang sebagai perubahan yang sah. Ditolak di sini, sebelum menyentuh
	// orchestrator.
	if message.Title == nil || message.Background == nil || message.Hidden == nil || message.Locked == nil {
		subscriber.sendError("malformed_message", "page.update requires title, background, hidden, and locked")
		return
	}

	// Warna diperiksa di sini juga, sejalan dengan penolakan di atas: page.update
	// sengaja tidak membalas pengirimnya, sehingga apa pun yang dapat ditolak
	// harus ditahan sebelum menyentuh orchestrator. Latar yang cacat yang lolos
	// akan tercetak hitam tanpa ada yang pernah memintanya.
	if !design.IsColor(*message.Background) {
		subscriber.sendError("malformed_message", "page.update background must be #rgb or #rrggbb")
		return
	}

	h.service.UpdatePage(documentToken, subscriber, message.ID, design.PageProps{
		Title:      *message.Title,
		Background: *message.Background,
		Hidden:     *message.Hidden,
		Locked:     *message.Locked,
	})
}

func (h *DocumentDesignHandler) deletePage(ctx context.Context, documentToken string, payload []byte, subscriber *designSubscriber) {
	var message dto.DesignPageDelete
	if err := json.Unmarshal(payload, &message); err != nil {
		subscriber.sendError("malformed_message", "page.delete payload is not valid")
		return
	}
	if message.ID == "" {
		subscriber.sendError("malformed_message", "page.delete requires an id")
		return
	}

	// Menunggu hasil, berbeda dari element.delete. Halaman terakhir tidak boleh
	// dibuang, dan penolakan itu wajib sampai — dokumen tanpa halaman akan ditimpa
	// panduan bawaan ketika orang berikutnya membukanya.
	if err := h.service.DeletePage(ctx, documentToken, subscriber, message.ID); err != nil {
		if !isEditRejection(err) {
			return
		}

		h.logger.Warn("delete document design page",
			"document", documentToken, "page", message.ID, "error", err)
		subscriber.sendError("page_rejected", err.Error())
	}
}

func (h *DocumentDesignHandler) reorderPage(documentToken string, payload []byte, subscriber *designSubscriber) {
	var message dto.DesignPageReorder
	if err := json.Unmarshal(payload, &message); err != nil {
		subscriber.sendError("malformed_message", "page.reorder payload is not valid")
		return
	}
	if message.ID == "" {
		subscriber.sendError("malformed_message", "page.reorder requires an id")
		return
	}

	h.service.ReorderPage(documentToken, subscriber, message.ID, message.Index)
}

// isEditRejection memisahkan penolakan yang sesungguhnya dari koneksi yang
// sedang berakhir.
//
// Context yang dibatalkan BUKAN penolakan. Kejadiannya bisa saja sudah masuk
// inbox orchestrator dan akan tetap diterapkan, sehingga menyampaikannya sebagai
// penolakan menyuruh klien membatalkan perubahan yang justru mendarat. Teks
// error-nya pun bukan frasa yang dimaksudkan untuk dibaca siapa pun di luar log.
//
// Diam adalah jawaban yang benar di sini: pembatalan hanya terjadi ketika koneksi
// itu sendiri sedang ditutup, dan tidak ada lagi yang akan membaca balasannya.
func isEditRejection(err error) bool {
	return !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded)
}
