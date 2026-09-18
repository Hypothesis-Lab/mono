package yandexdirect

import (
	"context"
	"encoding/json"
	"fmt"
)

func ListAll(ctx context.Context, client *Client, service, objectName string, options GetOptions) ([]json.RawMessage, error) {
	const limit = 10_000

	var objects []json.RawMessage
	for offset := 0; ; {
		options.Page = Page{Limit: limit, Offset: offset}
		result, err := client.Get(ctx, service, objectName, options)
		if err != nil {
			return nil, fmt.Errorf("list yandex direct %s: %w", service, err)
		}

		objects = append(objects, result.Objects...)

		if result.LimitedBy == nil {
			return objects, nil
		}

		if *result.LimitedBy <= offset {
			return nil, fmt.Errorf("yandex direct %s page limit did not advance: offset=%d limited_by=%d", service, offset, *result.LimitedBy)
		}

		offset = *result.LimitedBy
	}
}
