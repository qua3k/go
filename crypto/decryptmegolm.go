// Copyright (c) 2020 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package crypto

import (
	"encoding/json"

	"github.com/pkg/errors"

	"maunium.net/go/mautrix/event"
)

var (
	IncorrectEncryptedContentType = errors.New("event content is not instance of *event.EncryptedEventContent")
	NoSessionFound                = errors.New("failed to decrypt megolm event: no session with given ID found")
	DuplicateMessageIndex         = errors.New("duplicate message index")
)

type MegolmEvent struct {
	Type    event.Type    `json:"type"`
	Content event.Content `json:"content"`
}

func (mach *OlmMachine) DecryptMegolmEvent(evt *event.Event) (*event.Event, error) {
	content, ok := evt.Content.Parsed.(*event.EncryptedEventContent)
	if !ok {
		return nil, IncorrectEncryptedContentType
	}
	sess, err := mach.store.GetGroupSession(evt.RoomID, content.SenderKey, content.SessionID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get group session")
	} else if sess == nil {
		mach.checkIfWedged(evt)
		return nil, NoSessionFound
	}
	plaintext, messageIndex, err := sess.Decrypt(content.MegolmCiphertext)
	if err != nil {
		return nil, errors.Wrap(err, "failed to decrypt megolm event")
	} else if !mach.store.ValidateMessageIndex(content.SenderKey, content.SessionID, evt.ID, messageIndex, evt.Timestamp) {
		return nil, DuplicateMessageIndex
	}
	// TODO marking events as verified can probably be done here

	megolmEvt := &MegolmEvent{}
	err = json.Unmarshal(plaintext, &megolmEvt)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse megolm payload")
	}
	err = megolmEvt.Content.ParseRaw(megolmEvt.Type)
	if err != nil && !event.IsUnsupportedContentType(err) {
		return nil, errors.Wrap(err, "failed to parse content of megolm payload event")
	}
	return &event.Event{
		Sender:    evt.Sender,
		Type:      megolmEvt.Type,
		Timestamp: evt.Timestamp,
		ID:        evt.ID,
		RoomID:    evt.RoomID,
		Content:   megolmEvt.Content,
		Unsigned:  evt.Unsigned,
	}, nil
}
