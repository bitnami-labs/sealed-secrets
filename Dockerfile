FROM gcr.io/distroless/static@sha256:d6fa9db9548b5772860fecddb11d84f9ebd7e0321c0cb3c02870402680cc315f
LABEL maintainer "Bitnami <containers@bitnami.com>, Marko Mikulicic <mmikulicic@gmail.com>"

USER 1001

ARG TARGETARCH
COPY dist/controller_linux_$TARGETARCH/controller /usr/local/bin/

EXPOSE 8080
ENTRYPOINT ["controller"]
