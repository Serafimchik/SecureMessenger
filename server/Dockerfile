FROM golang:1.23-alpine AS builder

COPY . /github.com/Serafimchik/SecureMessenger/server/
WORKDIR /github.com/Serafimchik/SecureMessenger/server/

RUN go mod download
RUN go build -o ./bin/crud_server cmd/main.go

FROM alpine:latest

WORKDIR /root/
COPY --from=builder /github.com/Serafimchik/SecureMessenger/server/bin/crud_server .

CMD [ "./crud_server" ]
