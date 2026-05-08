# builder image
FROM golang:alpine AS builder

WORKDIR /src

# for downloading dependencies
RUN apk add --no-cache git

# copy and download dependencies
COPY go.mod go.sum ./
RUN go mod download

# copy code and compile 
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /app/bot ./cmd/DailySongBot/DailySongBot.go

# starting making final image
FROM alpine:latest

# download certs
RUN apk add --no-cache ca-certificates

WORKDIR /opt/DailySongBot

# copy binary from builder
COPY --from=builder /app/bot ./DailySongBot
RUN chmod +x DailySongBot

# symlink, volume for config cannot be in same folder as binary
RUN ln -s /etc/DailySongBot/config.json config.json

ENTRYPOINT ["./DailySongBot"]