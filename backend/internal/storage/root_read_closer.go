package storage

import "os"

// rootReadCloser is the reader LocalStorage.Open hands out. It pairs the opened
// file with the os.Root it was opened through: both are descriptors, and the
// caller only ever closes the io.ReadCloser.
type rootReadCloser struct {
	file *os.File
	root *os.Root
}

func (r *rootReadCloser) Read(p []byte) (int, error) {
	return r.file.Read(p)
}

// Close closes the file and then the root. The file error wins when both fail,
// since it is the one describing the read the caller was in the middle of.
func (r *rootReadCloser) Close() error {
	fileErr := r.file.Close()
	rootErr := r.root.Close()

	if fileErr != nil {
		return fileErr
	}

	return rootErr
}
