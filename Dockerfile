FROM vibioh/scratch

ENV API_PORT 1080
EXPOSE 1080

COPY ca-certificates.crt /etc/ssl/certs/ca-certificates.crt

HEALTHCHECK --retries=5 CMD [ "/kitten", "-url", "http://localhost:1080/health" ]
ENTRYPOINT [ "/kitten" ]

ARG VERSION
ENV VERSION=${VERSION}

ARG TARGETOS
ARG TARGETARCH

COPY release/kitten_${TARGETOS}_${TARGETARCH} /kitten

VOLUME /tmp
