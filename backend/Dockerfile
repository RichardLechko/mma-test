FROM golang:1.23.4 AS build

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod tidy

COPY . .

# Build with CGO disabled for static linking
RUN CGO_ENABLED=0 go build -o main .

# Final stage
FROM alpine:latest

WORKDIR /app

COPY --from=build /app/main .

EXPOSE 8080

CMD ["./main"]