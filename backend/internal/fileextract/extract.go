package fileextract

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"
)

const maxExtractedBytes = 50 * 1024 * 1024

type limitedBuffer struct {
	buffer   bytes.Buffer
	exceeded bool
}

func Extract(ctx context.Context, filename string, data []byte) (string, error) {
	switch strings.ToLower(filepath.Ext(filename)) {
	case ".txt", ".md":
		if !utf8.Valid(data) {
			return "", errors.New("text file is not valid UTF-8")
		}

		if len(data) > maxExtractedBytes {
			return "", errors.New("text file exceeds extraction limit")
		}

		return string(data), nil
	case ".docx":
		return extractDOCX(data)
	case ".pdf", ".doc":
		return extractExternal(ctx, filepath.Ext(filename), data)
	default:
		return "", errors.New("unsupported extraction format")
	}
}

func extractDOCX(data []byte) (string, error) {
	archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("open docx archive: %w", err)
	}

	for _, entry := range archive.File {
		if entry.Name != "word/document.xml" {
			continue
		}

		return extractDOCXEntry(entry)
	}

	return "", errors.New("docx document is missing word/document.xml")
}

func extractDOCXEntry(entry *zip.File) (string, error) {
	if entry.UncompressedSize64 > maxExtractedBytes {
		return "", errors.New("docx document exceeds extraction limit")
	}

	reader, err := entry.Open()
	if err != nil {
		return "", fmt.Errorf("open docx document: %w", err)
	}
	defer reader.Close()
	decoder := xml.NewDecoder(io.LimitReader(reader, maxExtractedBytes+1))
	var output strings.Builder
	inText := false
	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			break
		}

		if err != nil {
			return "", fmt.Errorf("parse docx document: %w", err)
		}

		switch value := token.(type) {
		case xml.StartElement:
			switch value.Name.Local {
			case "t":
				inText = true
			case "tab":
				output.WriteByte('\t')
			case "br":
				output.WriteByte('\n')
			}
		case xml.CharData:
			if inText {
				output.Write([]byte(value))
			}
		case xml.EndElement:
			switch value.Name.Local {
			case "t":
				inText = false
			case "p":
				output.WriteByte('\n')
			}
		}
	}

	return output.String(), nil
}

func extractExternal(ctx context.Context, extension string, data []byte) (string, error) {
	file, err := os.CreateTemp("", "product-document-*")
	if err != nil {
		return "", fmt.Errorf("create document extraction file: %w", err)
	}
	defer os.Remove(file.Name())
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		return "", fmt.Errorf("write document extraction file: %w", err)
	}

	if err := file.Close(); err != nil {
		return "", fmt.Errorf("close document extraction file: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	var command *exec.Cmd
	if strings.EqualFold(extension, ".pdf") {
		command = exec.CommandContext(ctx, "pdftotext", "-layout", "-enc", "UTF-8", file.Name(), "-") //nolint:gosec // executable and arguments are fixed; filename is a temporary path
	} else {
		command = exec.CommandContext(ctx, "antiword", "-m", "UTF-8.txt", file.Name()) //nolint:gosec // executable and arguments are fixed; filename is a temporary path
	}

	var output limitedBuffer
	command.Stdout = &output
	if err := command.Run(); err != nil {
		return "", fmt.Errorf("extract %s document: %w", extension, err)
	}

	if output.exceeded {
		return "", errors.New("document text exceeds extraction limit")
	}

	return output.buffer.String(), nil
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if b.buffer.Len()+len(p) > maxExtractedBytes {
		b.exceeded = true
		return 0, errors.New("extracted text exceeds limit")
	}

	return b.buffer.Write(p)
}
