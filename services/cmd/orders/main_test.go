package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// fakeCatalog fails the first `failures` requests with 503, then serves kb-01.
func fakeCatalog(t *testing.T, failures int32) (*catalogClient, *atomic.Int32) {
	t.Helper()
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) <= failures {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		if r.URL.Path != "/products/kb-01" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_ = json.NewEncoder(w).Encode(product{ID: "kb-01", Name: "Keyboard", PriceCents: 189900})
	}))
	t.Cleanup(srv.Close)
	return &catalogClient{baseURL: srv.URL, http: srv.Client(), attempts: 3, backoff: time.Millisecond}, &calls
}

func post(mux *http.ServeMux, body string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/orders", strings.NewReader(body)))
	return rec
}

func TestCreateAndGetOrder(t *testing.T) {
	catalog, _ := fakeCatalog(t, 0)
	mux := newMux(catalog, &store{orders: map[string]Order{}})

	rec := post(mux, `{"product_id":"kb-01","quantity":2}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201: %s", rec.Code, rec.Body)
	}
	var o Order
	if err := json.NewDecoder(rec.Body).Decode(&o); err != nil {
		t.Fatal(err)
	}
	if o.TotalCents != 379800 {
		t.Fatalf("total = %d, want 379800", o.TotalCents)
	}

	get := httptest.NewRecorder()
	mux.ServeHTTP(get, httptest.NewRequest(http.MethodGet, "/orders/"+o.ID, nil))
	if get.Code != http.StatusOK {
		t.Fatalf("get status = %d, want 200", get.Code)
	}
}

func TestRetriesTransientCatalogFailures(t *testing.T) {
	catalog, calls := fakeCatalog(t, 2)
	rec := post(newMux(catalog, &store{orders: map[string]Order{}}), `{"product_id":"kb-01","quantity":1}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201 after retries", rec.Code)
	}
	if got := calls.Load(); got != 3 {
		t.Fatalf("catalog calls = %d, want 3", got)
	}
}

func TestCatalogDownReturns503(t *testing.T) {
	catalog, calls := fakeCatalog(t, 99)
	rec := post(newMux(catalog, &store{orders: map[string]Order{}}), `{"product_id":"kb-01","quantity":1}`)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
	if got := calls.Load(); got != 3 {
		t.Fatalf("catalog calls = %d, want 3 (no retry storm)", got)
	}
}

func TestUnknownProductIsNotRetried(t *testing.T) {
	catalog, calls := fakeCatalog(t, 0)
	rec := post(newMux(catalog, &store{orders: map[string]Order{}}), `{"product_id":"nope","quantity":1}`)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", rec.Code)
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("catalog calls = %d, want 1", got)
	}
}

func TestValidation(t *testing.T) {
	catalog, _ := fakeCatalog(t, 0)
	mux := newMux(catalog, &store{orders: map[string]Order{}})
	for _, body := range []string{`{`, `{"quantity":1}`, `{"product_id":"kb-01","quantity":0}`, `{"product_id":"kb-01","quantity":101}`} {
		if rec := post(mux, body); rec.Code != http.StatusBadRequest {
			t.Errorf("body %s: status = %d, want 400", body, rec.Code)
		}
	}
}
