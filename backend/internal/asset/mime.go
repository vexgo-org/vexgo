package asset

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net/http"
)

// DetectMIMEType detects MIME type from the file header.
func DetectMIMEType(reader *bufio.Reader) (string, error) {
	header, err := reader.Peek(512)
	if err != nil &&
		!errors.Is(err, io.EOF) &&
		!errors.Is(err, bufio.ErrBufferFull) {
		return "", fmt.Errorf("read file header: %w", err)
	}

	if len(header) == 0 {
		return "", ErrEmptyFile
	}

	return http.DetectContentType(header), nil
}
