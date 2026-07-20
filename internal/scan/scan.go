// Package scan handles receipt/struk photo OCR via OpenAI vision.
package scan

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Result is what the model extracts from a receipt photo.
type Result struct {
	Item      string  `json:"item"`
	Store     string  `json:"store"`
	Currency  string  `json:"currency"`
	Amount    float64 `json:"amount"`
	PhotoPath string  `json:"-"` // server-set path to the saved receipt image
}

// Service calls OpenAI vision.
type Service struct {
	apiKey string
	client *http.Client
}

// New constructs a Service. If apiKey is empty, Extract returns ErrDisabled.
func New(apiKey string) *Service {
	return &Service{
		apiKey: apiKey,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

// ErrDisabled means no API key was configured.
var ErrDisabled = errors.New("scan disabled: OPENAI_API_KEY not set")

// Extract sends the image to gpt-4o-mini and parses the structured reply.
func (s *Service) Extract(ctx context.Context, imageBytes []byte, contentType string) (Result, error) {
	if s.apiKey == "" {
		return Result{}, ErrDisabled
	}
	if len(imageBytes) == 0 {
		return Result{}, errors.New("gambar kosong")
	}
	if contentType == "" {
		contentType = "image/jpeg"
	}

	dataURL := "data:" + contentType + ";base64," + base64.StdEncoding.EncodeToString(imageBytes)

	payload := chatRequest{
		Model: "gpt-4o-mini",
		Messages: []message{{
			Role: "user",
			Content: []content{{
				Type: "text",
				Text: "Baca struk/receipt ini. Ambil SATU item utama (nama produk), nama toko, mata uang (ISO 4217, mis. JPY/KRW/USD/SGD/THB/HKD/IDR), dan harga satuan dalam mata uang tersebut. Jawab HANYA JSON valid: {\"item\": string, \"store\": string, \"currency\": string, \"amount\": number}. Tidak ada penjelasan.",
			}, {
				Type:     "image_url",
				ImageURL: &imageURL{URL: dataURL},
			}},
		}},
		ResponseFormat: &responseFormat{Type: "json_object"},
		Temperature:    0,
		MaxTokens:      300,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return Result{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://api.openai.com/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return Result{}, err
	}
	req.Header.Set("Authorization", "Bearer "+s.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return Result{}, fmt.Errorf("openai request: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if resp.StatusCode != http.StatusOK {
		return Result{}, fmt.Errorf("openai HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	var cr chatResponse
	if err := json.Unmarshal(respBody, &cr); err != nil {
		return Result{}, err
	}
	if len(cr.Choices) == 0 {
		return Result{}, errors.New("openai: no choices")
	}
	raw := strings.TrimSpace(cr.Choices[0].Message.Content)
	// Strip code fences if present.
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)

	var r Result
	if err := json.Unmarshal([]byte(raw), &r); err != nil {
		return Result{}, fmt.Errorf("parse openai json: %w (raw=%q)", err, raw)
	}
	r.Currency = strings.ToUpper(strings.TrimSpace(r.Currency))
	return r, nil
}

// --- OpenAI request/response shapes ---

type content struct {
	Type     string    `json:"type"`
	Text     string    `json:"text,omitempty"`
	ImageURL *imageURL `json:"image_url,omitempty"`
}
type imageURL struct {
	URL string `json:"url"`
}
type message struct {
	Role    string `json:"role"`
	Content any    `json:"content"`
}
type responseFormat struct {
	Type string `json:"type"`
}
type chatRequest struct {
	Model          string          `json:"model"`
	Messages       []message       `json:"messages"`
	ResponseFormat *responseFormat `json:"response_format,omitempty"`
	Temperature    float64         `json:"temperature,omitempty"`
	MaxTokens      int             `json:"max_tokens,omitempty"`
}
type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}
