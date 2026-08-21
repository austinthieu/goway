FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/gateway ./cmd/gateway
RUN CGO_ENABLED=0 go build -o /out/demo-backend ./cmd/demo-backend

FROM alpine:3.20
COPY --from=build /out/gateway /usr/local/bin/gateway
COPY --from=build /out/demo-backend /usr/local/bin/demo-backend
COPY testdata/config.yaml /etc/gobalance/config.yaml
EXPOSE 8080 9090
ENTRYPOINT ["gateway"]
CMD ["-config", "/etc/gobalance/config.yaml"]
