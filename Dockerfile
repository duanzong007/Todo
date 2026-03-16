FROM golang:1.22-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/todo ./cmd/server

FROM alpine:3.20

WORKDIR /app

COPY --from=build /out/todo /app/todo
COPY db /app/db
COPY web /app/web

EXPOSE 8080

CMD ["/app/todo"]

