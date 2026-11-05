//go:build integration

package repository

import (
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/kingsgambit2700/platform/backend/internal/auth/model"
	platformid "github.com/kingsgambit2700/platform/backend/internal/platform/id"
	"github.com/kingsgambit2700/platform/backend/internal/platform/integrationtest"
)

func TestPostgresRememberedDeviceLifecycle(t *testing.T) {
	databaseURL := os.Getenv("KG_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("KG_TEST_DATABASE_URL not set")
	}
	pool := integrationtest.OpenMigratedSchema(t, databaseURL)
	repo := NewPostgres(pool)
	now := time.Now().UTC().Truncate(time.Microsecond)
	magic := postgresMagicLink(integrationDigest("device-login"), "remembered@example.test", now)
	if err := repo.CreateMagicLink(t.Context(), magic); err != nil {
		t.Fatal(err)
	}
	_, user, session, err := repo.ExchangeMagicLink(t.Context(), magic.TokenHash, postgresSession("remembered", now), now)
	if err != nil {
		t.Fatal(err)
	}
	device := model.Device{ID: platformid.New(), UserID: user.ID, TokenHash: integrationDigest("remembered-cookie"), CreatedAt: now, LastSeenAt: now}
	csrf := integrationDigest("shared-csrf")
	if _, err := repo.RememberSession(t.Context(), session.ID, device, csrf, now); err != nil {
		t.Fatal(err)
	}
	loaded, _, err := repo.SessionByTokenHash(t.Context(), session.TokenHash)
	if err != nil || loaded.DeviceID != device.ID {
		t.Fatalf("device association: %+v, %v", loaded, err)
	}
	remembered, _, err := repo.DeviceByTokenHash(t.Context(), device.TokenHash)
	if err != nil || remembered.ID != device.ID {
		t.Fatalf("remembered lookup: %v", err)
	}
	later := now.Add(30 * 24 * time.Hour)
	// Expired access sessions still expose the remembered browser for revocation.
	sessions, err := repo.ListActiveSessions(t.Context(), user.ID, later)
	if err != nil || len(sessions) != 1 {
		t.Fatalf("expired device listing: %+v, %v", sessions, err)
	}
	var wg sync.WaitGroup
	for i := range 8 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			replacement := postgresSession(platformid.New(), later)
			replacement.UserID = user.ID
			replacement.DeviceID = device.ID
			replacement.CSRFHash = csrf
			if err := repo.RestoreDeviceSession(t.Context(), device, replacement, later); err != nil {
				t.Errorf("concurrent restore: %v", err)
			}
		}(i)
	}
	wg.Wait()
	sessions, err = repo.ListActiveSessions(t.Context(), user.ID, later)
	if err != nil || len(sessions) != 1 {
		t.Fatalf("deduplicated browsers: %+v, %v", sessions, err)
	}
	if err := repo.RevokeUserSession(t.Context(), user.ID, session.ID, later); err != nil {
		t.Fatal(err)
	}
	revoked, _, err := repo.DeviceByID(t.Context(), device.ID)
	if err != nil || revoked.RevokedAt == nil {
		t.Fatalf("selected session did not revoke device: %v", err)
	}
	replacement := postgresSession("after-revoke", later)
	replacement.UserID = user.ID
	replacement.DeviceID = device.ID
	if err := repo.RestoreDeviceSession(t.Context(), device, replacement, later); !errors.Is(err, ErrNotFound) {
		t.Fatalf("revoked device resurrected: %v", err)
	}
	sessions, err = repo.ListActiveSessions(t.Context(), user.ID, later)
	if err != nil || len(sessions) != 0 {
		t.Fatalf("revoked browser remains: %+v, %v", sessions, err)
	}
	// Revocation and first-time remembering must serialize: a racing bootstrap
	// cannot create an unrevoked credential after revoke-all has scanned devices.
	for range 5 {
		nextMagic := postgresMagicLink(integrationDigest(platformid.New()), "remembered@example.test", later)
		if err := repo.CreateMagicLink(t.Context(), nextMagic); err != nil {
			t.Fatal(err)
		}
		_, _, nextSession, err := repo.ExchangeMagicLink(t.Context(), nextMagic.TokenHash, postgresSession(platformid.New(), later), later)
		if err != nil {
			t.Fatal(err)
		}
		nextDevice := model.Device{ID: platformid.New(), UserID: user.ID, TokenHash: integrationDigest(platformid.New()), CreatedAt: later, LastSeenAt: later}
		startRace := make(chan struct{})
		var race sync.WaitGroup
		race.Add(2)
		go func() {
			defer race.Done()
			<-startRace
			_, err := repo.RememberSession(t.Context(), nextSession.ID, nextDevice, csrf, later)
			if err != nil && !errors.Is(err, ErrNotFound) {
				t.Errorf("remember raced revocation: %v", err)
			}
		}()
		go func() {
			defer race.Done()
			<-startRace
			if err := repo.RevokeAllSessions(t.Context(), user.ID, later); err != nil {
				t.Errorf("revoke-all race: %v", err)
			}
		}()
		close(startRace)
		race.Wait()
		device, _, err := repo.DeviceByID(t.Context(), nextDevice.ID)
		if err == nil && device.RevokedAt == nil {
			t.Fatal("concurrent remembering resurrected a revoked browser")
		}
		if err != nil && !errors.Is(err, ErrNotFound) {
			t.Fatal(err)
		}
	}

}
