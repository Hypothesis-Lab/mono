package handlers

import (
	"bytes"
	"mime/multipart"
	"net/textproto"
	"testing"

	"app/internal/repositories/file"

	"github.com/stretchr/testify/require"
)

var (
	pdfContent = []byte("%PDF-1.4\n1 0 obj\n<<\n/Type /Catalog\n>>\nendobj\n")
	docxContent = []byte{0x50, 0x4B, 0x03, 0x04, 0x14, 0x00, 0x06, 0x00, 0x00, 0x00, 0x00, 0x00}
	docContent = []byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1, 0x00, 0x00, 0x00, 0x00}
	txtContent = []byte("hello world")
)

func TestParseMultipartUpload(t *testing.T) {
	cases := []struct {
		name     string
		build    func(t *testing.T, w *multipart.Writer)
		wantFile parsedFile
		wantType file.Type
		wantErr  string
	}{
		{
			name: "file and type fields pdf",
			build: func(t *testing.T, w *multipart.Writer) {
				writeFilePart(t, w, "test.pdf", "application/pdf", pdfContent)
				require.NoError(t, w.WriteField("type", "brandbook"))
			},
			wantFile: parsedFile{
				filename:    "test.pdf",
				contentType: "application/pdf",
				size:        int64(len(pdfContent)),
				data:        pdfContent,
			},
			wantType: file.TypeBrandbook,
		},
		{
			name: "file and type fields docx",
			build: func(t *testing.T, w *multipart.Writer) {
				writeFilePart(t, w, "test.docx", "application/vnd.openxmlformats-officedocument.wordprocessingml.document", docxContent)
				require.NoError(t, w.WriteField("type", "product"))
			},
			wantFile: parsedFile{
				filename:    "test.docx",
				contentType: "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
				size:        int64(len(docxContent)),
				data:        docxContent,
			},
			wantType: file.TypeProduct,
		},
		{
			name: "file and type fields doc",
			build: func(t *testing.T, w *multipart.Writer) {
				writeFilePart(t, w, "test.doc", "application/msword", docContent)
				require.NoError(t, w.WriteField("type", "reference"))
			},
			wantFile: parsedFile{
				filename:    "test.doc",
				contentType: "application/msword",
				size:        int64(len(docContent)),
				data:        docContent,
			},
			wantType: file.TypeReference,
		},
		{
			name: "file and type fields txt",
			build: func(t *testing.T, w *multipart.Writer) {
				writeFilePart(t, w, "test.txt", "text/plain", txtContent)
				require.NoError(t, w.WriteField("type", "other"))
			},
			wantFile: parsedFile{
				filename:    "test.txt",
				contentType: "text/plain",
				size:        int64(len(txtContent)),
				data:        txtContent,
			},
			wantType: file.TypeOther,
		},
		{
			name:    "empty body",
			build:   func(*testing.T, *multipart.Writer) {},
			wantErr: "file is required",
		},
		{
			name: "type only, invalid value",
			build: func(t *testing.T, w *multipart.Writer) {
				require.NoError(t, w.WriteField("type", "invalid"))
			},
			wantErr: "type must be one of product, brandbook, reference, other",
		},
		{
			name: "type only, valid brandbook",
			build: func(t *testing.T, w *multipart.Writer) {
				require.NoError(t, w.WriteField("type", "brandbook"))
			},
			wantErr: "file is required",
		},
		{
			name: "file only, pdf",
			build: func(t *testing.T, w *multipart.Writer) {
				writeFilePart(t, w, "test.pdf", "application/pdf", pdfContent)
			},
			wantErr: "file type is required",
		},
		{
			name: "invalid file extension",
			build: func(t *testing.T, w *multipart.Writer) {
				writeFilePart(t, w, "test.png", "image/png", txtContent)
				require.NoError(t, w.WriteField("type", "other"))
			},
			wantErr: "invalid file extension",
		},
		{
			name: "invalid declared content type",
			build: func(t *testing.T, w *multipart.Writer) {
				writeFilePart(t, w, "test.pdf", "image/png", pdfContent)
				require.NoError(t, w.WriteField("type", "other"))
			},
			wantErr: "declared content type does not match file extension",
		},
		{
			name: "invalid detected content type",
			build: func(t *testing.T, w *multipart.Writer) {
				writeFilePart(t, w, "test.pdf", "application/pdf", txtContent)
				require.NoError(t, w.WriteField("type", "other"))
			},
			wantErr: "detected content type does not match allowed types for file extension",
		},
		{
			name: "uppercase file extension is normalized",
			build: func(t *testing.T, w *multipart.Writer) {
				writeFilePart(t, w, "test.PDF", "application/pdf", pdfContent)
				require.NoError(t, w.WriteField("type", "other"))
			},
			wantFile: parsedFile{
				filename:    "test.pdf",
				contentType: "application/pdf",
				size:        int64(len(pdfContent)),
				data:        pdfContent,
			},
			wantType: file.TypeOther,
		},
		{
			name: "unexpected part name",
			build: func(t *testing.T, w *multipart.Writer) {
				writeFilePart(t, w, "test.pdf", "application/pdf", pdfContent)
				require.NoError(t, w.WriteField("type", "other"))
				require.NoError(t, w.WriteField("unexpected", "value"))
			},
			wantErr: `unexpected form field "unexpected": expected "file" or "type"`,
		},
		{
			name: "multiple type parts",
			build: func(t *testing.T, w *multipart.Writer) {
				writeFilePart(t, w, "test.pdf", "application/pdf", pdfContent)
				require.NoError(t, w.WriteField("type", "other"))
				require.NoError(t, w.WriteField("type", "brandbook"))
			},
			wantErr: "multiple type parts are not allowed",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var body bytes.Buffer
			writer := multipart.NewWriter(&body)

			tc.build(t, writer)
			require.NoError(t, writer.Close())

			t.Logf("constructed body:\n%s", body.String())

			reader := multipart.NewReader(&body, writer.Boundary())

			parsed, fileType, err := parseMultipartUpload(reader)
			if tc.wantErr != "" {
				require.EqualError(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.wantType, fileType)
			require.Equal(t, tc.wantFile, parsed)
		})
	}
}

func writeFilePart(t *testing.T, w *multipart.Writer, filename, contentType string, data []byte) {
	t.Helper()

	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", `form-data; name="file"; filename=`+quote(filename))
	header.Set("Content-Type", contentType)

	part, err := w.CreatePart(header)
	require.NoError(t, err)

	_, err = part.Write(data)
	require.NoError(t, err)
}

func quote(s string) string {
	return `"` + s + `"`
}
