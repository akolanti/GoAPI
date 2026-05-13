//go:build offline

package utils

import (
	"net/http"
	"sync"

	_ "github.com/akolanti/GoAPI/cmd/api/docs"
	"github.com/go-chi/chi/v5"
	httpSwagger "github.com/swaggo/http-swagger"
)

var once sync.Once
var router *chi.Mux

func GetRouter() RouterClient {
	once.Do(func() {
		router = chi.NewRouter()
		router.Get("/swagger", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "/swagger/index.html", http.StatusMovedPermanently)
		})
		router.Get("/swagger/*", httpSwagger.WrapHandler)
	})
	return RouterClient{Router: router}
}
