package yclients

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const searchTimeSlotsPath = "/api/v1/b2c/booking/availability/search-timeslots"

type Client struct {
	BaseURL    string
	Token      string
	HTTPClient *http.Client
}

type Service struct {
	ID       int
	Title    string
	PriceMin float64
}

type SearchTimeSlotsParams struct {
	LocationID int
	Date       string
}

type searchTimeSlotsRequest struct {
	Context searchTimeSlotsContext `json:"context"`
	Filter  searchTimeSlotsFilter  `json:"filter"`
}

type searchTimeSlotsContext struct {
	LocationID int `json:"location_id"`
}

type searchTimeSlotsFilter struct {
	Date string `json:"date"`
}

type searchTimeSlotsResponse struct {
	Data []searchTimeSlotsResponseItem `json:"data"`
}

type searchTimeSlotsResponseItem struct {
	Attributes searchTimeSlotsAttributes `json:"attributes"`
}

type searchTimeSlotsAttributes struct {
	DateTime   string `json:"datetime"`
	Time       string `json:"time"`
	IsBookable bool   `json:"is_bookable"`
}

type availableServicesResponse struct {
	Services []availableServiceItem `json:"services"`
}

type availableServiceItem struct {
	ID       int     `json:"id"`
	Title    string  `json:"title"`
	PriceMin float64 `json:"price_min"`
}

func (c Client) SearchAvailableTimeSlots(ctx context.Context, params SearchTimeSlotsParams) ([]time.Time, error) {
	requestBody := searchTimeSlotsRequest{
		Context: searchTimeSlotsContext{
			LocationID: params.LocationID,
		},
		Filter: searchTimeSlotsFilter{
			Date: params.Date,
		},
	}

	bodyBytes, err := json.Marshal(requestBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(c.BaseURL, "/")+searchTimeSlotsPath, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Content-Type", "application/json")

	httpClient := c.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("yclients search-timeslots failed: status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var decoded searchTimeSlotsResponse
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return nil, err
	}

	slots := make([]time.Time, 0, len(decoded.Data))
	for _, item := range decoded.Data {
		if !item.Attributes.IsBookable {
			continue
		}

		if strings.TrimSpace(item.Attributes.DateTime) == "" {
			continue
		}

		slotTime, err := time.Parse(time.RFC3339, item.Attributes.DateTime)
		if err != nil {
			return nil, err
		}

		slots = append(slots, slotTime)
	}

	return slots, nil
}

func (c Client) AvailableServices(ctx context.Context, locationID int) ([]Service, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/api/v1/b2c/booking/availability/book_services/%d?without_seances=1", strings.TrimRight(c.BaseURL, "/"), locationID), nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+c.Token)

	httpClient := c.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("yclients available-services failed: status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var decoded availableServicesResponse
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return nil, err
	}

	services := make([]Service, 0, len(decoded.Services))
	for _, item := range decoded.Services {
		services = append(services, Service{
			ID:       item.ID,
			Title:    item.Title,
			PriceMin: item.PriceMin,
		})
	}

	return services, nil
}
