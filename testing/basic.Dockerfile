FROM ubuntu:bionic

ENV PATH /tmp/bin:$PATH
ENV HOME /tmp/home
RUN mkdir /tmp/home

COPY ./out/mooncake /tmp/bin/mooncake

RUN mkdir /tmp/mooncake-automation/
WORKDIR /tmp/mooncake-automation

COPY ./basic-automation/** /tmp/mooncake-automation/

RUN mooncake run -c ./config.yml -v ./variables.yml
