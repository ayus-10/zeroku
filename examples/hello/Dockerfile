FROM golang:1.27.3-alpine AS build
WORKDIR /app
COPY . .
RUN go build -o hello .

FROM alpine:3.20
WORKDIR /app
COPY --from=build /app/hello /app
ENTRYPOINT ["/app/hello"]
