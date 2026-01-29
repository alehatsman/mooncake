FROM ubuntu:bionic

RUN apt-get clean
RUN apt-get update 
RUN apt-get upgrade -y
RUN apt-get -y install sudo

ENV PATH /tmp/bin:$PATH
ENV HOME /tmp/home
RUN mkdir /tmp/home

COPY ./out/mooncake /tmp/bin/mooncake

RUN mkdir /tmp/mooncake-automation/
WORKDIR /tmp/mooncake-automation

COPY ./mooncake-automation/global_variables.yml /tmp/mooncake-automation/global_variables.yml

# Ubuntu packages
COPY ./mooncake-automation/packages/ /tmp/mooncake-automation/packages/
RUN mooncake run -c ./packages/ubuntu/basics.yml -v ./global_variables.yml

# Golang installation
COPY ./mooncake-automation/golang/ /tmp/mooncake-automation/golang/
RUN mooncake run -c ./golang/main.yml -v ./global_variables.yml

# Node installation

COPY ./mooncake-automation/node/ /tmp/mooncake-automation/node/
RUN mooncake run -c ./node/main.yml -v ./global_variables.yml
