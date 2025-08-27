# development docker file
FROM golang:1.24.5

EXPOSE 8005

WORKDIR /app

# Add user
# Add group with ID 1000 and user with ID 1000
RUN groupadd -g 1000 appuser && \
    useradd -r -u 1000 -g appuser appuser -d /home/appuser -m

# install deps
COPY go.mod /app/go.mod
RUN mkdir -p /app/.dev
RUN mkdir -p /app/.dev-air
RUN mkdir -p /app/.test
RUN chown -R appuser:appuser /app

USER appuser
RUN go install github.com/cosmtrek/air@v1.40.4 && go install github.com/go-delve/delve/cmd/dlv@latest
RUN go mod tidy

CMD ["air", "-c", "/.air.docker.toml"]
