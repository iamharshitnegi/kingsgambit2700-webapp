package daily

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/kingsgambit2700/platform/backend/internal/provider"
)

func TestRecordingAndTranscriptionStayDisabled(t *testing.T) {
	t.Parallel()

	requests := make(chan map[string]any, 2)
	httpClient := &http.Client{Transport: roundTripper(func(r *http.Request) (*http.Response, error) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Errorf("Authorization = %q", got)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode request: %v", err)
			return &http.Response{StatusCode: http.StatusBadRequest, Body: io.NopCloser(strings.NewReader("{}"))}, nil
		}
		requests <- body
		responseBody := "{}"
		switch r.URL.Path {
		case "/rooms":
			responseBody = `{"id":"room-id","name":"lesson-1","url":"https://example.daily.co/lesson-1"}`
		case "/meeting-tokens":
			responseBody = `{"token":"meeting-token"}`
		default:
			return &http.Response{StatusCode: http.StatusNotFound, Body: io.NopCloser(strings.NewReader("{}"))}, nil
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(responseBody)),
		}, nil
	})}

	client, err := New("test-key", "https://daily.test", httpClient)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	expiresAt := time.Unix(1_800_000_000, 0).UTC()
	if _, err := client.CreateRoom(context.Background(), "lesson-1", expiresAt); err != nil {
		t.Fatalf("CreateRoom() error = %v", err)
	}
	roomRequest := <-requests
	roomProperties := nestedMap(t, roomRequest, "properties")
	assertValue(t, roomProperties, "enable_recording", "off")
	assertValue(t, roomProperties, "enable_recording_ui", false)
	assertValue(t, roomProperties, "enable_live_captions_ui", false)
	assertValue(t, roomProperties, "enable_transcription_storage", false)
	assertValue(t, roomProperties, "auto_start_transcription", false)

	_, err = client.IssueToken(context.Background(), provider.MeetingTokenRequest{
		RoomExternalID:       "lesson-1",
		UserID:               "user-1",
		DisplayName:          "Coach",
		CanAdminParticipants: true,
		ExpiresAt:            expiresAt,
	})
	if err != nil {
		t.Fatalf("IssueToken() error = %v", err)
	}
	tokenRequest := <-requests
	tokenProperties := nestedMap(t, tokenRequest, "properties")
	if _, exists := tokenProperties["is_owner"]; exists {
		t.Fatal("meeting token must not use is_owner")
	}
	assertValue(t, tokenProperties, "enable_recording", "off")
	assertValue(t, tokenProperties, "enable_recording_ui", false)
	assertValue(t, tokenProperties, "enable_live_captions_ui", false)
	assertValue(t, tokenProperties, "auto_start_transcription", false)
	permissions := nestedMap(t, tokenProperties, "permissions")
	assertValue(t, permissions, "canAdmin", []any{"participants"})
}

type roundTripper func(*http.Request) (*http.Response, error)

func (r roundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	return r(request)
}

func nestedMap(t *testing.T, value map[string]any, key string) map[string]any {
	t.Helper()
	nested, ok := value[key].(map[string]any)
	if !ok {
		t.Fatalf("%s = %#v, want object", key, value[key])
	}
	return nested
}

func assertValue(t *testing.T, values map[string]any, key string, want any) {
	t.Helper()
	if got := values[key]; !reflect.DeepEqual(got, want) {
		t.Errorf("%s = %#v, want %#v", key, got, want)
	}
}
