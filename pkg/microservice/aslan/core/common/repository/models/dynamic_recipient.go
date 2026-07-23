package models

import (
	"encoding/json"
	"fmt"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/bsontype"
)

// DynamicRecipients keeps compatibility with the temporary PR schema
// [{"value":"{{.payload.user.email}}","identity_type":"email"}].
type DynamicRecipients []string

var _ bson.ValueUnmarshaler = (*DynamicRecipients)(nil)

func CloneStrings(items []string) []string {
	if items == nil {
		return nil
	}
	resp := make([]string, len(items))
	copy(resp, items)
	return resp
}

func CloneDynamicRecipients(items DynamicRecipients) DynamicRecipients {
	return DynamicRecipients(CloneStrings(items))
}

type legacyDynamicRecipient struct {
	Value        string `bson:"value"         json:"value"`
	IdentityType string `bson:"identity_type" json:"identity_type"`
}

func (r *DynamicRecipients) UnmarshalJSON(data []byte) error {
	var recipients []string
	if err := json.Unmarshal(data, &recipients); err == nil {
		*r = recipients
		return nil
	}

	var legacyRecipients []legacyDynamicRecipient
	if err := json.Unmarshal(data, &legacyRecipients); err != nil {
		return fmt.Errorf("failed to unmarshal dynamic recipients: %w", err)
	}

	*r = legacyDynamicRecipientsToStrings(legacyRecipients)
	return nil
}

func (r *DynamicRecipients) UnmarshalBSONValue(t bsontype.Type, data []byte) error {
	// Mongo's BSON decoder invokes this hook when reading the legacy recipient
	// object array or the current string array from an existing task.
	if t == bsontype.Null || t == bsontype.Undefined {
		*r = nil
		return nil
	}

	var recipients []string
	if err := (bson.RawValue{Type: t, Value: data}).Unmarshal(&recipients); err == nil {
		*r = recipients
		return nil
	}

	var legacyRecipients []legacyDynamicRecipient
	if err := (bson.RawValue{Type: t, Value: data}).Unmarshal(&legacyRecipients); err != nil {
		return fmt.Errorf("failed to unmarshal dynamic recipients: %w", err)
	}

	*r = legacyDynamicRecipientsToStrings(legacyRecipients)
	return nil
}

func legacyDynamicRecipientsToStrings(recipients []legacyDynamicRecipient) []string {
	resp := make([]string, 0, len(recipients))
	for _, recipient := range recipients {
		resp = append(resp, recipient.Value)
	}
	return resp
}
