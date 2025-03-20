FROM golang:latest

WORKDIR /app

COPY . .

RUN go mod download

RUN go build -o ./server-ex ./server
RUN go build -o ./worker-ex ./worker
RUN go build -o ./scanner-ex ./scanner