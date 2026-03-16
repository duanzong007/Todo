FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS build

ARG TARGETOS
ARG TARGETARCH

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} go build -trimpath -ldflags="-s -w" -o /out/todo ./cmd/server

FROM alpine:3.20

WORKDIR /app

RUN apk add --no-cache tzdata ca-certificates

COPY --from=build /out/todo /app/todo
COPY db /app/db
COPY web /app/web

EXPOSE 8080

CMD ["/app/todo"]
