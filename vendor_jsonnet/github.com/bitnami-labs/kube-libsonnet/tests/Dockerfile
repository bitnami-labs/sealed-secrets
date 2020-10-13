FROM bitnami/minideb:buster
MAINTAINER sre@bitnami.com

ARG jsonnet_version=0.14.0
ARG kubectl_version=v1.17.0
ARG kubecfg_version=v0.14.0

RUN install_packages jq make curl ca-certificates
RUN adduser --home /home/user --disabled-password --gecos User user

RUN curl -sLo /tmp/jsonnet-v${jsonnet_version}.tar.gz https://github.com/google/jsonnet/releases/download/v${jsonnet_version}/jsonnet-bin-v${jsonnet_version}-linux.tar.gz
RUN tar -zxf /tmp/jsonnet-v${jsonnet_version}.tar.gz -C /tmp && mv /tmp/jsonnet /tmp/jsonnetfmt /usr/local/bin

RUN curl -sLo /usr/local/bin/kubectl https://storage.googleapis.com/kubernetes-release/release/${kubectl_version}/bin/linux/amd64/kubectl
RUN chmod +x /usr/local/bin/kubectl

RUN curl -sLo /usr/local/bin/kubecfg https://github.com/bitnami/kubecfg/releases/download/${kubecfg_version}/kubecfg-linux-amd64
RUN chmod +x /usr/local/bin/kubecfg

USER user
WORKDIR /home/user
CMD ["/bin/bash", "-l"]
