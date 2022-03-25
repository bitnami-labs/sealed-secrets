FROM gcr.io/distroless/static@sha256:8ad6f3ec70dad966479b9fb48da991138c72ba969859098ec689d1450c2e6c97
LABEL maintainer "Bitnami <containers@bitnami.com>, Marko Mikulicic <mmikulicic@gmail.com>"

ARG TARGETARCH
COPY dist/controller_linux_$TARGETARCH/controller /usr/local/bin/

EXPOSE 8080
ENTRYPOINT ["controller"]
