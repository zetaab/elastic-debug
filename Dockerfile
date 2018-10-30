FROM alpine

USER 1001
COPY bin/elastic-debug .
ENTRYPOINT ["./elastic-debug"]
