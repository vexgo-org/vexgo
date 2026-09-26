package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// LocalStorage stores files under <dataDir>/media and serves them at
// /uploads/<filename>.
type LocalStorage struct {
	rootDir string
	baseURL string
}

// NewLocalStorage creates a local-disk file storage.
func NewLocalStorage(dataDir string) *LocalStorage {
	return &LocalStorage{
		rootDir: filepath.Join(dataDir, "media"),
		baseURL: "/uploads",
	}
}

func (s *LocalStorage) Put(
	_ context.Context,
	key string,
	reader io.Reader,
	size int64,
	contentType string,
) error {
	cleanedKey, err := unwrapCleanKey(key)
	if err != nil {
		return err
	}

	root, err := s.openRoot()
	if err != nil {
		return err
	}
	defer func() { _ = root.Close() }()

	directory := path.Dir(cleanedKey)
	if directory != "." {
		if err := root.MkdirAll(directory, 0o750); err != nil {
			return fmt.Errorf("create object directory failed: %w", err)
		}
	}

	tempKey := cleanedKey + ".uploading"

	dst, err := root.Create(tempKey)
	if err != nil {
		return fmt.Errorf("create temporary object failed: %w", err)
	}

	if err := writeUploadFile(dst, reader); err != nil {
		removeLocalObject(root, tempKey)
		return err
	}

	if size >= 0 {
		info, err := root.Stat(tempKey)
		if err != nil {
			removeLocalObject(root, tempKey)
			return fmt.Errorf("stat uploaded object failed: %w", err)
		}

		if info.Size() != size {
			removeLocalObject(root, tempKey)
			return fmt.Errorf(
				"uploaded object size mismatch: expected %d, got %d",
				size,
				info.Size(),
			)
		}
	}

	if err := root.Rename(tempKey, cleanedKey); err != nil {
		removeLocalObject(root, tempKey)
		return fmt.Errorf("commit uploaded object failed: %w", err)
	}

	return nil
}

func (s *LocalStorage) Open(_ context.Context, key string) (io.ReadCloser, error) {
	cleanedKey, err := unwrapCleanKey(key)
	if err != nil {
		return nil, err
	}

	root, err := s.openRoot()
	if err != nil {
		return nil, err
	}

	file, err := root.Open(cleanedKey)
	if err != nil {
		_ = root.Close()
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrNotFound
		}

		return nil, fmt.Errorf("open local storage object failed: %w", err)
	}

	return &rootReadCloser{
		file: file,
		root: root,
	}, nil
}

func (s *LocalStorage) Delete(_ context.Context, key string) error {
	cleanedKey, err := unwrapCleanKey(key)
	if err != nil {
		return err
	}

	root, err := s.openRoot()
	if err != nil {
		return err
	}
	defer func() { _ = root.Close() }()

	if err := root.Remove(cleanedKey); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("delete local storage object failed: %w", err)
	}

	return nil
}

func (s *LocalStorage) URL(_ context.Context, key string) (string, error) {
	cleanedKey, err := unwrapCleanKey(key)
	if err != nil {
		return "", err
	}
	return strings.TrimRight(s.baseURL, "/") +
		"/" +
		cleanedKey, nil
}

func removeLocalObject(root *os.Root, key string) {
	err := root.Remove(key)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		slog.Warn(
			"failed to remove local storage object",
			"key", key,
			"err", err,
		)
	}
}

// openRoot opens an os.Root over the data directory. Every media file
// operation goes through it so a hostile filename (absolute path, ".."
// segment, volume name) can never resolve outside the media tree — the
// containment is enforced at the OS level, not by string checks.
func (s *LocalStorage) openRoot() (*os.Root, error) {
	if err := os.MkdirAll(s.rootDir, 0o750); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}
	root, err := os.OpenRoot(s.rootDir)
	if err != nil {
		return nil, fmt.Errorf("failed to open data directory: %w", err)
	}
	return root, nil
}

// writeUploadFile copies the upload into dst and closes it exactly once.
// Closing is part of writing, not cleanup: the bytes are only durable once the
// descriptor is flushed, so a late write-back failure (full disk, NFS) must
// fail the upload instead of being deferred away — that reported a truncated
// or empty file as stored. When both steps fail the copy error wins, since it
// describes the content loss, but the close still runs to release the
// descriptor.
func writeUploadFile(dst io.WriteCloser, reader io.Reader) error {
	_, copyErr := io.Copy(dst, reader)
	closeErr := dst.Close()
	switch {
	case copyErr != nil:
		return fmt.Errorf("failed to save file: %w", copyErr)
	case closeErr != nil:
		return fmt.Errorf("failed to flush uploaded file: %w", closeErr)
	default:
		return nil
	}
}
