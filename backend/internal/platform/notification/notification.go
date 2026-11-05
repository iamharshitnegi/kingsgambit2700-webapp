// Package notification persists email work in the same transaction as its event.
package notification

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	platformid "github.com/kingsgambit2700/platform/backend/internal/platform/id"
)

// Message contains a safe event summary, never a private message or lesson body.
// EventKey is stable across retries. Class and trial bindings suppress stale reminders.
type Message struct {
	RecipientID        string     `json:"recipientId,omitempty"`
	Address            string     `json:"address,omitempty"`
	StudentID          string     `json:"studentId,omitempty"`
	Title              string     `json:"title"`
	Section            string     `json:"section"`
	EventKey           string     `json:"eventKey"`
	At                 time.Time  `json:"at"`
	GuardianPermission string     `json:"guardianPermission,omitempty"`
	ClassID            string     `json:"classId,omitempty"`
	ClassRevision      uint64     `json:"classRevision,omitempty"`
	RequireUpcoming    bool       `json:"requireUpcoming,omitempty"`
	TrialID            string     `json:"trialId,omitempty"`
	TrialSlotID        string     `json:"trialSlotId,omitempty"`
	AssignmentID       string     `json:"assignmentId,omitempty"`
	AssignmentDueAt    *time.Time `json:"assignmentDueAt,omitempty"`
}

func EnqueueEmail(ctx context.Context, tx pgx.Tx, message Message) error {
	if tx == nil || (message.RecipientID == "" && message.Address == "") || message.Title == "" || message.EventKey == "" || strings.ContainsAny(message.Title, "\r\n") {
		return errors.New("notification: recipient, safe title, transaction and event key are required")
	}
	if message.At.IsZero() {
		message.At = time.Now().UTC()
	}
	body, err := json.Marshal(message)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO notifications(id,recipient_user_id,recipient_address,template_key,template_version,channel,render_context,deduplication_key,scheduled_at,created_at) VALUES($1::uuid,NULLIF($2,'')::uuid,NULLIF($3,''),'academy_event',1,'email',$4,$5,$6,now()) ON CONFLICT(deduplication_key) DO NOTHING`, platformid.New(), message.RecipientID, message.Address, body, message.EventKey+":"+message.RecipientID+":"+message.Address, message.At)
	return err
}
