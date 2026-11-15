package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"net/url"

	"github.com/jackc/pgx/v5"
)

type pilotAd struct {
	Title       string `json:"title"`
	Text        string `json:"text"`
	ImageBase64 string `json:"image_base64"`
}

type pilotAudience struct {
	Rank    int       `json:"rank"`
	Keyword string    `json:"keyword"`
	Ads     []pilotAd `json:"ads"`
}

type pilotPreparation struct {
	Format     string          `json:"format"`
	LandingURL string          `json:"landing_url"`
	Audiences  []pilotAudience `json:"audiences"`
}

func draftPilotConfig(ctx context.Context, tx pgx.Tx, sourceProductID string, audienceCount int, landingURL string) ([]byte, error) {
	if landingURL == "" {
		return nil, nil
	}

	if err := validateDraftLandingURL(landingURL); err != nil {
		return nil, err
	}

	if audienceCount != 0 && audienceCount != 3 {
		return nil, errors.New("draft pilot requires exactly three source audiences or --pilot-audience with an empty source")
	}

	var name string
	if err := tx.QueryRow(ctx, "SELECT name FROM product WHERE id=$1", sourceProductID).Scan(&name); err != nil {
		return nil, fmt.Errorf("get draft pilot source product: %w", err)
	}

	if name != "Tempo" {
		return nil, errors.New("synthetic draft creative fixture is specific to the Tempo product")
	}

	config := pilotPreparation{Format: "unified_draft", LandingURL: landingURL}
	keywords := []string{"расписание смен кафе", "график персонала кафе", "учёт рабочего времени кафе"}
	copyByAudience := [3][3]struct{ title, text string }{
		{
			{"Tempo: график смен", "Составьте расписание команды кафе. Тестовый вариант."},
			{"Tempo: план недели", "Подготовьте график работы кафе на неделю. Тестовый вариант."},
			{"Tempo: смены команды", "Соберите смены сотрудников в одном месте. Тестовый вариант."},
		},
		{
			{"Tempo: расписание кафе", "Создайте расписание для команды кафе. Тестовый вариант."},
			{"Tempo: рабочая неделя", "Распределите рабочие смены на неделю. Тестовый вариант."},
			{"Tempo: публикация графика", "Подготовьте и опубликуйте график команды. Тестовый вариант."},
		},
		{
			{"Tempo: смены на неделю", "Запланируйте смены команды вашего кафе. Тестовый вариант."},
			{"Tempo: график команды", "Соберите рабочее расписание сотрудников. Тестовый вариант."},
			{"Tempo: план работы кафе", "Создайте план смен и поделитесь с командой. Тестовый вариант."},
		},
	}
	for rank, keyword := range keywords {
		audience := pilotAudience{Rank: rank + 1, Keyword: keyword}
		for variant, creativeCopy := range copyByAudience[rank] {
			data, err := pilotCalendarPNG(rank, variant)
			if err != nil {
				return nil, fmt.Errorf("encode pilot image for audience %d variant %d: %w", rank+1, variant+1, err)
			}

			audience.Ads = append(audience.Ads, pilotAd{Title: creativeCopy.title, Text: creativeCopy.text, ImageBase64: base64.StdEncoding.EncodeToString(data)})
		}

		config.Audiences = append(config.Audiences, audience)
	}

	encoded, err := json.Marshal(config)
	if err != nil {
		return nil, fmt.Errorf("encode synthetic draft pilot configuration: %w", err)
	}

	return encoded, nil
}

func validateDraftLandingURL(value string) error {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Hostname() == "" || (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.User != nil || len(value) > 1024 {
		return errors.New("draft pilot landing URL must be an absolute http(s) URL without credentials, at most 1024 bytes")
	}

	return nil
}

func pilotCalendarPNG(audience, variant int) ([]byte, error) {
	canvas := image.NewRGBA(image.Rect(0, 0, 600, 600))
	backgrounds := []color.RGBA{{24, 65, 105, 255}, {40, 92, 68, 255}, {88, 47, 110, 255}}
	accents := []color.RGBA{{245, 160, 70, 255}, {245, 185, 70, 255}, {245, 210, 70, 255}}
	draw.Draw(canvas, canvas.Bounds(), image.NewUniform(backgrounds[audience]), image.Point{}, draw.Src)
	draw.Draw(canvas, image.Rect(40, 40, 560, 110), image.NewUniform(color.RGBA{240, 245, 250, 255}), image.Point{}, draw.Src)
	for row := range 4 {
		for column := range 7 {
			shade := color.RGBA{220, 230, 240, 255}
			if (row+column+variant)%3 == 0 {
				shade = accents[variant]
			}

			cell := image.Rect(40+column*75, 145+row*98, 100+column*75, 222+row*98)
			draw.Draw(canvas, cell, image.NewUniform(shade), image.Point{}, draw.Src)
		}
	}

	var buffer bytes.Buffer
	if err := png.Encode(&buffer, canvas); err != nil {
		return nil, fmt.Errorf("encode synthetic calendar PNG: %w", err)
	}

	return buffer.Bytes(), nil
}
