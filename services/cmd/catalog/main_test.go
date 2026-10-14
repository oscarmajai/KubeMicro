package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetProduct(t *testing.T) {
	mux := newMux()

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/products/kb-01", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var p Product
	if err := json.NewDecoder(rec.Body).Decode(&p); err != nil {
		t.Fatal(err)
	}
	if p.PriceCents != 189900 {
		t.Fatalf("price = %d, want 189900", p.PriceCents)
	}
}

func TestUnknownProduct(t *testing.T) {
	rec := httptest.NewRecorder()
	newMux().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/products/nope", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestListIsSorted(t *testing.T) {
	rec := httptest.NewRecorder()
	newMux().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/products", nil))
	var list []Product
	if err := json.NewDecoder(rec.Body).Decode(&list); err != nil {
		t.Fatal(err)
	}
	if len(list) != len(products) {
		t.Fatalf("len = %d, want %d", len(list), len(products))
	}
	for i := 1; i < len(list); i++ {
		if list[i-1].ID > list[i].ID {
			t.Fatalf("not sorted at %d: %s > %s", i, list[i-1].ID, list[i].ID)
		}
	}
}
