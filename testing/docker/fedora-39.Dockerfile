FROM fedora:39

ENV PATH=/usr/local/bin:$PATH

# Install test dependencies
RUN dnf install -y \
    curl \
    git \
    sudo \
    bash \
    ca-certificates \
    && dnf clean all

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
