FROM golang:1.24.1-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git

RUN go install github.com/swaggo/swag/cmd/swag@latest

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN swag init -g cmd/api/main.go --parseDependency --parseInternal --dir ./ --output ./cmd/api/docs

RUN CGO_ENABLED=0 GOOS=linux go build -o main ./cmd/api/main.go


FROM alpine:latest

#add certs for https

WORKDIR /root/

COPY --from=builder /app/main .

EXPOSE 3000

CMD ["./main"]