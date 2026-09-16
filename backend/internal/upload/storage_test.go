package upload

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// closingWriter is an io.WriteCloser whose Close can be made to fail, standing
// in for a backend that only reports a write-back error at flush time.
type closingWriter struct {
	io.Writer
	closeErr error
	closed   bool
}

func (c *closingWriter) Close() error {
	c.closed = true
	return c.closeErr
}

// failingReader fails on its first Read.
type failingReader struct {
	err error
}

func (r failingReader) Read([]byte) (int, error) {
	return 0, r.err
}

// A close failure means the bytes never reached durable storage. Reporting it
// keeps a truncated file from being recorded as a successful upload.
func TestWriteUploadFile_CloseErrorIsReported(t *testing.T) {
	flushErr := errors.New("no space left on device")
	dst := &closingWriter{Writer: io.Discard, closeErr: flushErr}

	err := writeUploadFile(dst, strings.NewReader("data"))
	if !errors.Is(err, flushErr) {
		t.Fatalf("expected the close error to be reported, got %v", err)
	}
	if !dst.closed {
		t.Error("expected the file to be closed")
	}
}

// A copy failure is the primary error — the close error must not mask it — but
// the descriptor still has to be released.
func TestWriteUploadFile_CopyErrorWins(t *testing.T) {
	flushErr := errors.New("flush failed")
	copyErr := errors.New("read failed")
	dst := &closingWriter{Writer: io.Discard, closeErr: flushErr}

	err := writeUploadFile(dst, failingReader{err: copyErr})
	if !errors.Is(err, copyErr) {
		t.Fatalf("expected the copy error to be reported, got %v", err)
	}
	if errors.Is(err, flushErr) {
		t.Errorf("copy error should not be joined with the close error, got %v", err)
	}
	if !dst.closed {
		t.Error("expected the file to be closed even when the copy failed")
	}
}

// A failed write must not leave the partial file behind: no media row is
// created for it, so nothing could ever serve or delete it through the API and
// it would only accumulate orphans under data/media.
func TestLocalStorage_UploadRemovesPartialFileOnCopyError(t *testing.T) {
	dataDir := t.TempDir()
	storage := NewLocalStorage(dataDir)
	copyErr := errors.New("read failed")

	if _, err := storage.Upload(context.Background(), failingReader{err: copyErr}, "partial.txt", ""); !errors.Is(err, copyErr) {
		t.Fatalf("expected the copy error to be reported, got %v", err)
	}

	if _, err := os.Stat(filepath.Join(dataDir, "media", "partial.txt")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("partial upload should be removed, Stat error = %v", err)
	}
}

// The happy path writes the content and reports no error.
func TestWriteUploadFile_Success(t *testing.T) {
	var buf strings.Builder
	dst := &closingWriter{Writer: &buf}

	if err := writeUploadFile(dst, strings.NewReader("hello")); err != nil {
		t.Fatalf("writeUploadFile error: %v", err)
	}
	if buf.String() != "hello" {
		t.Errorf("written content = %q, want %q", buf.String(), "hello")
	}
	if !dst.closed {
		t.Error("expected the file to be closed")
	}
}
