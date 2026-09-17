FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /teamops ./cmd/api

FROM alpine:3.21
RUN addgroup -S app && adduser -S -G app app
COPY --from=build /teamops /usr/local/bin/teamops
USER app
EXPOSE 8080
HEALTHCHECK --interval=10s --timeout=3s --retries=5 CMD wget -qO- http://localhost:8080/health || exit 1
ENTRYPOINT ["teamops"]
