FROM ubuntu:20.04

ENV DEBIAN_FRONTEND=noninteractive
ENV PATH=/usr/local/bin:$PATH

# Install test dependencies
RUN apt-get update && apt-get install -y \
    curl \
    git \
    sudo \
    bash \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

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
