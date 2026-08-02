FROM golang:1.25-alpine AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o /chatapp ./cmd/server

FROM alpine:3.19
COPY --from=build /chatapp /chatapp
EXPOSE 8080
ENTRYPOINT ["/chatapp"]