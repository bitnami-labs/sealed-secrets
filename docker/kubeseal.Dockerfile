FROM gcr.io/distroless/static@sha256:4b2a093ef4649bccd586625090a3c668b254cfe180dee54f4c94f3e9bd7e381e
LABEL maintainer "Sealed Secrets <sealed-secrets.pdl@broadcom.com>"

USER 1001

ARG TARGETARCH
COPY dist/kubeseal_linux_${TARGETARCH}*/kubeseal /usr/local/bin/

ENTRYPOINT ["kubeseal"]
