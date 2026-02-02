FROM alpine:3.19

ENV PATH=/usr/local/bin:$PATH

# Install test dependencies
RUN apk add --no-cache \
    bash \
    curl \
    git \
    sudo \
    ca-certificates

# Copy mooncake binary
COPY out/mooncake-linux-amd64 /usr/local/bin/mooncake
RUN chmod +x /usr/local/bin/mooncake

# Copy test runner and fixtures
COPY testing/common/test-runner.sh /test-runner.sh
RUN chmod +x /test-runner.sh
COPY testing/fixtures/ /fixtures/

WORKDIR /workspace

ENTRYPOINT ["/test-runner.sh"]
CMD ["smoke"]
