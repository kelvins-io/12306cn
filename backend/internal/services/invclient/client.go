package invclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/kelvins-io/12306cn/backend/internal/services/inventory"
)

// Client talks to inventory microservice.
type Client struct {
	BaseURL string
	HTTP    *http.Client
	Local   *inventory.Service // fallback when BaseURL empty
}

func New(baseURL string, local *inventory.Service) *Client {
	return &Client{
		BaseURL: baseURL,
		HTTP:    &http.Client{Timeout: 15 * time.Second},
		Local:   local,
	}
}

func (c *Client) remote() bool {
	return c != nil && c.BaseURL != ""
}

func (c *Client) EnsureDaily(ctx context.Context, trainID uint, date string) error {
	if !c.remote() {
		return c.Local.EnsureDaily(trainID, date)
	}
	body, _ := json.Marshal(map[string]interface{}{"train_id": trainID, "travel_date": date})
	return c.post(ctx, "/internal/v1/ensure", body, nil)
}

func (c *Client) Occupy(ctx context.Context, req inventory.OccupyRequest) ([]inventory.SeatPickDTO, error) {
	if !c.remote() {
		return c.Local.OccupyAndPublish(ctx, req)
	}
	var out struct {
		Code    int                     `json:"code"`
		Message string                  `json:"message"`
		Data    []inventory.SeatPickDTO `json:"data"`
	}
	raw, _ := json.Marshal(req)
	if err := c.post(ctx, "/internal/v1/occupy", raw, &out); err != nil {
		return nil, err
	}
	if out.Code != 0 {
		return nil, fmt.Errorf("%s", out.Message)
	}
	return out.Data, nil
}

func (c *Client) Release(ctx context.Context, req inventory.ReleaseRequest) error {
	if !c.remote() {
		return c.Local.ReleaseAndPublish(ctx, req)
	}
	var out struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	raw, _ := json.Marshal(req)
	if err := c.post(ctx, "/internal/v1/release", raw, &out); err != nil {
		return err
	}
	if out.Code != 0 {
		return fmt.Errorf("%s", out.Message)
	}
	return nil
}

func (c *Client) Remaining(ctx context.Context, trainID uint, date, seatType string, fromSeq, toSeq int) (int, error) {
	if !c.remote() {
		return c.Local.RemainingByType(trainID, date, seatType, fromSeq, toSeq)
	}
	url := fmt.Sprintf("%s/internal/v1/remaining?train_id=%d&travel_date=%s&seat_type=%s&from_seq=%d&to_seq=%d",
		c.BaseURL, trainID, date, seatType, fromSeq, toSeq)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, err
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	var out struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			Remaining int `json:"remaining"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return 0, err
	}
	if out.Code != 0 {
		return 0, fmt.Errorf("%s", out.Message)
	}
	return out.Data.Remaining, nil
}

func (c *Client) post(ctx context.Context, path string, body []byte, dest interface{}) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if dest == nil {
		var tmp struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		}
		_ = json.Unmarshal(raw, &tmp)
		if tmp.Code != 0 {
			return fmt.Errorf("%s", tmp.Message)
		}
		return nil
	}
	return json.Unmarshal(raw, dest)
}
