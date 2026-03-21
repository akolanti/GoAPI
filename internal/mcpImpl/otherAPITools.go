package mcpImpl

import (
	"context"
	"time"
)

//Gah I dont remember the web api parameters for the request
//TODO

func callOtherAPI(ctx context.Context, query string, traceId string) (string, error) {
	//SystemMessageQuery(ctx, query, traceId)
	//simulate calling another API and getting a response
	time.Sleep(500 * time.Millisecond)
	return "Response from other API for query: " + query, nil
}

type systemMessage struct {
	Message string `json:"message"`
	Code    int    `json:"code"`
}

type systemMessageAPICall struct {
	Message string `json:"message"`
}
