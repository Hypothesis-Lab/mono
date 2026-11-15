package yandexdirect

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

type imageDeleteResponse struct {
	Result struct {
		DeleteResults []struct {
			Hash   string               `json:"AdImageHash"`
			Errors []actionNotification `json:"Errors"`
		} `json:"DeleteResults"`
	} `json:"result"`
	Error *yandexRequestError `json:"error"`
}

// UploadAdImageTracked distinguishes allocation from provider content deduplication.
func (c *Client) UploadAdImageTracked(ctx context.Context, name string, data []byte) (string, bool, error) {
	objects, err := c.listAllAt(ctx, c.v501Endpoint(), "adimages", "AdImages", map[string]any{}, []string{"AdImageHash"}, nil)
	if err != nil {
		return "", false, fmt.Errorf("snapshot image hashes before upload: %w", err)
	}

	existing := make(map[string]struct{}, len(objects))
	for _, raw := range objects {
		var image struct {
			Hash string `json:"AdImageHash"`
		}
		if err := json.Unmarshal(raw, &image); err != nil || image.Hash == "" {
			return "", false, errors.New("image allocation snapshot contained an invalid hash")
		}

		existing[image.Hash] = struct{}{}
	}

	hash, err := c.CreateAdImage(ctx, name, data)
	if err != nil {
		return "", false, err
	}

	_, reused := existing[hash]
	return hash, !reused, nil
}

func (c *Client) ImageExistsForRollback(ctx context.Context, hash string) (bool, error) {
	_, found, err := c.imageForRollback(ctx, hash)
	return found, err
}

func (c *Client) imageForRollback(ctx context.Context, hash string) (string, bool, error) {
	if hash == "" {
		return "", false, errors.New("rollback image hash must not be empty")
	}

	result, err := c.getAt(ctx, c.v501Endpoint(), "adimages", "AdImages", GetOptions{
		SelectionCriteria: map[string]any{"AdImageHashes": []string{hash}},
		FieldNames:        []string{"AdImageHash", "Associated"}, Page: Page{Limit: 2},
	})
	if err != nil {
		return "", false, fmt.Errorf("get rollback image: %w", err)
	}

	if len(result.Objects) == 0 {
		return "", false, nil
	}

	var image struct {
		Hash       string `json:"AdImageHash"`
		Associated string `json:"Associated"`
	}
	if err := decodeOne(result.Objects, &image, "rollback image"); err != nil {
		return "", false, err
	}

	if image.Hash != hash || (image.Associated != "YES" && image.Associated != "NO") {
		return "", false, errors.New("rollback image response has unexpected hash or association state")
	}

	return image.Associated, true, nil
}

func (c *Client) DeleteAdImage(ctx context.Context, hash string) error {
	associated, found, err := c.imageForRollback(ctx, hash)
	if err != nil || !found {
		return err
	}

	if associated != "NO" {
		return fmt.Errorf("rollback image %q is associated and cannot be deleted", hash)
	}

	var response imageDeleteResponse
	body := map[string]any{"method": "delete", "params": map[string]any{"SelectionCriteria": map[string]any{"AdImageHashes": []string{hash}}}}
	if err := c.postAt(ctx, c.v501Endpoint(), "adimages", body, &response); err != nil {
		return fmt.Errorf("delete rollback image: %w", err)
	}

	if len(response.Result.DeleteResults) != 1 {
		return errors.New("rollback image deletion did not return exactly one result")
	}

	item := response.Result.DeleteResults[0]
	if len(item.Errors) > 0 {
		return fmt.Errorf("rollback image deletion rejected: code=%d message=%q details=%q", item.Errors[0].Code, item.Errors[0].Message, item.Errors[0].Details)
	}

	if item.Hash != hash {
		return errors.New("rollback image deletion returned an unexpected hash")
	}

	return nil
}

func (c *Client) ExistsForRollback(ctx context.Context, service, objectName string, id int64) (bool, error) {
	if id <= 0 {
		return false, errors.New("rollback resource ID must be positive")
	}

	criteria := map[string]any{"Ids": []int64{id}}
	if service == "bidmodifiers" {
		criteria["Levels"] = []string{"AD_GROUP"}
	}

	result, err := c.getAt(ctx, c.v501Endpoint(), service, objectName, GetOptions{
		SelectionCriteria: criteria, FieldNames: []string{"Id"}, Page: Page{Limit: 2},
	})
	if err != nil {
		return false, fmt.Errorf("verify rollback %s existence: %w", service, err)
	}

	if len(result.Objects) == 0 {
		return false, nil
	}

	var object struct {
		ID int64 `json:"Id"`
	}
	if err := decodeOne(result.Objects, &object, "rollback resource"); err != nil {
		return false, err
	}

	if object.ID != id {
		return false, errors.New("rollback existence query returned an unexpected ID")
	}

	return true, nil
}
