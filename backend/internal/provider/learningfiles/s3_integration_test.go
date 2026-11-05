//go:build integration

package learningfiles

import (
	"context"
	"io"
	"os"
	"testing"
	"time"

	"github.com/kingsgambit2700/platform/backend/internal/platform/config"
	platformid "github.com/kingsgambit2700/platform/backend/internal/platform/id"
)

func TestLocalS3AndClamAVPipeline(t *testing.T) {
	if os.Getenv("KG_TEST_FILES") != "1" {
		t.Skip("KG_TEST_FILES=1 enables local S3Mock/ClamAV integration")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	store, err := NewStore(ctx, config.Config{S3Endpoint: "http://127.0.0.1:9090", S3Region: "us-east-1", S3AccessKeyID: "local-access-key", S3SecretAccessKey: "local-secret-key", S3BucketClean: "kg2700-clean", S3BucketQuarantine: "kg2700-quarantine", S3ForcePathStyle: true})
	if err != nil {
		t.Fatal(err)
	}
	key := "integration/" + platformid.New()
	body := []byte("[Event \"Integration\"]\n\n1. e4 e5 *")
	t.Cleanup(func() {
		for _, clean := range []bool{true, false} {
			if err := store.Delete(context.Background(), key, clean); err != nil {
				t.Error(err)
			}
		}
	})
	if err = store.Put(ctx, key, "application/x-chess-pgn", body, false); err != nil {
		t.Fatal(err)
	}
	scanner := Scanner{Address: "127.0.0.1:3310"}
	clean, err := scanner.Scan(ctx, body)
	if err != nil || !clean {
		t.Fatalf("clean scan: %v, %v", clean, err)
	}
	if err = store.Put(ctx, key, "application/x-chess-pgn", body, true); err != nil {
		t.Fatal(err)
	}
	reader, err := store.Get(ctx, key)
	if err != nil {
		t.Fatal(err)
	}
	got, readErr := io.ReadAll(reader)
	reader.Close()
	if readErr != nil || string(got) != string(body) {
		t.Fatal("clean object roundtrip failed")
	}
	// EICAR is the harmless standard antivirus test fixture.
	fixture := []byte("X5O!P%@AP[4\\PZX54(P^)7CC)7}$EICAR-STANDARD-ANTIVIRUS-TEST-FILE!$H+H*")
	clean, err = scanner.Scan(ctx, fixture)
	if err != nil {
		t.Fatal(err)
	}
	if clean {
		t.Fatal("scanner accepted EICAR test fixture")
	}
}
