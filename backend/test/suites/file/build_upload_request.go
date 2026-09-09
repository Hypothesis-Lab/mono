package file

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildUploadRequest(t *testing.T) {
	body, contentType := buildUploadRequest("test.pdf", "application/pdf", []byte("hello world"), "other")

	require.Equal(t, "multipart/form-data; boundary=test-boundary", contentType)
	require.Equal(t, "--test-boundary\r\n"+
		"Content-Disposition: form-data; name=\"file\"; filename=\"test.pdf\"\r\n"+
		"Content-Type: application/pdf\r\n"+
		"\r\n"+
		"hello world\r\n"+
		"--test-boundary\r\n"+
		"Content-Disposition: form-data; name=\"type\"\r\n"+
		"\r\n"+
		"other\r\n"+
		"--test-boundary--\r\n", body.String())
}
