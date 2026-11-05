package academy

import (
	"context"
	"encoding/json"
	"github.com/jackc/pgx/v5"
	"slices"
	"time"
)

func (s *Service) UpdateProfile(ctx context.Context, actor string, in ProfileInput) (Profile, error) {
	if !validText(in.DisplayName, 2, 100) {
		return Profile{}, invalid("enter a display name between 2 and 100 characters")
	}
	if _, err := time.LoadLocation(in.TimeZone); err != nil {
		return Profile{}, invalid("choose an IANA time zone")
	}
	return s.repository.UpdateProfile(ctx, actor, in, s.now().UTC())
}
func (s *Service) SavePreferences(ctx context.Context, actor string, in Preferences) (Profile, error) {
	if _, err := time.LoadLocation(in.TimeZone); err != nil {
		return Profile{}, invalid("choose an IANA time zone")
	}
	if len(in.SavedViews) > 20 {
		return Profile{}, invalid("save up to 20 views")
	}
	names := map[string]bool{}
	for _, view := range in.SavedViews {
		if !validText(view.Name, 1, 60) || len(view.Query) > 500 || !slices.Contains([]string{"schedule", "students", "batches", "homework", "games", "library", "operations", "work", "classes", "enrolments", "attendance", "assignments", "sessions", "leads", "trials", "home", "calendar", "board", "progress", "messages", "notifications", "settings"}, view.Section) || names[view.Name] {
			return Profile{}, invalid("saved view name, section or query is invalid")
		}
		names[view.Name] = true
	}
	if in.SavedViews == nil {
		in.SavedViews = []SavedView{}
	}
	return s.repository.SavePreferences(ctx, actor, in, s.now().UTC())
}
func loadProfile(ctx context.Context, q querier, actor string) (Profile, error) {
	out := Profile{SavedViews: []SavedView{}}
	var raw []byte
	err := q.QueryRow(ctx, `SELECT u.display_name,COALESCE(p.time_zone,'UTC'),COALESCE(p.saved_views,'[]'::jsonb) FROM users u LEFT JOIN academy_user_preferences p ON p.user_id=u.id WHERE u.id=$1::uuid AND u.status='active'`, actor).Scan(&out.DisplayName, &out.TimeZone, &raw)
	if err != nil {
		return out, err
	}
	return out, json.Unmarshal(raw, &out.SavedViews)
}
func (r *PostgresRepository) UpdateProfile(ctx context.Context, actor string, in ProfileInput, now time.Time) (Profile, error) {
	var out Profile
	err := r.within(ctx, func(tx pgx.Tx) error {
		if _, err := loadProfile(ctx, tx, actor); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE users SET display_name=$2,updated_at=$3 WHERE id=$1::uuid`, actor, in.DisplayName, now); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO academy_user_preferences(user_id,time_zone,updated_at) VALUES($1::uuid,$2,$3) ON CONFLICT(user_id) DO UPDATE SET time_zone=EXCLUDED.time_zone,updated_at=EXCLUDED.updated_at`, actor, in.TimeZone, now); err != nil {
			return err
		}
		if err := audit(ctx, tx, actor, "academy.profile.updated", actor, now, struct{}{}); err != nil {
			return err
		}
		var err error
		out, err = loadProfile(ctx, tx, actor)
		return err
	})
	return out, err
}
func (r *PostgresRepository) SavePreferences(ctx context.Context, actor string, in Preferences, now time.Time) (Profile, error) {
	var out Profile
	err := r.within(ctx, func(tx pgx.Tx) error {
		if _, err := loadProfile(ctx, tx, actor); err != nil {
			return err
		}
		body, err := json.Marshal(in.SavedViews)
		if err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO academy_user_preferences(user_id,time_zone,saved_views,updated_at) VALUES($1::uuid,$2,$3,$4) ON CONFLICT(user_id) DO UPDATE SET time_zone=EXCLUDED.time_zone,saved_views=EXCLUDED.saved_views,updated_at=EXCLUDED.updated_at`, actor, in.TimeZone, body, now); err != nil {
			return err
		}
		out, err = loadProfile(ctx, tx, actor)
		return err
	})
	return out, err
}
