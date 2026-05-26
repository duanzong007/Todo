package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const externalQuotePath = "/external/quote"

type QuoteService struct {
	endpoint string
	secret   string
	client   *http.Client
}

type Quote struct {
	Text   string
	Author string
	Source string
}

type externalQuoteResponse struct {
	Data   *Quote          `json:"data"`
	Error  json.RawMessage `json:"error"`
	Text   string          `json:"text"`
	Author string          `json:"author"`
	Source string          `json:"source"`
}

func NewQuoteService(rawEndpoint, secret string) (*QuoteService, error) {
	endpoint, err := normalizeQuoteEndpoint(rawEndpoint)
	if err != nil {
		return nil, err
	}
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return nil, fmt.Errorf("external quote secret is empty")
	}

	return &QuoteService{
		endpoint: endpoint,
		secret:   secret,
		client: &http.Client{
			Timeout: 3 * time.Second,
		},
	}, nil
}

func (s *QuoteService) Random(ctx context.Context) (Quote, error) {
	if s == nil || s.client == nil || s.endpoint == "" || s.secret == "" {
		return Quote{}, nil
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, s.endpoint, nil)
	if err != nil {
		return Quote{}, fmt.Errorf("build quote request: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+s.secret)
	request.Header.Set("Accept", "application/json")

	response, err := s.client.Do(request)
	if err != nil {
		return Quote{}, fmt.Errorf("request external quote: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode == http.StatusNoContent || response.StatusCode == http.StatusNotFound {
		return Quote{}, nil
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return Quote{}, fmt.Errorf("external quote status: %s", response.Status)
	}

	var payload externalQuoteResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return Quote{}, fmt.Errorf("decode external quote: %w", err)
	}

	quote := Quote{
		Text:   payload.Text,
		Author: payload.Author,
		Source: payload.Source,
	}
	if payload.Data != nil {
		quote = *payload.Data
	}

	quote.Text = strings.TrimSpace(quote.Text)
	quote.Author = strings.TrimSpace(quote.Author)
	quote.Source = strings.TrimSpace(quote.Source)
	if quote.Text == "" {
		return Quote{}, nil
	}
	return quote, nil
}

func (s *QuoteService) Check(ctx context.Context) (bool, error) {
	quote, err := s.Random(ctx)
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(quote.Text) != "", nil
}

func (s *QuoteService) Endpoint() string {
	if s == nil {
		return ""
	}
	return s.endpoint
}

func normalizeQuoteEndpoint(rawEndpoint string) (string, error) {
	trimmed := strings.TrimSpace(rawEndpoint)
	if trimmed == "" {
		return "", fmt.Errorf("external quote url is empty")
	}

	parsed, err := url.Parse(trimmed)
	if err != nil {
		return "", fmt.Errorf("parse external quote url: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("external quote url must use http or https")
	}
	if parsed.Host == "" {
		return "", fmt.Errorf("external quote url host is empty")
	}
	if parsed.Path == "" || parsed.Path == "/" {
		parsed.Path = externalQuotePath
	}
	return parsed.String(), nil
}
