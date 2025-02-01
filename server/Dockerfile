FROM golang:1.20.3-alpine AS builder

COPY . /github.com/Serafimchik/SecureMessenger/server/source/
WORKDIR /github.com/Serafimchik/SecureMessenger/server/source/

RUN go mod download
RUN go build -o ./bin/crud_server main.go

FROM alpine:latest

WORKDIR /root/
COPY --from=builder /github.com/Serafimchik/SecureMessenger/server/source/bin/crud_server .

CMD [ "./crud_server" ]