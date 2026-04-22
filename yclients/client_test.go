package yclients

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestSearchAvailableTimeSlots(t *testing.T) {
	client := Client{
		BaseURL: "https://platform.yclients.com",
		Token:   "test-token",
		HTTPClient: &http.Client{
			Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				if r.Method != http.MethodPost {
					t.Fatalf("method = %q, want %q", r.Method, http.MethodPost)
				}

				if r.URL.Path != "/api/v1/b2c/booking/availability/search-timeslots" {
					t.Fatalf("path = %q, want %q", r.URL.Path, "/api/v1/b2c/booking/availability/search-timeslots")
				}

				expectedHeaders := map[string]string{
					"Authorization":                "Bearer test-token",
					"Content-Type":                 "application/json",
					"X-App-Client-Context-Version": "2",
				}

				for name, want := range expectedHeaders {
					if got := r.Header.Get(name); got != want {
						t.Fatalf("header %s = %q, want %q", name, got, want)
					}
				}

				analyticsUDID := r.Header.Get("X-App-Client-Context-Analytics-Udid")
				if len(analyticsUDID) != 36 {
					t.Fatalf("analytics UDID length = %d, want 36", len(analyticsUDID))
				}

				appClientContext := r.Header.Get("X-App-Client-Context")
				if strings.TrimSpace(appClientContext) == "" {
					t.Fatal("X-App-Client-Context header is empty")
				}

				payload, err := decodeAppClientContext(analyticsUDID, appClientContext)
				if err != nil {
					t.Fatalf("decodeAppClientContext returned error: %v", err)
				}

				if requestUDID, ok := payload["requestUdid"].(string); !ok || len(requestUDID) != 36 {
					t.Fatalf("requestUdid = %#v, want UUID string", payload["requestUdid"])
				}

				timestamp, ok := payload["timestamp"].(float64)
				if !ok || timestamp <= 0 {
					t.Fatalf("timestamp = %#v, want positive unix timestamp", payload["timestamp"])
				}

				var gotBody map[string]any
				if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
					t.Fatalf("Decode request body returned error: %v", err)
				}

				wantBody := map[string]any{
					"context": map[string]any{
						"location_id": float64(1296020),
					},
					"filter": map[string]any{
						"date": "2026-03-18",
					},
				}

				if !reflect.DeepEqual(gotBody, wantBody) {
					t.Fatalf("request body = %#v, want %#v", gotBody, wantBody)
				}

				return jsonResponse(r, http.StatusOK, `{"data":[{"type":"booking_search_result_timeslots","id":"a","attributes":{"datetime":"2026-03-18T18:00:00+03:00","time":"18:00","is_bookable":true}},{"type":"booking_search_result_timeslots","id":"b","attributes":{"datetime":"2026-03-18T19:00:00+03:00","time":"19:00","is_bookable":false}},{"type":"booking_search_result_timeslots","id":"c","attributes":{"datetime":"2026-03-18T21:00:00+03:00","time":"21:00","is_bookable":true}}]}`), nil
			}),
		},
	}

	gotSlots, err := client.SearchAvailableTimeSlots(context.Background(), SearchTimeSlotsParams{
		LocationID: 1296020,
		Date:       "2026-03-18",
	})
	if err != nil {
		t.Fatalf("SearchAvailableTimeSlots returned error: %v", err)
	}

	first, err := time.Parse(time.RFC3339, "2026-03-18T18:00:00+03:00")
	if err != nil {
		t.Fatalf("Parse first returned error: %v", err)
	}

	second, err := time.Parse(time.RFC3339, "2026-03-18T21:00:00+03:00")
	if err != nil {
		t.Fatalf("Parse second returned error: %v", err)
	}

	wantSlots := []time.Time{first, second}
	if !reflect.DeepEqual(gotSlots, wantSlots) {
		t.Fatalf("slots = %#v, want %#v", gotSlots, wantSlots)
	}
}

func TestSearchAvailableTimeSlotsReturnsErrorForBadStatus(t *testing.T) {
	client := &Client{
		BaseURL: "https://platform.yclients.com",
		Token:   "test-token",
		HTTPClient: &http.Client{
			Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				return jsonResponse(r, http.StatusBadGateway, `{"error":"bad gateway"}`), nil
			}),
		},
	}

	_, err := client.SearchAvailableTimeSlots(context.Background(), SearchTimeSlotsParams{
		LocationID: 1296020,
		Date:       "2026-03-18",
	})
	if err == nil {
		t.Fatal("SearchAvailableTimeSlots returned nil error, want non-nil")
	}
}

func TestSearchAvailableTimeSlotsReturnsErrorForInvalidParams(t *testing.T) {
	client := &Client{
		BaseURL: "https://platform.yclients.com",
		Token:   "test-token",
	}

	_, err := client.SearchAvailableTimeSlots(context.Background(), SearchTimeSlotsParams{
		LocationID: 1296020,
	})
	if err == nil {
		t.Fatal("SearchAvailableTimeSlots returned nil error, want non-nil")
	}
}

func TestSearchAvailableTimeSlotsWithServiceFilter(t *testing.T) {
	client := &Client{
		BaseURL: "https://platform.yclients.com",
		Token:   "test-token",
		HTTPClient: &http.Client{
			Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				var gotBody map[string]any
				if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
					t.Fatalf("Decode request body returned error: %v", err)
				}

				wantBody := map[string]any{
					"context": map[string]any{
						"location_id": float64(1296020),
					},
					"filter": map[string]any{
						"date": "2026-03-18",
						"records": []any{
							map[string]any{
								"attendance_service_items": []any{
									map[string]any{
										"type": "service",
										"id":   float64(19432008),
									},
								},
							},
						},
					},
				}

				if !reflect.DeepEqual(gotBody, wantBody) {
					t.Fatalf("request body = %#v, want %#v", gotBody, wantBody)
				}

				return jsonResponse(r, http.StatusOK, `{"data":[{"type":"booking_search_result_timeslots","id":"a","attributes":{"datetime":"2026-03-18T18:00:00+03:00","time":"18:00","is_bookable":true}}]}`), nil
			}),
		},
	}

	gotSlots, err := client.SearchAvailableTimeSlots(context.Background(), SearchTimeSlotsParams{
		LocationID: 1296020,
		Date:       "2026-03-18",
		ServiceID:  19432008,
	})
	if err != nil {
		t.Fatalf("SearchAvailableTimeSlots returned error: %v", err)
	}

	if len(gotSlots) != 1 {
		t.Fatalf("slots length = %d, want %d", len(gotSlots), 1)
	}
}

func TestAvailableServices(t *testing.T) {
	client := &Client{
		BaseURL: "https://platform.yclients.com",
		Token:   "test-token",
		HTTPClient: &http.Client{
			Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				if r.Method != http.MethodPost {
					t.Fatalf("method = %q, want %q", r.Method, http.MethodPost)
				}

				if r.URL.Path != "/api/v1/b2c/booking/availability/search-services" {
					t.Fatalf("path = %q, want %q", r.URL.Path, "/api/v1/b2c/booking/availability/search-services")
				}

				expectedHeaders := map[string]string{
					"Authorization":                "Bearer test-token",
					"Content-Type":                 "application/json",
					"X-App-Client-Context-Version": "2",
				}

				for name, want := range expectedHeaders {
					if got := r.Header.Get(name); got != want {
						t.Fatalf("header %s = %q, want %q", name, got, want)
					}
				}

				var gotBody map[string]any
				if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
					t.Fatalf("Decode request body returned error: %v", err)
				}

				wantBody := map[string]any{
					"context": map[string]any{
						"location_id": float64(1296020),
					},
					"filter": map[string]any{
						"records": []any{
							map[string]any{
								"attendance_service_items": []any{},
							},
						},
					},
				}

				if !reflect.DeepEqual(gotBody, wantBody) {
					t.Fatalf("request body = %#v, want %#v", gotBody, wantBody)
				}

				return jsonResponse(r, http.StatusOK, `{"data":[{"type":"booking_search_result_services","id":"19432008","attributes":{"is_bookable":true,"price_min":5000.0}},{"type":"booking_search_result_services","id":"19346628","attributes":{"is_bookable":true,"price_min":2600.0}},{"type":"booking_search_result_services","id":"27138069","attributes":{"is_bookable":false,"price_min":1500.0}}]}`), nil
			}),
		},
	}

	gotServices, err := client.AvailableServices(context.Background(), 1296020)
	if err != nil {
		t.Fatalf("AvailableServices returned error: %v", err)
	}

	wantServices := []Service{
		{ID: 19432008, PriceMin: 5000.0},
		{ID: 19346628, PriceMin: 2600.0},
	}
	if !reflect.DeepEqual(gotServices, wantServices) {
		t.Fatalf("services = %#v, want %#v", gotServices, wantServices)
	}
}

func TestAvailableServicesReturnsErrorForBadStatus(t *testing.T) {
	client := &Client{
		BaseURL: "https://platform.yclients.com",
		Token:   "test-token",
		HTTPClient: &http.Client{
			Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				return jsonResponse(r, http.StatusBadGateway, `{"error":"bad gateway"}`), nil
			}),
		},
	}

	_, err := client.AvailableServices(context.Background(), 1296020)
	if err == nil {
		t.Fatal("AvailableServices returned nil error, want non-nil")
	}
}

func TestAvailableServicesReturnsErrorForInvalidLocationID(t *testing.T) {
	client := &Client{
		BaseURL: "https://platform.yclients.com",
		Token:   "test-token",
	}

	_, err := client.AvailableServices(context.Background(), 0)
	if err == nil {
		t.Fatal("AvailableServices returned nil error, want non-nil")
	}
}

func TestAvailableServicesReturnsErrorForInvalidServiceID(t *testing.T) {
	client := &Client{
		BaseURL: "https://platform.yclients.com",
		Token:   "test-token",
		HTTPClient: &http.Client{
			Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				return jsonResponse(r, http.StatusOK, `{"data":[{"type":"booking_search_result_services","id":"not-a-number","attributes":{"is_bookable":true,"price_min":5000.0}}]}`), nil
			}),
		},
	}

	_, err := client.AvailableServices(context.Background(), 1296020)
	if err == nil {
		t.Fatal("AvailableServices returned nil error, want non-nil")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func jsonResponse(req *http.Request, statusCode int, body string) *http.Response {
	return &http.Response{
		StatusCode: statusCode,
		Header: http.Header{
			"Content-Type": []string{"application/json"},
		},
		Body:    io.NopCloser(strings.NewReader(body)),
		Request: req,
	}
}

func decodeAppClientContext(analyticsUDID string, appClientContext string) (map[string]any, error) {
	parts := strings.Split(appClientContext, ":")
	if len(parts) != 2 {
		return nil, io.ErrUnexpectedEOF
	}

	nonce, err := base64.StdEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, err
	}

	ciphertext, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, err
	}

	block, err := aes.NewCipher([]byte(analyticsUDID[:32]))
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	plain, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}

	var payload map[string]any
	if err := json.Unmarshal(plain, &payload); err != nil {
		return nil, err
	}

	return payload, nil
}
