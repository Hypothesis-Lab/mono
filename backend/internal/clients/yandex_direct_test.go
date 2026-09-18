package clients

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestYandexDirectClientCreateCampaign(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "Bearer token", r.Header.Get("Authorization"))
		assert.Equal(t, "en", r.Header.Get("Accept-Language"))

		var request yandexCampaignAddRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&request))
		assert.Equal(t, "add", request.Method)
		require.Len(t, request.Params.Campaigns, 1)

		campaign := request.Params.Campaigns[0]
		assert.Equal(t, "PoC campaign", campaign.Name)
		assert.Equal(t, "2026-09-10", campaign.StartDate)
		assert.Equal(t, "SERVING_OFF", campaign.CpmBannerCampaign.BiddingStrategy.Search.BiddingStrategyType)
		assert.Equal(t, "MANUAL_CPM", campaign.CpmBannerCampaign.BiddingStrategy.Network.BiddingStrategyType)

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"result":{"AddResults":[{"Id":12345}]}}`))
	}))
	defer server.Close()

	client := NewYandexDirectClient("token")
	client.endpoint = server.URL

	id, err := client.CreateCampaign(context.Background(), "PoC campaign", "2026-09-10")
	require.NoError(t, err)
	assert.Equal(t, int64(12345), id)
}

func TestYandexDirectClientCreateCampaignReturnsItemError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"result":{"AddResults":[{"Errors":[{"Code":8800,"Message":"Object not found","Details":"Invalid account"}]}]}}`))
	}))
	defer server.Close()

	client := NewYandexDirectClient("token")
	client.endpoint = server.URL

	_, err := client.CreateCampaign(context.Background(), "PoC campaign", "2026-09-10")
	require.EqualError(t, err, `yandex direct campaign creation failed: code=8800 message="Object not found" details="Invalid account"`)
}
