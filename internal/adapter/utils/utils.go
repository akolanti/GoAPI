package utils

import (
	"net/http"
	"sync"

	_ "github.com/akolanti/GoAPI/cmd/api/docs"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/swaggo/http-swagger"
)

var once sync.Once
var router *chi.Mux

func GetNewUUID() string {
	return uuid.New().String()
}

type RouterClient struct {
	Router *chi.Mux
}

func GetChiURLParam(request *http.Request, key string) string {
	return chi.URLParam(request, key)
}

func GetRouter() RouterClient {
	once.Do(func() {
		router = chi.NewRouter()
		InitSwagger(router)
		//register prometheus
		router.Handle("/metrics", promhttp.Handler())
	})

	return RouterClient{Router: router}
}

func InitSwagger(r *chi.Mux) {
	r.Get("/swagger", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/swagger/index.html", http.StatusMovedPermanently)
	})
	r.Get("/swagger/*", httpSwagger.WrapHandler)
}

func ReverseStringArray(array []string) []string {
	for i, j := 0, len(array)-1; i < j; i, j = i+1, j-1 {
		array[i], array[j] = array[j], array[i]
	}
	return array
}
