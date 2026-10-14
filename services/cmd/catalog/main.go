// catalog serves the product catalog that orders prices against.
package main

import (
	"net/http"
	"os"
	"slices"
	"strings"

	"github.com/oscarmajai/kubemicro/services/internal/platform"
)

// Product is a sellable item. Prices are in cents to avoid float rounding.
type Product struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	PriceCents int64  `json:"price_cents"`
}

var products = map[string]Product{
	"kb-01":  {ID: "kb-01", Name: "Mechanical keyboard", PriceCents: 189900},
	"ms-01":  {ID: "ms-01", Name: "Wireless mouse", PriceCents: 64900},
	"mn-27":  {ID: "mn-27", Name: "27\" monitor", PriceCents: 529900},
	"hd-usb": {ID: "hd-usb", Name: "USB-C hub", PriceCents: 79900},
}

func newMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /products", func(w http.ResponseWriter, _ *http.Request) {
		list := make([]Product, 0, len(products))
		for _, p := range products {
			list = append(list, p)
		}
		slices.SortFunc(list, func(a, b Product) int { return strings.Compare(a.ID, b.ID) })
		platform.WriteJSON(w, http.StatusOK, list)
	})
	mux.HandleFunc("GET /products/{id}", func(w http.ResponseWriter, r *http.Request) {
		p, ok := products[r.PathValue("id")]
		if !ok {
			platform.WriteError(w, http.StatusNotFound, "product not found")
			return
		}
		platform.WriteJSON(w, http.StatusOK, p)
	})
	return mux
}

func main() {
	log := platform.Logger("catalog")
	srv := &platform.Server{Name: "catalog", Addr: ":" + platform.Env("PORT", "8080"), Log: log}
	if err := srv.Run(newMux()); err != nil {
		log.Error("server failed", "err", err)
		os.Exit(1)
	}
}
