package yclients

import (
	"context"
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
					"Authorization": "Bearer test-token",
					"Content-Type":  "application/json",
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
	client := Client{
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

func TestAvailableServices(t *testing.T) {
	client := Client{
		BaseURL: "https://platform.yclients.com",
		Token:   "test-token",
		HTTPClient: &http.Client{
			Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				if r.Method != http.MethodGet {
					t.Fatalf("method = %q, want %q", r.Method, http.MethodGet)
				}

				if r.URL.Path != "/api/v1/b2c/booking/availability/book_services/1296020" {
					t.Fatalf("path = %q, want %q", r.URL.Path, "/api/v1/b2c/booking/availability/book_services/1296020")
				}

				if got := r.URL.Query().Get("without_seances"); got != "1" {
					t.Fatalf("without_seances = %q, want %q", got, "1")
				}

				expectedHeaders := map[string]string{
					"Authorization": "Bearer test-token",
				}

				for name, want := range expectedHeaders {
					if got := r.Header.Get(name); got != want {
						t.Fatalf("header %s = %q, want %q", name, got, want)
					}
				}

				return jsonResponse(r, http.StatusOK, `{"events":[],"services":[{"id":19432008,"title":"Падел 2 корт 2 часа вт-вс","price_min":4800.0,"price_max":4800.0},{"id":19346628,"title":"Падел корт 1 вт-вс 1 час","price_min":2400.0,"price_max":2400.0},{"id":27138069,"title":"Акция в честь 8 марта 1500руб 1 корт","price_min":1500.0,"price_max":1500.0}],"category":[],"category_groups":[]}`), nil
			}),
		},
	}

	gotServices, err := client.AvailableServices(context.Background(), 1296020)
	if err != nil {
		t.Fatalf("AvailableServices returned error: %v", err)
	}

	wantServices := []Service{
		{ID: 19432008, Title: "Падел 2 корт 2 часа вт-вс", PriceMin: 4800.0},
		{ID: 19346628, Title: "Падел корт 1 вт-вс 1 час", PriceMin: 2400.0},
		{ID: 27138069, Title: "Акция в честь 8 марта 1500руб 1 корт", PriceMin: 1500.0},
	}
	if !reflect.DeepEqual(gotServices, wantServices) {
		t.Fatalf("services = %#v, want %#v", gotServices, wantServices)
	}
}

func TestAvailableServicesReturnsErrorForBadStatus(t *testing.T) {
	client := Client{
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
