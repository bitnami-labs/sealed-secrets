FROM gcr.io/distroless/static@sha256:f2ea2709ac8db56323cbd7d014277f32cb572d9ea124b0076f7aafe5980678fe
LABEL maintainer "Sealed Secrets <sealed-secrets.pdl@broadcom.com>"

USER 1001

ARG TARGETARCH
COPY dist/controller_linux_${TARGETARCH}*/controller /usr/local/bin/

EXPOSE 8080 8081

ENTRYPOINT ["controller"]
