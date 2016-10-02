FROM golang:1.7
RUN go get -d -v github.com/aws/aws-sdk-go/aws
RUN go get -d -v github.com/aws/aws-sdk-go/aws/session
RUN go get -d -v github.com/aws/aws-sdk-go/service/ec2
RUN go get -d -v github.com/aws/aws-sdk-go/service/sqs
RUN go get -d -v github.com/nlopes/slack
COPY *.go /cw-monitor/
WORKDIR /cw-monitor
RUN go build -v
