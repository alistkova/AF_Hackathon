FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/identity ./services/identity/cmd/api

FROM gcr.io/distroless/static-debian12
COPY --from=build /out/identity /identity
EXPOSE 8086
ENTRYPOINT ["/identity"]
