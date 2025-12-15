FROM --platform=$BUILDPLATFORM node:alpine AS front-builder
WORKDIR /app
COPY frontend/ ./
RUN npm install && npm run build

FROM golang:1.25-alpine AS backend-builder
WORKDIR /app
ARG TARGETARCH
ENV CGO_ENABLED=1
ENV GOARCH=$TARGETARCH

RUN apk update && apk add --no-cache \
    make \
    git \
    wget \
    unzip \
    bash \
    build-base

COPY . .
COPY --from=front-builder /app/dist/ /app/web/html/

RUN go build -ldflags="-w -s" \
    -tags "with_quic,with_grpc,with_utls,with_acme,with_gvisor,netgo" \
    -o sui main.go

FROM --platform=$TARGETPLATFORM alpine
LABEL org.opencontainers.image.authors="alireza7@gmail.com"
ENV TZ=Asia/Tehran
WORKDIR /app
RUN apk add --no-cache --update ca-certificates tzdata
COPY --from=backend-builder /app/sui /app/
COPY entrypoint.sh /app/
ENTRYPOINT [ "./entrypoint.sh" ]