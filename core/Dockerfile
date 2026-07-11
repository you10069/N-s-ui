FROM --platform=$BUILDPLATFORM golang:1.23-alpine AS singbox-builder
LABEL maintainer="Alireza <alireza7@gmail.com>"
WORKDIR /app
ARG TARGETOS TARGETARCH
ARG SINGBOX_VER=v1.12.25
ARG SINGBOX_TAGS="with_gvisor,with_quic,with_dhcp,with_wireguard,with_utls,with_acme,with_v2ray_api,with_clash_api,with_tailscale"
ARG GOPROXY=""
ENV GOPROXY ${GOPROXY}
ENV CGO_ENABLED=0
ENV GOOS=$TARGETOS
ENV GOARCH=$TARGETARCH
RUN apk --no-cache --update add build-base gcc wget unzip git
RUN set -ex \
    && git clone --depth 1 --branch $SINGBOX_VER https://github.com/SagerNet/sing-box.git \
    && cd sing-box \
    && go build -v -trimpath -tags \
        $SINGBOX_TAGS \
        -ldflags "-X \"github.com/sagernet/sing-box/constant.Version=$SINGBOX_VER\" -s -w -buildid=" \
        ./cmd/sing-box

FROM --platform=$TARGETPLATFORM alpine
LABEL maintainer="Alireza <alireza7@gmail.com>"
ENV TZ=Asia/Tehran
WORKDIR /app
RUN apk add  --no-cache --update ca-certificates tzdata bash
COPY --from=singbox-builder /app/sing-box/sing-box .
COPY runSingbox.sh .
ENTRYPOINT [ "./runSingbox.sh" ]