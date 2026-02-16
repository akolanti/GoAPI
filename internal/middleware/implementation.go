package middleware

import (
	"context"
	"crypto/subtle"
	"net"
	"net/http"
	"strings"

	"github.com/akolanti/GoAPI/internal/adapter/utils"
	"github.com/akolanti/GoAPI/internal/config"
	"github.com/akolanti/GoAPI/internal/handlers"
	"github.com/akolanti/GoAPI/pkg/logger_i"
)

func injectTrace(re requestResponseStruct) requestResponseStruct {
	re.logger.Debug("Injecting trace middleware")
	req := re.req
	if req == nil {
		//this is a bad request
		re.badRequest.httpCode = http.StatusBadRequest
		re.badRequest.errorMessage = "request is empty"
		re.badRequest.isBadRequest = true
		return re
	}
	trace := req.Header.Get("X-Trace-Id")
	if trace == "" {
		trace = utils.GetNewUUID()
	}
	re.logger = re.logger.With("traceId", trace)
	ctx := context.WithValue(req.Context(), config.TRACE_ID_KEY, trace)
	req.Header.Set(`X-Trace-Id`, trace)
	re.req = req.WithContext(ctx)

	re.logger.Debug("trace middleware injected")
	return re
}

func authenticate(re requestResponseStruct) requestResponseStruct {
	re.logger.Debug("Authenticating request")

	if !IsValidBearerToken(re.req.Header.Get("Authorization"), re.logger) {
		handlers.WriteErrorResponse(re.writer, http.StatusUnauthorized, "", "Unauthorized")
		re.badRequest.isBadRequest = true
		re.badRequest.errorMessage = "invalid token - you sus bruh"
		re.badRequest.httpCode = http.StatusUnauthorized
		return re
	}
	re.logger.Debug("Authorized")
	return re
}

func IsValidBearerToken(authHeader string, log *logger_i.Logger) bool {
	if config.NoAuthBypass {
		log.Error("--------------------------------------- auth bypass----------------------------------------------")
		return true
	}
	if authHeader == "" {
		log.Error("Empty authorization header")
		return false
	}
	if !strings.HasPrefix(authHeader, "Bearer ") {
		log.Error("No Bearer header")
		return false
	}
	if subtle.ConstantTimeCompare([]byte(strings.TrimPrefix(authHeader, "Bearer ")), []byte(config.AuthToken)) != 1 {
		log.Error("Invalid authorization header")
		return false
	}

	return true
}

func rateLimiter(re requestResponseStruct) requestResponseStruct {
	re.logger.Debug("Rate limiter middleware")
	ip, _, err := net.SplitHostPort(re.req.RemoteAddr)
	if err != nil {
		ip = re.req.RemoteAddr
	}

	if !limiterInstance.GetLimiter(ip).Allow() {
		re.logger.Error("Too many requests", "Rate Limiter exceeded", ip)
		re.badRequest = failureStruct{
			isBadRequest: true,
			httpCode:     http.StatusTooManyRequests,
			errorMessage: "Rate limit exceeded. Slow down bruh",
		}
		return re
	}
	re.logger.Debug("Rate limiter middleware authorized")
	return re
}

func handleBadRequest(re requestResponseStruct) bool {
	if re.badRequest.isBadRequest {
		re.logger.Warn("Bad request", "httpCode", re.badRequest.httpCode, "errorMessage", re.badRequest.errorMessage, "IP", re.req.RemoteAddr)
		handlers.WriteErrorResponse(re.writer, re.badRequest.httpCode, "Your IP: "+re.req.RemoteAddr, re.badRequest.errorMessage)
		return false
	}
	return true
}
