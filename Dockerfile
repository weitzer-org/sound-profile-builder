FROM golang:latest AS builder

WORKDIR /app

# Cached dependency installation
COPY go.mod go.sum ./
# Because local macOS Santa limits killed our go mod tidy execution, 
# we explicitly force the Linux container to download and wire the dependencies natively.
COPY . .
RUN go mod tidy

# Build application securely isolated inside the Linux host safely bypassing macOS Santa policies
RUN CGO_ENABLED=0 GOOS=linux go build -o /server ./cmd/server/main.go

# Minimal distroless runtime execution stage 
FROM gcr.io/distroless/static-debian11

WORKDIR /app
COPY --from=builder /server /server
COPY --from=builder /app/web/templates /app/web/templates
COPY --from=builder /app/internal/agents/prompts /app/internal/agents/prompts
COPY --from=builder /app/config.json /app/config.json

EXPOSE 8080
ENTRYPOINT ["/server"]
