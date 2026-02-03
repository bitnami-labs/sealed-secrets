FROM gcr.io/distroless/static@sha256:972618ca78034aaddc55864342014a96b85108c607372f7cbd0dbd1361f1d841
LABEL maintainer "Sealed Secrets <sealed-secrets.pdl@broadcom.com>"

USER 1001

ARG TARGETARCH
COPY dist/kubeseal_linux_${TARGETARCH}*/kubeseal /usr/local/bin/

ENTRYPOINT ["kubeseal"]
