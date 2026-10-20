FROM golang:1.26-alpine AS build
WORKDIR /src
RUN apk add --no-cache git ca-certificates
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/company ./services/company/cmd/api
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/seed ./services/company/cmd/seed

FROM alpine:3.21
RUN apk add --no-cache ca-certificates && adduser -D -H app
WORKDIR /app
COPY --from=build /out/company /app/company
COPY --from=build /out/seed /app/seed
COPY seed /seed
USER app
EXPOSE 8081
ENTRYPOINT ["/app/company"]
