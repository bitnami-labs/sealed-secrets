FROM gcr.io/distroless/static@sha256:c6d5981545ce1406d33e61434c61e9452dad93ecd8397c41e89036ef977a88f4
LABEL maintainer "Bitnami <containers@bitnami.com>, Marko Mikulicic <mmikulicic@gmail.com>"

ARG TARGETARCH
COPY dist/controller_linux_$TARGETARCH/controller /usr/local/bin/

EXPOSE 8080
ENTRYPOINT ["controller"]
