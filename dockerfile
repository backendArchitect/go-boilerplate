FROM golang:1.21-alpine AS build

ENV CGO_ENABLED=0

WORKDIR /go/src/app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN cp .env.example .env

RUN go build -o bin/go-boilerplate

ENTRYPOINT ["bin/go-boilerplate"]

CMD ["api"]
