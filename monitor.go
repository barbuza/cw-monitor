package main

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"time"
	"unicode/utf8"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/nlopes/slack"
)

type cloudWatchTrigger struct {
	MetricName         string
	Namespace          string
	Statistic          string
	Unit               *string
	Period             int
	EvaluationPeriods  int
	ComparisonOperator string
	Threshold          float32
}

type cloudWatchMessage struct {
	AlarmName        string
	NewStateValue    string
	AlarmDescription *string
	NewStateReason   *string
	StateChangeTime  string
	Region           *string
	OldStateValue    *string
	Trigger          cloudWatchTrigger
}

type sqsMessage struct {
	Type             string
	TopicArn         string
	Subject          string
	Message          string
	Timestamp        string
	SignatureVersion string
	Signature        string
	SigningCertURL   string
	UnsubscribeURL   string
}

func parseSqsMessage(input string) *cloudWatchMessage {
	var message sqsMessage
	if json.Unmarshal([]byte(input), &message) != nil {
		return nil
	}
	var cwMessage cloudWatchMessage
	if json.Unmarshal([]byte(message.Message), &cwMessage) != nil {
		return nil
	}

	return &cwMessage
}

func formatSlackMessage(
	botName string,
	cwMessage *cloudWatchMessage,
) *slack.PostMessageParameters {
	text := fmt.Sprintf("%s %s", cwMessage.AlarmName, cwMessage.NewStateValue)
	messageParams := slack.PostMessageParameters{Username: botName}
	attach := slack.Attachment{}
	attach.Fallback = text
	attach.Text = text
	if cwMessage.NewStateValue == "OK" {
		attach.Color = "good"
	} else if cwMessage.NewStateValue == "ALARM" {
		attach.Color = "danger"
	}
	messageParams.Attachments = []slack.Attachment{attach}
	return &messageParams
}

func watchQueue(
	sess *session.Session,
	region string,
	url string,
	slackAPI *slack.Client,
	botName string,
	channel string,
) {
	defer os.Exit(-1)
	svq := sqs.New(sess, &aws.Config{Region: &region})
	for {
		fmt.Printf("poll %s\n", url)
		resp, err := svq.ReceiveMessage(&sqs.ReceiveMessageInput{
			QueueUrl:        &url,
			WaitTimeSeconds: aws.Int64(20),
		})
		if err != nil {
			panic(err)
		}
		for _, msg := range resp.Messages {
			if cwMessage := parseSqsMessage(*msg.Body); cwMessage != nil {
				messageParams := formatSlackMessage(botName, cwMessage)
				_, _, err := slackAPI.PostMessage(channel, "", *messageParams)
				if err != nil {
					panic(err)
				}
			}
			if _, err := svq.DeleteMessage(&sqs.DeleteMessageInput{
				QueueUrl:      &url,
				ReceiptHandle: msg.ReceiptHandle,
			}); err != nil {
				panic(err)
			}
		}
	}
}

func findQueues(
	queueNameRegex string,
	token string,
	botName string,
	channel string,
) {
	slackAPI := slack.New(token)
	sess := session.New()
	svc := ec2.New(sess, &aws.Config{
		Region: aws.String("us-west-2"),
	})
	resp, err := svc.DescribeRegions(nil)
	if err != nil {
		panic(err)
	}

	for _, region := range resp.Regions {
		fmt.Printf("checking %s\n", *region.RegionName)
		svq := sqs.New(sess, &aws.Config{
			Region: region.RegionName,
		})
		resp, err := svq.ListQueues(&sqs.ListQueuesInput{})
		if err != nil {
			panic(err)
		}
		for _, url := range resp.QueueUrls {
			match, err := regexp.Match("-monitoring$", []byte(*url))
			if err != nil {
				panic(err)
			}
			if match {
				go watchQueue(
					sess,
					*region.RegionName,
					*url,
					slackAPI,
					botName,
					channel,
				)
			}
		}
	}
}

func main() {
	token := os.Getenv("SLACK_TOKEN")
	if utf8.RuneCountInString(token) == 0 {
		panic("SLACK_TOKEN is required")
	}

	var botName = os.Getenv("BOT_NAME")
	if utf8.RuneCountInString(botName) == 0 {
		botName = "bot"
	}

	var channel = os.Getenv("SLACK_CHANNEL")
	if utf8.RuneCountInString(channel) == 0 {
		channel = "monitoring"
	}

	var queueNameRegex = os.Getenv("QUEUE_NAME_REGEX")
	if utf8.RuneCountInString(queueNameRegex) == 0 {
		queueNameRegex = "-monitoring$"
	}

	findQueues(queueNameRegex, token, botName, channel)

	for {
		time.Sleep(time.Second)
	}
}
