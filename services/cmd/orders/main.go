// orders creates orders priced against the catalog service. Calls to catalog
// have a per-attempt timeout and retry with backoff, so a single slow or
// restarting catalog pod does not fail the order.
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/oscarmajai/kubemicro/services/internal/platform"
)

type product struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	PriceCents int64  `json:"price_cents"`
}

// Order is a priced purchase of one product.
type Order struct {
	ID         string `json:"id"`
	ProductID  string `json:"product_id"`
	Quantity   int    `json:"quantity"`
	TotalCents int64  `json:"total_cents"`
}

var errNotFound = errors.New("product not found")

// catalogClient fetches products with retries on transient failures.
type catalogClient struct {
	baseURL  string
	http     *http.Client
	attempts int
	backoff  time.Duration
}

func (c *catalogClient) product(ctx context.Context, id string) (product, error) {
	var lastErr error
	for attempt := range c.attempts {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return product{}, ctx.Err()
			case <-time.After(c.backoff << (attempt - 1)):
			}
		}
		p, err := c.fetch(ctx, id)
		if err == nil || errors.Is(err, errNotFound) {
			return p, err
		}
		lastErr = err
	}
	return product{}, fmt.Errorf("catalog unavailable after %d attempts: %w", c.attempts, lastErr)
}

func (c *catalogClient) fetch(ctx context.Context, id string) (product, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/products/"+id, nil)
	if err != nil {
		return product{}, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return product{}, err
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusNotFound:
		return product{}, errNotFound
	case resp.StatusCode != http.StatusOK:
		return product{}, fmt.Errorf("catalog returned %d", resp.StatusCode)
	}
	var p product
	return p, json.NewDecoder(resp.Body).Decode(&p)
}

type store struct {
	mu     sync.RWMutex
	orders map[string]Order
}

func newID() string {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func newMux(catalog *catalogClient, st *store) *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /orders", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			ProductID string `json:"product_id"`
			Quantity  int    `json:"quantity"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&in); err != nil {
			platform.WriteError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		if in.ProductID == "" || in.Quantity < 1 || in.Quantity > 100 {
			platform.WriteError(w, http.StatusBadRequest, "product_id is required and quantity must be 1-100")
			return
		}

		p, err := catalog.product(r.Context(), in.ProductID)
		switch {
		case errors.Is(err, errNotFound):
			platform.WriteError(w, http.StatusUnprocessableEntity, "unknown product")
			return
		case err != nil:
			platform.WriteError(w, http.StatusServiceUnavailable, "catalog unavailable, try again")
			return
		}

		o := Order{ID: newID(), ProductID: p.ID, Quantity: in.Quantity, TotalCents: p.PriceCents * int64(in.Quantity)}
		st.mu.Lock()
		st.orders[o.ID] = o
		st.mu.Unlock()
		platform.WriteJSON(w, http.StatusCreated, o)
	})

	mux.HandleFunc("GET /orders/{id}", func(w http.ResponseWriter, r *http.Request) {
		st.mu.RLock()
		o, ok := st.orders[r.PathValue("id")]
		st.mu.RUnlock()
		if !ok {
			platform.WriteError(w, http.StatusNotFound, "order not found")
			return
		}
		platform.WriteJSON(w, http.StatusOK, o)
	})

	return mux
}

func main() {
	log := platform.Logger("orders")
	catalog := &catalogClient{
		baseURL:  platform.Env("CATALOG_URL", "http://catalog"),
		http:     &http.Client{Timeout: 800 * time.Millisecond},
		attempts: 3,
		backoff:  100 * time.Millisecond,
	}
	st := &store{orders: map[string]Order{}}
	srv := &platform.Server{Name: "orders", Addr: ":" + platform.Env("PORT", "8080"), Log: log}
	if err := srv.Run(newMux(catalog, st)); err != nil {
		log.Error("server failed", "err", err)
		os.Exit(1)
	}
}
