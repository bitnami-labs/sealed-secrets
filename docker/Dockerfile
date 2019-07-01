# Ideally we could use the "static" flavour but let's first start with the base flavour (which has glibc).
FROM gcr.io/distroless/base@sha256:edc3643ddf96d75032a55e240900b68b335186f1e5fea0a95af3b4cc96020b77
MAINTAINER Marko Mikulicic <mkmik@vmware.com>
COPY controller /usr/local/bin/

EXPOSE 8080
ENTRYPOINT ["controller"]
