package storage

import "os"

type rootReadCloser struct {
	file *os.File
	root *os.Root
}

func (r *rootReadCloser) Read(p []byte) (int, error) {
	return r.file.Read(p)
}

func (r *rootReadCloser) Close() error {
	fileErr := r.file.Close()
	rootErr := r.root.Close()

	if fileErr != nil {
		return fileErr
	}

	return rootErr
}
