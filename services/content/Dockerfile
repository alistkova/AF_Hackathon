FROM golang:1.26-alpine AS build
WORKDIR /src
RUN apk add --no-cache git ca-certificates
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/content ./services/content/cmd/api
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/content-worker ./services/content/cmd/worker
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/content-seed ./services/content/cmd/seed

FROM alpine:3.21
RUN apk add --no-cache ca-certificates && adduser -D -H app
WORKDIR /app
COPY --from=build /out/content /app/content
COPY --from=build /out/content-worker /app/content-worker
COPY --from=build /out/content-seed /app/content-seed
COPY seed /seed
USER app
EXPOSE 8082
ENTRYPOINT ["/app/content"]
