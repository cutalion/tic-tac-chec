FROM golang:1.25-alpine AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o /ssh-server ./cmd/ssh

FROM alpine:3.20
RUN apk add --no-cache ncurses-terminfo-base
COPY --from=build /ssh-server /ssh-server
ENV TERM=xterm-256color
CMD ["/ssh-server"]
