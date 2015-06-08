FROM google/golang

WORKDIR /gopath/src/github.com/dfb5865/property-financials-api
ADD . /gopath/src/github.com/dfb5865/property-financials-api

# go get our dependency manager
RUN go get github.com/tools/godep

# go get our code
RUN go get github.com/dfb5865/property-financials-api

# install our dependencies
RUN godep restore

EXPOSE 8080
CMD []
ENTRYPOINT ["/gopath/bin/property-financials-api"]
