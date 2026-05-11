FROM alpine:3.21
RUN apk add --no-cache ca-certificates bash
COPY mooncake /usr/local/bin/mooncake
ENTRYPOINT ["mooncake"]
