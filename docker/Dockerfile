FROM alpine
MAINTAINER Angus Lees <gus@inodes.org>
COPY controller /usr/local/bin/
COPY trigger /usr/local/bin/
COPY blacklist /usr/local/bin

EXPOSE 8080
EXPOSE 8081
ENTRYPOINT ["controller"]
