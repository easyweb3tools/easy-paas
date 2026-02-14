package clob

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type Client struct {
	host       string
	httpClient *http.Client
}

type APIError struct {
	Status int
	Body   string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("API error (%d): %s", e.Status, e.Body)
}

func NewClient(httpClient *http.Client, host string) *Client {
	if host == "" {
		host = "https://clob.polymarket.com"
	}
	host = strings.TrimRight(host, "/")
	return &Client{
		host:       host,
		httpClient: httpClient,
	}
}

func (c *Client) doRequest(ctx context.Context, path string, query url.Values) ([]byte, error) {
	fullURL := c.host + path
	if query != nil && len(query) > 0 {
		fullURL = fullURL + "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, &APIError{Status: resp.StatusCode, Body: string(body)}
	}
	return body, nil
}

func (c *Client) GetPrice(ctx context.Context, tokenID, side string) (Decimal, error) {
	if tokenID == "" {
		return Decimal{}, fmt.Errorf("token_id is required")
	}
	query := url.Values{}
	query.Set("token_id", tokenID)
	if side != "" {
		query.Set("side", side)
	}
	body, err := c.doRequest(ctx, "/price", query)
	if err != nil {
		return Decimal{}, err
	}
	return parsePrice(body)
}

func (c *Client) GetBook(ctx context.Context, tokenID string) (*OrderBook, error) {
	if tokenID == "" {
		return nil, fmt.Errorf("token_id is required")
	}
	query := url.Values{}
	query.Set("token_id", tokenID)
	body, err := c.doRequest(ctx, "/book", query)
	if err != nil {
		return nil, err
	}
	return parseOrderBook(body)
}

func (c *Client) GetBookRaw(ctx context.Context, tokenID string) ([]byte, *OrderBook, error) {
	if tokenID == "" {
		return nil, nil, fmt.Errorf("token_id is required")
	}
	query := url.Values{}
	query.Set("token_id", tokenID)
	body, err := c.doRequest(ctx, "/book", query)
	if err != nil {
		return nil, nil, err
	}
	book, err := parseOrderBook(body)
	if err != nil {
		return body, nil, err
	}
	return body, book, nil
}

func (c *Client) GetPriceHistory(ctx context.Context, tokenID, interval string, startTs, endTs *int64) ([]PricePoint, error) {
	if tokenID == "" {
		return nil, fmt.Errorf("token_id is required")
	}
	query := url.Values{}
	query.Set("market", tokenID)
	if interval != "" {
		query.Set("interval", interval)
	}
	if startTs != nil {
		query.Set("startTs", fmt.Sprintf("%d", *startTs))
	}
	if endTs != nil {
		query.Set("endTs", fmt.Sprintf("%d", *endTs))
	}
	body, err := c.doRequest(ctx, "/prices-history", query)
	if err != nil {
		return nil, err
	}
	return parsePriceHistory(body)
}
