#
# SMTP Tester Dockerfile
#
FROM golang:alpine as intermediate

# Add Maintainer Info
LABEL maintainer="Moritz Wacker <wacker@kreapptivo.de>"

#Install ssh-client
# Make ssh dir
# Create known_hosts
# Add ssh key
#RUN apk --no-cache add --update openssh-client \
#  && mkdir /root/.ssh/ \
#  && touch /root/.ssh/known_hosts \ 
#  && ssh-keyscan -p 7999 dev.kreapptivo.de >> /root/.ssh/known_hosts

# Copy over private key, and set permissions
# Warning! Anyone who gets their hands on this image will be able
# to retrieve this private key file from the corresponding image layer
#ADD repo.key /root/.ssh/id_rsa


# Install GIT:
RUN apk --no-cache add --virtual build-dependencies \
    git \
  && mkdir -p /root/gocode/src \
  && export GOPATH=/root/gocode 

WORKDIR /root/gocode/src

# CLONE code
#RUN git clone ssh://git@dev.kreapptivo.de:7999/mwacker/smtp-tester.git
RUN git clone https://github.com/kreapptivo/statusok.git

WORKDIR /root/gocode/src/statusok
#RUN git pull
RUN git log --name-status HEAD^..HEAD

#Build APP
RUN go mod download \
#  && go get github.com/GeertJohan/go.rice/rice \
#  && rice embed-go \
   && go build
#  && ls -al

ADD https://github.com/ufoscout/docker-compose-wait/releases/download/2.5.0/wait wait
RUN chmod +x wait

FROM golang:alpine
COPY --from=intermediate /root/gocode/src/statusok/statusok /usr/local/bin
COPY --from=intermediate /root/gocode/src/statusok/wait /usr/local/bin

# Install APP

# Add mailhog user/group with uid/gid 1000.
# This is a workaround for boot2docker issue #581, see
# https://github.com/boot2docker/boot2docker/issues/581
RUN adduser -D -u 1000 statusok

USER statusok

WORKDIR /home/statusok
VOLUME /config
COPY --from=intermediate /root/gocode/src/statusok ./
#COPY --from=intermediate /root/gocode/src/smtp-tester/templates ./

#ENTRYPOINT ["statusok", "--config", "/config/config.json"]
CMD "wait && statusok --config /config/config.json"

# Expose the HTTP port:
#EXPOSE 8080