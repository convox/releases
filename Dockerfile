FROM convox/golang

WORKDIR $GOPATH/src/github.com/convox/releases
COPY . .
RUN go install .

CMD ["releases"]
