package documentdesign

import (
	"cmp"
	"slices"
	"strings"
)

// presentUsers menyusun daftar orang yang sedang membuka dokumen, tanpa
// pengulangan.
//
// Urutannya dibuat pasti — menurut nama, lalu id sebagai pemutus seri — karena
// iterasi peta di Go berurutan acak. Tanpa pengurutan, tumpukan avatar di
// frontend akan berganti susunan setiap kali ada yang datang atau pergi.
func (r *Room) presentUsers() []PresenceUser {
	seen := make(map[int64]struct{}, len(r.members))
	users := make([]PresenceUser, 0, len(r.members))

	for _, member := range r.members {
		if _, exists := seen[member.UserID]; exists {
			continue
		}
		seen[member.UserID] = struct{}{}
		users = append(users, PresenceUser{ID: member.UserID, Name: member.UserName})
	}

	slices.SortFunc(users, func(a, b PresenceUser) int {
		if order := strings.Compare(a.Name, b.Name); order != 0 {
			return order
		}

		return cmp.Compare(a.ID, b.ID)
	})

	return users
}

// broadcastPresence memberi tahu seluruh penghuni siapa saja yang sedang membuka
// dokumen ini.
//
// Kegagalan menyusun payload hanya dicatat, tidak menandai room rusak. Daftar
// kehadiran adalah hiasan di sekitar pekerjaan yang sebenarnya; menghentikan
// penyuntingan karena ia gagal disusun jauh lebih merugikan daripada tumpukan
// avatar yang tidak diperbarui.
func (r *Room) broadcastPresence() {
	payload, err := r.encodePresence()
	if err != nil {
		return
	}

	for subscriber := range r.members {
		subscriber.Send(payload)
	}
}

func (r *Room) sendPresence(subscriber Subscriber) {
	payload, err := r.encodePresence()
	if err != nil {
		return
	}

	subscriber.Send(payload)
}

func (r *Room) encodePresence() ([]byte, error) {
	payload, err := r.encoder.EncodePresence(r.presentUsers())
	if err != nil {
		r.logger.Error("encode document design presence", "document", r.token, "error", err)
		return nil, err
	}

	return payload, nil
}
