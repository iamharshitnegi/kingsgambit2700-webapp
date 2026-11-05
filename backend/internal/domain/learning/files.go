package learning

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	platformid "github.com/kingsgambit2700/platform/backend/internal/platform/id"
	"github.com/kingsgambit2700/platform/backend/internal/platform/idempotency"
)

type FileStore interface {
	Put(context.Context, string, string, []byte, bool) error
	Get(context.Context, string) (io.ReadCloser, error)
	Delete(context.Context, string, bool) error
}
type MalwareScanner interface {
	Scan(context.Context, []byte) (bool, error)
}
type File struct {
	ID        string `json:"id"`
	Filename  string `json:"filename"`
	MediaType string `json:"mediaType"`
	SizeBytes int64  `json:"sizeBytes"`
	Status    string `json:"status"`
}
type UploadCommand struct {
	Filename, Title, Visibility, LevelSlug, AssignmentID, CertificateID string
	Tags                                                                []string
}
type UploadResult struct {
	File      File      `json:"file"`
	Workspace Workspace `json:"workspace"`
}

// ConfigureFiles must be called before serving requests. Uploads remain unavailable without both providers.
func (s *Service) ConfigureFiles(store FileStore, scanner MalwareScanner, maxBytes int64) error {
	if store == nil || scanner == nil {
		return errors.New("learning: file storage and malware scanner are required")
	}
	if maxBytes <= 0 || maxBytes > 20<<20 {
		maxBytes = 20 << 20
	}
	s.files = store
	s.scanner = scanner
	s.uploadMax = maxBytes
	s.uploads = make(chan struct{}, 4)
	return nil
}
func validateFile(name string, data []byte) (string, string, error) {
	extension := strings.ToLower(filepath.Ext(name))
	media := http.DetectContentType(data)
	switch extension {
	case ".pgn":
		if !utf8.Valid(data) {
			return "", "", invalid("PGN must be UTF-8 text")
		}
		if err := ValidatePGN(string(data)); err != nil {
			return "", "", err
		}
		return "application/x-chess-pgn", "pgn", nil
	case ".fen":
		if !utf8.Valid(data) {
			return "", "", invalid("FEN must be UTF-8 text")
		}
		if err := validateResource(Command{Title: "Position", Kind: "fen", Content: strings.TrimSpace(string(data))}); err != nil {
			return "", "", err
		}
		return "text/plain", "fen", nil
	case ".pdf":
		if media != "application/pdf" || len(data) > 20<<20 {
			return "", "", invalid("upload a PDF smaller than 20 MB")
		}
		for _, token := range []string{"/JavaScript", "/JS", "/Launch", "/EmbeddedFile", "/RichMedia"} {
			if bytes.Contains(data, []byte(token)) {
				return "", "", invalid("PDFs with scripts, embedded files or active content are not supported")
			}
		}
		return media, "pdf", nil
	case ".png", ".jpg", ".jpeg":
		if len(data) > 10<<20 || (media != "image/png" && media != "image/jpeg") {
			return "", "", invalid("upload a PNG or JPEG smaller than 10 MB")
		}
		cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
		if err != nil || cfg.Width <= 0 || cfg.Height <= 0 || int64(cfg.Width)*int64(cfg.Height) > 16_000_000 {
			return "", "", invalid("image dimensions are invalid or exceed 16 megapixels")
		}
		if _, _, err = image.Decode(bytes.NewReader(data)); err != nil {
			return "", "", invalid("image file is incomplete or invalid")
		}
		return media, "image", nil
	default:
		return "", "", invalid("supported files are PGN, FEN, PDF, PNG and JPEG")
	}
}
func (s *Service) UploadFile(ctx context.Context, actor string, c UploadCommand, reader io.Reader) (UploadResult, error) {
	if s.files == nil || s.scanner == nil {
		return UploadResult{}, invalid("file uploads are not configured")
	}
	if !platformid.ValidReference(actor) {
		return UploadResult{}, ErrDenied
	}
	if c.AssignmentID != "" && c.CertificateID != "" {
		return UploadResult{}, invalid("choose one upload destination")
	}
	for _, id := range []string{c.AssignmentID, c.CertificateID} {
		if id != "" && !platformid.ValidReference(id) {
			return UploadResult{}, invalid("upload target is invalid")
		}
	}
	select {
	case s.uploads <- struct{}{}:
		defer func() { <-s.uploads }()
	case <-ctx.Done():
		return UploadResult{}, ctx.Err()
	}
	student, err := s.authorizeUpload(ctx, s.pool, actor, c)
	if err != nil {
		return UploadResult{}, err
	}
	data, err := io.ReadAll(io.LimitReader(reader, s.uploadMax+1))
	if err != nil {
		return UploadResult{}, err
	}
	if len(data) == 0 || int64(len(data)) > s.uploadMax {
		return UploadResult{}, invalid("file is empty or exceeds the upload limit")
	}
	filename := filepath.Base(strings.ReplaceAll(c.Filename, "\\", "/"))
	if len(filename) > 180 || strings.ContainsAny(filename, "\r\n\x00") || filename == "." {
		return UploadResult{}, invalid("file name is invalid")
	}
	media, kind, err := validateFile(filename, data)
	if err != nil {
		return UploadResult{}, err
	}
	if c.CertificateID != "" && kind != "pdf" {
		return UploadResult{}, invalid("certificate attachments must be PDF files")
	}
	if c.AssignmentID == "" && c.CertificateID == "" {
		if !required(c.Title, 200) {
			return UploadResult{}, invalid("resource title is required")
		}
		if c.Visibility == "" {
			c.Visibility = "personal"
		}
		if c.Visibility != "personal" && c.Visibility != "shared" {
			return UploadResult{}, invalid("visibility must be personal or shared")
		}
		if len(c.Tags) > 20 || len(c.LevelSlug) > 100 {
			return UploadResult{}, invalid("resource metadata is too long")
		}
		for _, tag := range c.Tags {
			if !required(tag, 80) {
				return UploadResult{}, invalid("resource tags are invalid")
			}
		}
	}
	file := File{ID: platformid.New(), Filename: filename, MediaType: media, SizeBytes: int64(len(data)), Status: "quarantined"}
	key := "learning/" + file.ID
	hash := sha256.Sum256(data)
	request := struct {
		Command UploadCommand
		SHA256  [32]byte
	}{c, hash}
	if idempotency.Key(ctx) != "" {
		preflight, beginErr := s.pool.Begin(ctx)
		if beginErr != nil {
			return UploadResult{}, beginErr
		}
		var existing File
		found, replayErr := idempotency.Replay(ctx, preflight, actor, "learning.uploadFile", request, &existing)
		_ = preflight.Rollback(ctx)
		if errors.Is(replayErr, idempotency.ErrConflict) {
			return UploadResult{}, ErrConflict
		}
		if replayErr != nil {
			return UploadResult{}, replayErr
		}
		if found {
			w, loadErr := s.Workspace(ctx, actor)
			return UploadResult{File: existing, Workspace: w}, loadErr
		}
	}
	if _, err = s.pool.Exec(ctx, `INSERT INTO teaching_files(id,owner_id,student_id,filename,media_type,size_bytes,sha256,object_key,status) VALUES($1::uuid,$2::uuid,$3::uuid,$4,$5,$6,$7,$8,'quarantined')`, file.ID, actor, nullID(student), filename, media, file.SizeBytes, hash[:], key); err != nil {
		return UploadResult{}, err
	}
	var activeTx pgx.Tx
	fail := func(status string, cause error) (UploadResult, error) {
		// Release the transaction before acquiring another pooled connection.
		if activeTx != nil {
			_ = activeTx.Rollback(ctx)
			activeTx = nil
		}
		_, recordErr := s.pool.Exec(ctx, `UPDATE teaching_files SET status=$2,scanned_at=now() WHERE id=$1::uuid`, file.ID, status)
		return UploadResult{}, errors.Join(cause, recordErr)
	}
	if err = s.files.Put(ctx, key, media, data, false); err != nil {
		return fail("failed", err)
	}
	clean, err := s.scanner.Scan(ctx, data)
	if err != nil {
		return fail("failed", invalid("the file could not be scanned; retry after the scanner is available"))
	}
	if !clean {
		return fail("rejected", invalid("the file was rejected by malware scanning"))
	}
	if err = s.files.Put(ctx, key, media, data, true); err != nil {
		return fail("failed", err)
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fail("failed", err)
	}
	activeTx = tx
	defer tx.Rollback(ctx)
	if _, err = s.authorizeUpload(ctx, tx, actor, c); err != nil {
		return fail("failed", err)
	}
	var existing File
	found, replayErr := idempotency.Replay(ctx, tx, actor, "learning.uploadFile", request, &existing)
	if errors.Is(replayErr, idempotency.ErrConflict) {
		return fail("failed", ErrConflict)
	}
	if replayErr != nil {
		return fail("failed", replayErr)
	}
	if found {
		_ = tx.Rollback(ctx)
		activeTx = nil
		for _, clean := range []bool{false, true} {
			if deleteErr := s.files.Delete(ctx, key, clean); deleteErr != nil {
				slog.Warn("duplicate upload cleanup failed", "fileId", file.ID)
			}
		}
		if _, err = s.pool.Exec(ctx, `DELETE FROM teaching_files WHERE id=$1::uuid AND status='quarantined'`, file.ID); err != nil {
			return UploadResult{}, err
		}
		w, loadErr := s.Workspace(ctx, actor)
		return UploadResult{File: existing, Workspace: w}, loadErr
	}
	switch {
	case c.AssignmentID != "":
		a, loadErr := scanAssignment(tx.QueryRow(ctx, `SELECT `+assignmentColumns+` FROM learning_assignments WHERE id=$1::uuid FOR UPDATE`, c.AssignmentID))
		if loadErr != nil {
			return fail("failed", loadErr)
		}
		if a.Status != "assigned" {
			return fail("failed", ErrConflict)
		}
		attempts, loadErr := loadAttempts(ctx, tx, a.ID, len(a.Puzzles))
		if loadErr != nil {
			return fail("failed", loadErr)
		}
		attemptID := platformid.New()
		number := 1
		if len(attempts) > 0 {
			last := attempts[len(attempts)-1]
			if last.Status == "started" {
				attemptID = last.ID
				number = last.Number
			} else if last.Status == "returned" {
				number = last.Number + 1
			} else {
				return fail("failed", ErrConflict)
			}
		}
		if _, err = tx.Exec(ctx, `INSERT INTO assignment_attempts(id,assignment_id,student_profile_id,attempt_number,status,created_at) VALUES($1::uuid,$2::uuid,$3::uuid,$4,'started',now()) ON CONFLICT(id) DO NOTHING`, attemptID, a.ID, a.StudentID, number); err != nil {
			return fail("failed", err)
		}
		if _, err = tx.Exec(ctx, `INSERT INTO assignment_evidence(id,assignment_attempt_id,object_key,content_type,size_bytes,sha256,scan_status,scanned_at,created_at,file_id) VALUES($1::uuid,$2::uuid,$3,$4,$5,$6,'clean',now(),now(),$7::uuid)`, platformid.New(), attemptID, key, media, file.SizeBytes, hash[:], file.ID); err != nil {
			return fail("failed", err)
		}
		if _, err = tx.Exec(ctx, `UPDATE learning_assignments SET revision=revision+1 WHERE id=$1::uuid`, a.ID); err != nil {
			return fail("failed", err)
		}
	case c.CertificateID != "":
		tag, updateErr := tx.Exec(ctx, `UPDATE learning_certificates SET file_id=$2::uuid WHERE id=$1::uuid AND status='issued' AND file_id IS NULL`, c.CertificateID, file.ID)
		if updateErr != nil {
			return fail("failed", updateErr)
		}
		if tag.RowsAffected() != 1 {
			return fail("failed", ErrConflict)
		}
	default:
		content := ""
		if kind == "pgn" || kind == "fen" {
			content = strings.TrimSpace(string(data))
		}
		if c.Tags == nil {
			c.Tags = []string{}
		}
		_, err = tx.Exec(ctx, `INSERT INTO teaching_resources(id,owner_id,title,kind,visibility,status,level_slug,tags,content,file_id) VALUES($1::uuid,$2::uuid,$3,$4,$5,'draft',$6,$7,$8,$9::uuid)`, platformid.New(), actor, c.Title, kind, c.Visibility, c.LevelSlug, c.Tags, content, file.ID)
		if err != nil {
			return fail("failed", err)
		}
	}
	if _, err = tx.Exec(ctx, `UPDATE teaching_files SET status='clean',scanned_at=now() WHERE id=$1::uuid`, file.ID); err != nil {
		return fail("failed", err)
	}
	if err = audit(ctx, tx, actor, "uploadFile", file.ID); err != nil {
		return fail("failed", err)
	}
	file.Status = "clean"
	if err = idempotency.Save(ctx, tx, actor, "learning.uploadFile", request, file); err != nil {
		return fail("failed", err)
	}
	if err = tx.Commit(ctx); err != nil {
		return fail("failed", err)
	}
	if err = s.files.Delete(ctx, key, false); err != nil {
		slog.Warn("quarantine cleanup failed", "fileId", file.ID)
	}
	file.Status = "clean"
	w, err := s.Workspace(ctx, actor)
	return UploadResult{File: file, Workspace: w}, err
}
func (s *Service) authorizeUpload(ctx context.Context, q querier, actor string, c UploadCommand) (string, error) {
	if c.AssignmentID != "" {
		var student, status string
		err := q.QueryRow(ctx, `SELECT student_profile_id::text,status FROM learning_assignments WHERE id=$1::uuid`, c.AssignmentID).Scan(&student, &status)
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrNotFound
		}
		if err != nil {
			return "", err
		}
		if err = ownStudent(ctx, q, actor, student); err != nil {
			return "", err
		}
		if status != "assigned" {
			return "", ErrConflict
		}
		return student, nil
	}
	if c.CertificateID != "" {
		var student, status string
		err := q.QueryRow(ctx, `SELECT student_id::text,status FROM learning_certificates WHERE id=$1::uuid`, c.CertificateID).Scan(&student, &status)
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrNotFound
		}
		if err != nil {
			return "", err
		}
		if status != "issued" {
			return "", ErrConflict
		}
		return student, authorizeStudent(ctx, q, actor, student, "learning.publish", "", false)
	}
	staff, err := staffResourceAccess(ctx, q, actor)
	if err != nil {
		return "", err
	}
	if !staff {
		return "", ErrDenied
	}
	return "", nil
}
func (s *Service) DownloadFile(ctx context.Context, actor, id string) (File, io.ReadCloser, error) {
	if !platformid.ValidReference(actor) || !platformid.ValidReference(id) {
		return File{}, nil, ErrDenied
	}
	if s.files == nil {
		return File{}, nil, invalid("file downloads are not configured")
	}
	var file File
	var owner, student, key string
	err := s.pool.QueryRow(ctx, `SELECT id::text,owner_id::text,COALESCE(student_id::text,''),filename,media_type,size_bytes,status,object_key FROM teaching_files WHERE id=$1::uuid`, id).Scan(&file.ID, &owner, &student, &file.Filename, &file.MediaType, &file.SizeBytes, &file.Status, &key)
	if errors.Is(err, pgx.ErrNoRows) {
		return file, nil, ErrNotFound
	}
	if err != nil {
		return file, nil, err
	}
	if file.Status != "clean" {
		return file, nil, ErrDenied
	}
	if student != "" {
		err = authorizeStudent(ctx, s.pool, actor, student, "learning.view", "view_progress", true)
	} else {
		assigned, checkErr := assignedFileAccess(ctx, s.pool, actor, id)
		if checkErr != nil {
			return file, nil, checkErr
		}
		if assigned {
			body, readErr := s.files.Get(ctx, key)
			return file, body, readErr
		}
		staff, checkErr := staffResourceAccess(ctx, s.pool, actor)
		if checkErr != nil {
			return file, nil, checkErr
		}
		if !staff {
			return file, nil, ErrDenied
		}
		publish, checkErr := permission(ctx, s.pool, actor, "learning.publish")
		if checkErr != nil {
			return file, nil, checkErr
		}
		var visible bool
		err = s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM teaching_resources WHERE file_id=$1::uuid AND (owner_id=$2::uuid OR (visibility='shared' AND (status='published' OR $3))))`, id, actor, publish).Scan(&visible)
		if err == nil && !visible {
			err = ErrDenied
		}
	}
	if err != nil {
		return file, nil, err
	}
	body, err := s.files.Get(ctx, key)
	return file, body, err
}
