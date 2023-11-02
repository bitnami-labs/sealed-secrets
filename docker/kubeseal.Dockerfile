FROM gcr.io/distroless/static@sha256:d6fa9db9548b5772860fecddb11d84f9ebd7e0321c0cb3c02870402680cc315f
LABEL maintainer "Sealed Secrets  <bitnami-sealed-secrets@vmware.com>"

USER 1001

ARG TARGETARCH
COPY dist/kubeseal_linux_${TARGETARCH}*/kubeseal /usr/local/bin/

ENTRYPOINT ["kubeseal"]
