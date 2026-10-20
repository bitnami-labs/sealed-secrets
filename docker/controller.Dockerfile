FROM gcr.io/distroless/static@sha256:9e0130bec991f471279b70f1630ada11e46dad439ef2949544f96001225329fb
LABEL maintainer "Sealed Secrets <sealed-secrets.pdl@broadcom.com>"

USER 1001

ARG TARGETARCH
COPY dist/controller_linux_${TARGETARCH}*/controller /usr/local/bin/

EXPOSE 8080 8081

ENTRYPOINT ["controller"]
