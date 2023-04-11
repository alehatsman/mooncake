FROM ubuntu:bionic

RUN apt-get clean
RUN apt-get update 
RUN apt-get upgrade -y
RUN apt-get -y install sudo

RUN echo 'debconf debconf/frontend select Noninteractive' | debconf-set-selections

ENV DEBIAN_FRONTEND noninteractive
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

# Zsh installation
COPY ./mooncake-automation/zsh/ /tmp/mooncake-automation/zsh/
RUN mooncake run -c ./zsh/main.yml -v ./global_variables.yml

# Node installation
COPY ./mooncake-automation/node/ /tmp/mooncake-automation/node/
RUN mooncake run -c ./node/main.yml -v ./global_variables.yml

COPY ./mooncake-automation/git/ /tmp/mooncake-automation/git/
RUN mooncake run -c ./git/main.yml -v ./global_variables.yml

COPY ./mooncake-automation/tmux/ /tmp/mooncake-automation/tmux/
RUN mooncake run -c ./tmux/main.yml -v ./global_variables.yml

COPY ./mooncake-automation/google-cloud/ /tmp/mooncake-automation/google-cloud/
RUN mooncake run -c ./google-cloud/main.yml -v ./global_variables.yml
