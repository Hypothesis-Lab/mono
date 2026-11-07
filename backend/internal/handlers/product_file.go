package handlers

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"

	"app/internal/api"
	"app/internal/fileextract"
	"app/internal/http/middlewares"
	"app/internal/repositories/file"
	"app/internal/repositories/hypothesis"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

const (
	maxFileSize  = 50 * 1024 * 1024 // 50 MB
	filePartName = "file"
	typePartName = "type"
)

var (
	errFileTooLarge = errors.New("file too large")

	allowedFileTypes = map[string]fileTypeInfo{
		".docx": {
			contentType:   "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
			detectedTypes: []string{"application/vnd.openxmlformats-officedocument.wordprocessingml.document", "application/zip"},
		},
		".doc": {
			contentType:   "application/msword",
			detectedTypes: []string{"application/msword", "application/octet-stream"},
		},
		".pdf": {
			contentType:   "application/pdf",
			detectedTypes: []string{"application/pdf"},
		},
		".txt": {
			contentType:   "text/plain",
			detectedTypes: []string{"text/plain"},
		},
		".md": {
			contentType:   "text/markdown",
			detectedTypes: []string{"text/plain"},
		},
	}
)

type fileTypeInfo struct {
	contentType   string
	detectedTypes []string
}

type parsedFile struct {
	filename    string
	contentType string
	size        int64
	data        []byte
}

func (s *Server) UploadProductFile(ctx context.Context, req api.UploadProductFileRequestObject) (api.UploadProductFileResponseObject, error) {
	userID, ok := middlewares.UserIDFromContext(ctx)
	if !ok {
		s.logger.Error("missing user id in context")

		return api.UploadProductFile500JSONResponse(errBody(http.StatusInternalServerError, "internal server error")), nil
	}

	if _, err := s.hypotheses.Get(ctx, userID, req.Id); err != nil {
		if errors.Is(err, hypothesis.ErrNotFound) {
			s.logger.Error("get hypothesis for upload", zap.String("id", req.Id), zap.Error(err))

			return api.UploadProductFile404JSONResponse(errBody(http.StatusNotFound, "hypothesis not found")), nil
		}

		s.logger.Error("get hypothesis for upload", zap.Error(err))

		return api.UploadProductFile500JSONResponse(errBody(http.StatusInternalServerError, "internal server error")), nil
	}

	if req.Body == nil {
		return api.UploadProductFile400JSONResponse(errBody(http.StatusBadRequest, "request body is required")), nil
	}

	filePart, fileType, err := parseMultipartUpload(req.Body)
	if err != nil {
		if errors.Is(err, errFileTooLarge) {
			s.logger.Error("upload product file", zap.Error(err))

			return api.UploadProductFile413JSONResponse(errBody(http.StatusRequestEntityTooLarge, "file exceeds 50 MB limit")), nil
		}

		s.logger.Error("upload product file", zap.Error(err))

		return api.UploadProductFile400JSONResponse(errBody(http.StatusBadRequest, err.Error())), nil
	}

	text, err := fileextract.Extract(ctx, filePart.filename, filePart.data)
	if err != nil {
		s.logger.Error("extract product file text", zap.String("hypothesis_id", req.Id), zap.String("filename", filePart.filename), zap.Error(err))
		return api.UploadProductFile400JSONResponse(errBody(http.StatusBadRequest, "could not extract file text")), nil
	}

	s3Key := uuid.NewString() + "/" + filePart.filename

	if err := s.storage.Upload(ctx, s3Key, filePart.contentType, filePart.size, bytes.NewReader(filePart.data)); err != nil {
		s.logger.Error("upload file to s3", zap.Error(err))

		return api.UploadProductFile500JSONResponse(errBody(http.StatusInternalServerError, "internal server error")), nil
	}

	textKey := s3Key + ".extracted.txt"
	if err := s.storage.Upload(ctx, textKey, "text/plain; charset=utf-8", int64(len(text)), strings.NewReader(text)); err != nil {
		s.logger.Error("upload extracted file text", zap.String("hypothesis_id", req.Id), zap.Error(err))
		if deleteErr := s.storage.Delete(ctx, s3Key); deleteErr != nil {
			s.logger.Error("cleanup original file after extraction upload failure", zap.String("key", s3Key), zap.Error(deleteErr))
		}

		return api.UploadProductFile500JSONResponse(errBody(http.StatusInternalServerError, "internal server error")), nil
	}

	created, err := s.files.Create(ctx, file.CreateInput{
		HypothesisID: req.Id,
		Type:         fileType,
		Filename:     filePart.filename,
		ContentType:  filePart.contentType,
		Size:         filePart.size,
		S3Key:        s3Key,
	})
	if err != nil {
		if deleteErr := s.storage.Delete(ctx, textKey); deleteErr != nil {
			s.logger.Error("cleanup extracted text after db failure", zap.String("key", textKey), zap.Error(deleteErr))
		}

		if deleteErr := s.storage.Delete(ctx, s3Key); deleteErr != nil {
			s.logger.Error("cleanup s3 object after db failure", zap.Error(deleteErr))
		}

		if errors.Is(err, file.ErrBrandbookAlreadyExists) {
			s.logger.Error("create file record", zap.String("hypothesis_id", req.Id), zap.Error(err))

			return api.UploadProductFile409JSONResponse(errBody(http.StatusConflict, "brandbook already exists for this hypothesis")), nil
		}

		s.logger.Error("create file record", zap.Error(err))

		return api.UploadProductFile500JSONResponse(errBody(http.StatusInternalServerError, "internal server error")), nil
	}

	return api.UploadProductFile201JSONResponse(newProductFileResponse(created)), nil
}

func (s *Server) ListProductFiles(ctx context.Context, req api.ListProductFilesRequestObject) (api.ListProductFilesResponseObject, error) {
	userID, ok := middlewares.UserIDFromContext(ctx)
	if !ok {
		s.logger.Error("missing user id in context")

		return api.ListProductFiles500JSONResponse(errBody(http.StatusInternalServerError, "internal server error")), nil
	}

	if _, err := s.hypotheses.Get(ctx, userID, req.Id); err != nil {
		if errors.Is(err, hypothesis.ErrNotFound) {
			s.logger.Error("get hypothesis for files", zap.String("id", req.Id), zap.Error(err))

			return api.ListProductFiles404JSONResponse(errBody(http.StatusNotFound, "hypothesis not found")), nil
		}

		s.logger.Error("get hypothesis for files", zap.Error(err))

		return api.ListProductFiles500JSONResponse(errBody(http.StatusInternalServerError, "internal server error")), nil
	}

	items, err := s.files.ListByHypothesisID(ctx, req.Id)
	if err != nil {
		s.logger.Error("list files", zap.Error(err))

		return api.ListProductFiles500JSONResponse(errBody(http.StatusInternalServerError, "internal server error")), nil
	}

	resp := make(api.ListProductFiles200JSONResponse, 0, len(items))
	for _, f := range items {
		resp = append(resp, newProductFileResponse(f))
	}

	return resp, nil
}

func (s *Server) DeleteProductFile(ctx context.Context, req api.DeleteProductFileRequestObject) (api.DeleteProductFileResponseObject, error) {
	userID, ok := middlewares.UserIDFromContext(ctx)
	if !ok {
		s.logger.Error("missing user id in context")

		return api.DeleteProductFile500JSONResponse(errBody(http.StatusInternalServerError, "internal server error")), nil
	}

	f, err := s.files.GetByUserID(ctx, userID, req.Id)
	if err != nil {
		if errors.Is(err, file.ErrNotFound) {
			s.logger.Error("get file for delete", zap.String("id", req.Id), zap.Error(err))

			return api.DeleteProductFile404JSONResponse(errBody(http.StatusNotFound, "file not found")), nil
		}

		s.logger.Error("get file for delete", zap.Error(err))

		return api.DeleteProductFile500JSONResponse(errBody(http.StatusInternalServerError, "internal server error")), nil
	}

	if err := s.files.Delete(ctx, req.Id); err != nil {
		s.logger.Error("delete file record", zap.Error(err))

		return api.DeleteProductFile500JSONResponse(errBody(http.StatusInternalServerError, "internal server error")), nil
	}

	if err := s.storage.Delete(ctx, f.S3Key); err != nil {
		s.logger.Error("delete file from s3", zap.Error(err))
	}

	if err := s.storage.Delete(ctx, f.S3Key+".extracted.txt"); err != nil {
		s.logger.Error("delete extracted file text from s3", zap.String("id", req.Id), zap.Error(err))
	}

	return api.DeleteProductFile204Response{}, nil
}

func parseMultipartUpload(reader *multipart.Reader) (parsedFile, file.Type, error) {
	var (
		result   parsedFile
		fileType file.Type
		hasFile  bool
		hasType  bool
	)

	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}

		if err != nil {
			return parsedFile{}, "", fmt.Errorf("invalid multipart body: %w", err)
		}

		switch part.FormName() {
		case filePartName:
			if hasFile {
				return parsedFile{}, "", errors.New("multiple file parts are not allowed")
			}

			parsed, parseErr := parseFilePart(part)
			if parseErr != nil {
				return parsedFile{}, "", parseErr
			}

			hasFile = true
			result = parsed

		case typePartName:
			if hasType {
				return parsedFile{}, "", errors.New("multiple type parts are not allowed")
			}

			parsedType, parseErr := parseFileTypePart(part)
			if parseErr != nil {
				return parsedFile{}, "", parseErr
			}

			hasType = true
			fileType = parsedType

		default:
			return parsedFile{}, "", fmt.Errorf("unexpected form field %q: expected %q or %q", part.FormName(), filePartName, typePartName)
		}
	}

	if !hasFile {
		return parsedFile{}, "", errors.New("file is required")
	}

	if !hasType {
		return parsedFile{}, "", errors.New("file type is required")
	}

	if err := validateUploadedFile(result.filename, result.contentType, result.data); err != nil {
		return parsedFile{}, "", err
	}

	return result, fileType, nil
}

func parseFilePart(part *multipart.Part) (parsedFile, error) {
	filename := part.FileName()
	if filename == "" {
		return parsedFile{}, errors.New("file name is required")
	}

	if ext := filepath.Ext(filename); ext != "" {
		filename = filename[:len(filename)-len(ext)] + strings.ToLower(ext)
	}

	contentType := part.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	data, err := io.ReadAll(io.LimitReader(part, maxFileSize+1))
	if err != nil {
		return parsedFile{}, fmt.Errorf("failed to read file: %w", err)
	}

	if int64(len(data)) > maxFileSize {
		return parsedFile{}, errFileTooLarge
	}

	return parsedFile{filename: filename, contentType: contentType, size: int64(len(data)), data: data}, nil
}

func parseFileTypePart(part *multipart.Part) (file.Type, error) {
	value, err := io.ReadAll(part)
	if err != nil {
		return "", fmt.Errorf("failed to read file type: %w", err)
	}

	switch strings.TrimSpace(string(value)) {
	case "other":
		return file.TypeOther, nil
	case "brandbook":
		return file.TypeBrandbook, nil
	case "product":
		return file.TypeProduct, nil
	case "reference":
		return file.TypeReference, nil
	default:
		return "", errors.New("type must be one of product, brandbook, reference, other")
	}
}

func validateUploadedFile(filename, contentType string, data []byte) error {
	ext := strings.ToLower(filepath.Ext(filename))
	info, ok := allowedFileTypes[ext]
	if !ok {
		return errors.New("invalid file extension")
	}

	declared, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return fmt.Errorf("parse declared content type: %w", err)
	}

	isTextFallback := (ext == ".md" || ext == ".txt") && (declared == "text/plain" || declared == "application/octet-stream")
	if declared != info.contentType && !isTextFallback {
		return errors.New("declared content type does not match file extension")
	}

	detected, _, err := mime.ParseMediaType(http.DetectContentType(data))
	if err != nil {
		return fmt.Errorf("parse detected content type: %w", err)
	}

	for _, allowed := range info.detectedTypes {
		if detected == allowed {
			return nil
		}
	}

	return errors.New("detected content type does not match allowed types for file extension")
}

func newProductFileResponse(f *file.File) api.ProductFileResponse {
	var fileType api.ProductFileResponseType
	switch f.Type {
	case file.TypeBrandbook:
		fileType = api.ProductFileResponseTypeBrandbook
	case file.TypeProduct:
		fileType = api.ProductFileResponseTypeProduct
	case file.TypeReference:
		fileType = api.ProductFileResponseTypeReference
	case file.TypeOther:
		fileType = api.ProductFileResponseTypeOther
	default:
		fileType = api.ProductFileResponseTypeOther
	}

	return api.ProductFileResponse{
		ContentType:  f.ContentType,
		CreatedAt:    f.CreatedAt,
		Filename:     f.Filename,
		HypothesisId: f.HypothesisID,
		Id:           f.ID,
		Size:         int(f.Size),
		Type:         fileType,
	}
}
