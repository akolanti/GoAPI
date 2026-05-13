package utils

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func GetNewUUID() string {
	return uuid.New().String()
}

type RouterClient struct {
	Router *chi.Mux
}

func GetChiURLParam(request *http.Request, key string) string {
	return chi.URLParam(request, key)
}

func ReverseStringArray(array []string) []string {
	for i, j := 0, len(array)-1; i < j; i, j = i+1, j-1 {
		array[i], array[j] = array[j], array[i]
	}
	return array
}
