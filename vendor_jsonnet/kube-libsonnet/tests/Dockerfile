FROM debian:8
MAINTAINER sre@bitnami.com

RUN adduser --home /home/user --disabled-password --gecos User user

RUN apt-get -q update && apt-get -qy install jq make

ADD https://storage.googleapis.com/bitnami-jenkins-tools/jsonnet-0.9.5 /usr/local/bin/jsonnet
RUN chmod +x /usr/local/bin/jsonnet

ADD https://storage.googleapis.com/kubernetes-release/release/v1.9.0/bin/linux/amd64/kubectl /usr/local/bin/kubectl
RUN chmod +x /usr/local/bin/kubectl

ADD https://github.com/ksonnet/kubecfg/releases/download/v0.7.2/kubecfg-linux-amd64 /usr/local/bin/kubecfg
RUN chmod +x /usr/local/bin/kubecfg

USER user
WORKDIR /home/user
CMD ["/bin/bash", "-l"]
