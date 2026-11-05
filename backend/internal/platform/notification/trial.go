package notification

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// EnqueueTrial queues transactional family and assessor confirmations/reminders.
// extraCoach receives a rescheduling update, but no reminder for the new slot.
func EnqueueTrial(ctx context.Context, tx pgx.Tx, id, title, key string, at time.Time, reminders bool, extraCoach string) error {
	var address, slot, coach, zone string
	var starts time.Time
	if err := tx.QueryRow(ctx, `SELECT b.email::text,b.slot_id::text,COALESCE(s.assessor_user_id::text,''),b.starts_at,b.display_time_zone FROM trial_bookings b JOIN trial_slots s ON s.id=b.slot_id WHERE b.id=$1::uuid`, id).Scan(&address, &slot, &coach, &starts, &zone); err != nil {
		return err
	}
	location, err := time.LoadLocation(zone)
	if err != nil {
		location = time.UTC
	}
	title = title + " · " + starts.In(location).Format("02 Jan 2006 15:04 MST")
	messages := []Message{{Address: address, Title: title, Section: "calendar", EventKey: key, At: at, TrialID: id, TrialSlotID: slot}}
	if coach != "" {
		next := messages[0]
		next.Address = ""
		next.RecipientID = coach
		messages = append(messages, next)
	}
	for _, message := range messages {
		if err = EnqueueEmail(ctx, tx, message); err != nil {
			return err
		}
		if reminders {
			for _, lead := range []time.Duration{24 * time.Hour, time.Hour} {
				if !starts.Add(-lead).After(at) {
					continue
				}
				reminder := message
				reminder.Title = "Your academy trial starts soon · " + starts.In(location).Format("02 Jan 2006 15:04 MST")
				reminder.At = starts.Add(-lead)
				reminder.RequireUpcoming = true
				reminder.EventKey = fmt.Sprintf("trial-reminder:%s:%s:%s", id, slot, lead)
				if err = EnqueueEmail(ctx, tx, reminder); err != nil {
					return err
				}
			}
		}
	}
	if extraCoach != "" && extraCoach != coach {
		message := messages[0]
		message.Address = ""
		message.RecipientID = extraCoach
		message.Title = "A trial booking moved from your slot"
		return EnqueueEmail(ctx, tx, message)
	}
	return nil
}
