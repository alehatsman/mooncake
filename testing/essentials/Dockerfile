FROM ubuntu:bionic

ENV PATH /tmp/bin:$PATH
ENV HOME /tmp/home
RUN mkdir /tmp/home

COPY ../../out/mooncake-linux-amd64 /tmp/bin/mooncake

RUN mkdir /tmp/mooncake-automation/
WORKDIR /tmp/mooncake-automation

COPY ./testing/essentials/file.yml /tmp/mooncake-automation/file.yml

RUN mooncake run -c ./file.yml
