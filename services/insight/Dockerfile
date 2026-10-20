FROM golang:1.26-alpine AS build
WORKDIR /src
RUN apk add --no-cache git ca-certificates
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/insight ./services/insight/cmd/api
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/insight-worker ./services/insight/cmd/worker

FROM alpine:3.21
RUN apk add --no-cache ca-certificates && adduser -D -H app
WORKDIR /app
COPY --from=build /out/insight /app/insight
COPY --from=build /out/insight-worker /app/insight-worker
USER app
EXPOSE 8083
ENTRYPOINT ["/app/insight"]
