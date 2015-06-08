FROM google/golang

WORKDIR /gopath/src/github.com/dfb5865/property-financials
ADD . /gopath/src/github.com/dfb5865/property-financials

# go get our dependency manager
RUN go get github.com/tools/godep

# go get our code
RUN go get github.com/dfb5865/property-financials

# install our dependencies
RUN godep restore

EXPOSE 8080
CMD []
ENTRYPOINT ["/gopath/bin/property-financials"]
