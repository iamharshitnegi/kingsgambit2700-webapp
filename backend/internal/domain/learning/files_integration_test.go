//go:build integration

package learning

import (
	"bytes"
	"context"
	"errors"
	"github.com/kingsgambit2700/platform/backend/internal/platform/idempotency"
	"io"
	"sync"
	"testing"
)

type memoryFiles struct {
	objects map[string][]byte
	mu      sync.Mutex
}

func (m *memoryFiles) Put(_ context.Context, key, _ string, data []byte, clean bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if clean {
		key = "clean/" + key
	}
	m.objects[key] = bytes.Clone(data)
	return nil
}
func (m *memoryFiles) Get(_ context.Context, key string) (io.ReadCloser, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	data, ok := m.objects["clean/"+key]
	if !ok {
		return nil, ErrNotFound
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}
func (m *memoryFiles) Delete(_ context.Context, key string, clean bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if clean {
		key = "clean/" + key
	}
	delete(m.objects, key)
	return nil
}

type resultScanner struct {
	clean bool
	err   error
}

func (s resultScanner) Scan(context.Context, []byte) (bool, error) { return s.clean, s.err }

func TestUploadEvidenceAndLiveDownloadAuthorization(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	store := &memoryFiles{objects: map[string][]byte{}}
	if err := f.service.ConfigureFiles(store, resultScanner{clean: true}, 20<<20); err != nil {
		t.Fatal(err)
	}
	pdf := []byte("%PDF-1.4\n1 0 obj << /Type /Catalog >> endobj\n%%EOF")
	w := mustAction(t, f, f.coach, Command{Action: "createAssignment", StudentIDs: []string{f.profile}, Title: "Annotated game", Description: "Upload your written analysis", Kind: "file_upload", Status: "assigned"})
	a := w.Assignments[0]
	if _, err := f.service.Action(ctx, f.student, Command{Action: "submitAssignment", ID: a.ID, Version: a.Version}); err == nil {
		t.Fatal("empty file submission accepted")
	}
	upload, err := f.service.UploadFile(ctx, f.student, UploadCommand{Filename: "analysis.pdf", AssignmentID: a.ID}, bytes.NewReader(pdf))
	if err != nil {
		t.Fatal(err)
	}
	if upload.File.Status != "clean" || len(upload.Workspace.Assignments[0].Attempts[0].Files) != 1 {
		t.Fatal("clean evidence missing")
	}
	if _, ok := store.objects["learning/"+upload.File.ID]; ok {
		t.Fatal("clean upload still in quarantine")
	}
	_, reader, err := f.service.DownloadFile(ctx, f.guardian, upload.File.ID)
	if err != nil {
		t.Fatal(err)
	}
	got, err := io.ReadAll(reader)
	reader.Close()
	if err != nil || !bytes.Equal(got, pdf) {
		t.Fatal("guardian download differs")
	}
	if _, _, err = f.service.DownloadFile(ctx, f.outsider, upload.File.ID); !errors.Is(err, ErrDenied) {
		t.Fatalf("outsider download: %v", err)
	}
	a = upload.Workspace.Assignments[0]
	mustAction(t, f, f.student, Command{Action: "submitAssignment", ID: a.ID, Version: a.Version})
	if _, err = f.pool.Exec(ctx, `UPDATE guardian_student_relationships SET status='revoked' WHERE guardian_user_id=$1::uuid`, f.guardian); err != nil {
		t.Fatal(err)
	}
	if _, _, err = f.service.DownloadFile(ctx, f.guardian, upload.File.ID); !errors.Is(err, ErrDenied) {
		t.Fatalf("revoked guardian download: %v", err)
	}
}
func TestScannerFailureCannotPublishFiles(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	store := &memoryFiles{objects: map[string][]byte{}}
	if err := f.service.ConfigureFiles(store, resultScanner{clean: false}, 20<<20); err != nil {
		t.Fatal(err)
	}
	_, err := f.service.UploadFile(ctx, f.coach, UploadCommand{Filename: "study.pdf", Title: "Study"}, bytes.NewBufferString("%PDF-1.4\n%%EOF"))
	if err == nil {
		t.Fatal("malware rejection ignored")
	}
	var count int
	if err = f.pool.QueryRow(ctx, `SELECT count(*) FROM teaching_files WHERE status='rejected'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatal("rejection was not persisted")
	}
	if err = f.pool.QueryRow(ctx, `SELECT count(*) FROM teaching_resources`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("rejected file became a resource")
	}
	if err = f.service.ConfigureFiles(store, resultScanner{err: errors.New("scanner down")}, 20<<20); err != nil {
		t.Fatal(err)
	}
	_, err = f.service.UploadFile(ctx, f.coach, UploadCommand{Filename: "study.pdf", Title: "Study"}, bytes.NewBufferString("%PDF-1.4\n%%EOF"))
	if err == nil {
		t.Fatal("scanner outage accepted upload")
	}
}

func TestAssignedResourceSnapshotSurvivesArchival(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	store := &memoryFiles{objects: map[string][]byte{}}
	if err := f.service.ConfigureFiles(store, resultScanner{clean: true}, 20<<20); err != nil {
		t.Fatal(err)
	}
	upload, err := f.service.UploadFile(ctx, f.coach, UploadCommand{Filename: "study.pdf", Title: "Opening study", Visibility: "personal"}, bytes.NewBufferString("%PDF-1.4\n%%EOF"))
	if err != nil {
		t.Fatal(err)
	}
	r := upload.Workspace.Resources[0]
	c := Command{Action: "createAssignment", StudentID: f.profile, Title: "Study this position", Description: "Explain your first move", Kind: "study", Status: "assigned", ResourceIDs: []string{r.ID}}
	if _, err = f.service.Action(ctx, f.coach, c); err == nil {
		t.Fatal("draft resource attached")
	}
	w := mustAction(t, f, f.coach, Command{Action: "publishResource", ID: r.ID, Version: r.Version})
	r = w.Resources[0]
	if _, _, err = f.service.DownloadFile(ctx, f.guardian, upload.File.ID); !errors.Is(err, ErrDenied) {
		t.Fatalf("unassigned personal file: %v", err)
	}
	w = mustAction(t, f, f.coach, c)
	a := w.Assignments[0]
	if len(a.Resources) != 1 || a.Resources[0].Version != r.Version || a.Resources[0].Title != r.Title {
		t.Fatal("resource version was not snapshotted")
	}
	w = mustAction(t, f, f.coach, Command{Action: "reviseResource", ID: r.ID, Version: r.Version})
	if _, reader, err := f.service.DownloadFile(ctx, f.guardian, upload.File.ID); err != nil {
		t.Fatalf("revision hid assigned download: %v", err)
	} else {
		reader.Close()
	}
	w = mustAction(t, f, f.coach, Command{Action: "editResource", ID: r.ID, Version: w.Resources[0].Version, Title: "Revised PDF description", Kind: r.Kind, Content: r.Content, Tags: []string{"classwork"}})
	if w.Resources[0].FileID != r.FileID {
		t.Fatal("metadata revision replaced immutable file")
	}
	if _, err = f.service.Action(ctx, f.coach, Command{Action: "editResource", ID: r.ID, Version: w.Resources[0].Version, Title: "Forged PDF replacement", Kind: "study", Content: "Different"}); err == nil {
		t.Fatal("metadata edit replaced uploaded file content")
	}
	mustAction(t, f, f.coach, Command{Action: "archiveResource", ID: r.ID, Version: w.Resources[0].Version})
	family, err := f.service.Workspace(ctx, f.guardian)
	if err != nil {
		t.Fatal(err)
	}
	if len(family.Assignments[0].Resources) != 1 || family.Assignments[0].Resources[0].Version != r.Version {
		t.Fatal("archival changed assigned material")
	}
	_, reader, err := f.service.DownloadFile(ctx, f.guardian, upload.File.ID)
	if err != nil {
		t.Fatal(err)
	}
	reader.Close()
	if _, err = f.pool.Exec(ctx, `UPDATE guardian_student_relationships SET status='revoked' WHERE guardian_user_id=$1::uuid`, f.guardian); err != nil {
		t.Fatal(err)
	}
	if _, _, err = f.service.DownloadFile(ctx, f.guardian, upload.File.ID); !errors.Is(err, ErrDenied) {
		t.Fatalf("revoked guardian assigned file: %v", err)
	}
}

func TestUploadIdempotencyPreventsDuplicateResources(t *testing.T) {
	f := setup(t)
	ctx := idempotency.WithKey(context.Background(), "stable-upload-test")
	store := &memoryFiles{objects: map[string][]byte{}}
	if err := f.service.ConfigureFiles(store, resultScanner{clean: true}, 20<<20); err != nil {
		t.Fatal(err)
	}
	command := UploadCommand{Filename: "study.pdf", Title: "Idempotent study", Visibility: "personal"}
	type result struct {
		upload UploadResult
		err    error
	}
	results := make(chan result, 2)
	for range 2 {
		go func() {
			upload, err := f.service.UploadFile(ctx, f.coach, command, bytes.NewBufferString("%PDF-1.4\n%%EOF"))
			results <- result{upload, err}
		}()
	}
	first, second := <-results, <-results
	if first.err != nil || second.err != nil {
		t.Fatalf("parallel retry: %v / %v", first.err, second.err)
	}
	if first.upload.File.ID != second.upload.File.ID {
		t.Fatal("retry created another file")
	}
	var count int
	if err := f.pool.QueryRow(ctx, `SELECT count(*) FROM teaching_resources`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatal("retry published duplicate resources")
	}
	if err := f.pool.QueryRow(ctx, `SELECT count(*) FROM teaching_files`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatal("duplicate file metadata was not cleaned")
	}
	if _, err := f.service.UploadFile(ctx, f.coach, command, bytes.NewBufferString("%PDF-1.4\nchanged\n%%EOF")); !errors.Is(err, ErrConflict) {
		t.Fatalf("changed bytes with reused key: %v", err)
	}
	if _, err := f.pool.Exec(ctx, `UPDATE users SET status='suspended' WHERE id=$1::uuid`, f.coach); err != nil {
		t.Fatal(err)
	}
	if _, err := f.service.UploadFile(ctx, f.coach, command, bytes.NewBufferString("%PDF-1.4\n%%EOF")); !errors.Is(err, ErrDenied) {
		t.Fatalf("replay bypassed revoked actor: %v", err)
	}
}
