FROM golang:1.25.2-alpine AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o /bin/go-blockchain .

FROM alpine:3.21
WORKDIR /app
COPY --from=build /bin/go-blockchain /usr/local/bin/go-blockchain
EXPOSE 3030 4030
ENTRYPOINT ["go-blockchain"]
