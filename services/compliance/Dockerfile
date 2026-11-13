# Builds the Compliance & Insights Service into a small static image (ADR-005 rule 4).
FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /compliance ./cmd/compliance

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /compliance /compliance
EXPOSE 8083
ENTRYPOINT ["/compliance"]
