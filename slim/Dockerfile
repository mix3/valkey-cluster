FROM alpine:latest AS prebuild

ARG TARGETARCH
ARG GOREMAN_VERSION=0.3.16

RUN wget https://github.com/mattn/goreman/releases/download/v${GOREMAN_VERSION}/goreman_v${GOREMAN_VERSION}_linux_${TARGETARCH}.tar.gz \
 && tar xvf goreman_v${GOREMAN_VERSION}_linux_${TARGETARCH}.tar.gz

FROM valkey/valkey:7.2.6-alpine

ARG TARGETARCH
ARG GOREMAN_VERSION=0.3.16

RUN apk add --update libintl \
 && apk add --virtual build_deps gettext \
 && cp /usr/bin/envsubst /usr/local/bin/envsubst \
 && apk del build_deps \
 && rm -rf /var/cache/apk/*

COPY --from=prebuild /goreman_v${GOREMAN_VERSION}_linux_${TARGETARCH}/goreman /usr/local/bin/

COPY valkey-cluster.tmpl /valkey-conf/valkey-cluster.tmpl
COPY valkey.tmpl         /valkey-conf/valkey.tmpl
COPY sentinel.tmpl      /valkey-conf/sentinel.tmpl
COPY LICENSE /LICENSE

# Add startup script
COPY slim/docker-entrypoint.sh /docker-entrypoint.sh

# Add script that generates supervisor conf file based on environment variables
COPY slim/generate-procfile.sh /generate-procfile.sh

RUN chmod 755 /docker-entrypoint.sh

EXPOSE 7000 7001 7002 7003 7004 7005 7006 7007 5000 5001 5002

ENTRYPOINT ["/docker-entrypoint.sh"]
CMD ["valkey-cluster"]
