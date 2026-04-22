package yclients

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	searchTimeSlotsPath     = "/api/v1/b2c/booking/availability/search-timeslots"
	searchServicesPath      = "/api/v1/b2c/booking/availability/search-services"
	appClientContextVersion = "2"
)

type Client struct {
	BaseURL    string
	Token      string
	HTTPClient *http.Client
}

type Service struct {
	ID       int
	PriceMin float64
}

type SearchTimeSlotsParams struct {
	LocationID int
	Date       string
	ServiceID  int
}

type requestContext struct {
	LocationID int `json:"location_id"`
}

type attendanceServiceItem struct {
	Type string `json:"type"`
	ID   int    `json:"id"`
}

type record struct {
	AttendanceServiceItems []attendanceServiceItem `json:"attendance_service_items"`
}

type timeSlotsRequest struct {
	Context requestContext  `json:"context"`
	Filter  timeSlotsFilter `json:"filter"`
}

type timeSlotsFilter struct {
	Date    string   `json:"date"`
	Records []record `json:"records,omitempty"`
}

type timeSlotsResponse struct {
	Data []timeSlotsItem `json:"data"`
}

type timeSlotsItem struct {
	Attributes timeSlotAttributes `json:"attributes"`
}

type timeSlotAttributes struct {
	DateTime   string `json:"datetime"`
	IsBookable bool   `json:"is_bookable"`
}

type servicesRequest struct {
	Context requestContext `json:"context"`
	Filter  servicesFilter `json:"filter"`
}

type servicesFilter struct {
	Records []record `json:"records"`
}

type servicesResponse struct {
	Data []serviceItem `json:"data"`
}

type serviceItem struct {
	ID         string            `json:"id"`
	Attributes serviceAttributes `json:"attributes"`
}

type serviceAttributes struct {
	IsBookable bool    `json:"is_bookable"`
	PriceMin   float64 `json:"price_min"`
}

func (c *Client) SearchAvailableTimeSlots(ctx context.Context, params SearchTimeSlotsParams) ([]time.Time, error) {
	var response timeSlotsResponse
	if err := c.doJSON(ctx, "search-timeslots", http.MethodPost, searchTimeSlotsPath, makeSearchTimeSlotsRequest(params), &response); err != nil {
		return nil, err
	}

	slots := make([]time.Time, 0, len(response.Data))
	for _, item := range response.Data {
		if !item.Attributes.IsBookable || strings.TrimSpace(item.Attributes.DateTime) == "" {
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

func (c *Client) AvailableServices(ctx context.Context, locationID int) ([]Service, error) {
	var response servicesResponse
	if err := c.doJSON(ctx, "available-services", http.MethodPost, searchServicesPath, makeSearchServicesRequest(locationID), &response); err != nil {
		return nil, err
	}

	services := make([]Service, 0, len(response.Data))
	for _, item := range response.Data {
		if !item.Attributes.IsBookable {
			continue
		}

		serviceID, err := strconv.Atoi(item.ID)
		if err != nil {
			return nil, fmt.Errorf("parse service id %q: %w", item.ID, err)
		}

		services = append(services, Service{
			ID:       serviceID,
			PriceMin: item.Attributes.PriceMin,
		})
	}

	return services, nil
}

func makeSearchTimeSlotsRequest(params SearchTimeSlotsParams) timeSlotsRequest {
	request := timeSlotsRequest{
		Context: requestContext{LocationID: params.LocationID},
		Filter:  timeSlotsFilter{Date: params.Date},
	}

	if params.ServiceID != 0 {
		request.Filter.Records = []record{
			{
				AttendanceServiceItems: []attendanceServiceItem{
					{Type: "service", ID: params.ServiceID},
				},
			},
		}
	}

	return request
}

func makeSearchServicesRequest(locationID int) servicesRequest {
	return servicesRequest{
		Context: requestContext{LocationID: locationID},
		Filter: servicesFilter{
			Records: []record{
				{AttendanceServiceItems: []attendanceServiceItem{}},
			},
		},
	}
}

func (c *Client) doJSON(ctx context.Context, operation string, method string, path string, input any, output any) error {
	if err := c.normalizeBaseURL(); err != nil {
		return err
	}
	if strings.TrimSpace(c.Token) == "" {
		return errors.New("yclients token is required")
	}

	var body io.Reader
	if input != nil {
		payload, err := json.Marshal(input)
		if err != nil {
			return err
		}
		body = bytes.NewReader(payload)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, body)
	if err != nil {
		return err
	}

	if err := c.setCommonHeaders(req); err != nil {
		return err
	}

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("yclients %s failed: status %d: %s", operation, resp.StatusCode, strings.TrimSpace(string(body)))
	}

	if output == nil {
		return nil
	}

	return json.NewDecoder(resp.Body).Decode(output)
}

func (c *Client) setCommonHeaders(req *http.Request) error {
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Content-Type", "application/json")

	analyticsUDID, appClientContext, err := makeAppClientContextHeaders()
	if err != nil {
		return err
	}

	req.Header.Set("X-App-Client-Context-Analytics-Udid", analyticsUDID)
	req.Header.Set("X-App-Client-Context", appClientContext)
	req.Header.Set("X-App-Client-Context-Version", appClientContextVersion)

	return nil
}

func (c *Client) httpClient() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}

	return http.DefaultClient
}

func (c *Client) normalizeBaseURL() error {
	if strings.TrimSpace(c.BaseURL) == "" {
		return errors.New("yclients base url is required")
	}

	c.BaseURL = strings.TrimRight(c.BaseURL, "/")
	return nil
}

func (c *Client) validateSearchTimeSlotsParams(params SearchTimeSlotsParams) error {
	if err := c.validateLocationID(params.LocationID); err != nil {
		return err
	}
	if strings.TrimSpace(params.Date) == "" {
		return errors.New("yclients date is required")
	}

	return nil
}

func (c *Client) validateLocationID(locationID int) error {
	if locationID <= 0 {
		return errors.New("yclients location id must be greater than zero")
	}

	return nil
}

func makeAppClientContextHeaders() (string, string, error) {
	analyticsUDID, err := uuidV4()
	if err != nil {
		return "", "", err
	}
	payload, err := json.Marshal(map[string]any{
		"requestUdid": analyticsUDID,
		"timestamp":   time.Now().Unix(),
	})
	if err != nil {
		return "", "", err
	}

	appClientContext, err := encryptAppClientContext(analyticsUDID, payload)
	if err != nil {
		return "", "", err
	}

	return analyticsUDID, appClientContext, nil
}

func encryptAppClientContext(analyticsUDID string, payload []byte) (string, error) {
	block, err := aes.NewCipher([]byte(analyticsUDID[:32]))
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}

	ciphertextWithTag := gcm.Seal(nil, nonce, payload, nil)
	return base64.StdEncoding.EncodeToString(nonce) + ":" + base64.StdEncoding.EncodeToString(ciphertextWithTag), nil
}

func uuidV4() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}

	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80

	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}
