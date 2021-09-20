package main

import (
	"statusok/database"
	"statusok/notify"
	"statusok/requests"
	"testing"

	"github.com/stretchr/testify/assert"
)

/* test config parser */
func TestReadConfig(t *testing.T) {
	const testConfigFileName = "sample_config.json"
	expected := configuration{
		NotifyWhen: NotifyWhen{MinResponseCount: 0, ErrorCount: 0},
		Requests: []requests.RequestConfig{
			{
				Url:         "http://mywebsite.com/v1/data",
				RequestType: "POST",
				Headers: map[string]string{
					"Authorization": "Bearer ac2168444f4de69c27d6384ea2ccf61a49669be5a2fb037ccc1f",
					"Content-Type":  "application/json",
				},
				FormParams: map[string]string{
					"description": "sanath test",
					"url":         "http://google.com",
				},
				CheckEvery:   "30s",
				ResponseCode: 200,
				ResponseTime: 800,
			},
			{
				Url:         "http://mywebsite.com/v1/data",
				RequestType: "GET",
				Headers: map[string]string{
					"Authorization": "Bearer ac2168444f4de69c27d6384ea2ccf61a49669be5a2fb037ccc1f",
				},
				UrlParams: map[string]string{
					"name": "statusok",
				},
				CheckEvery:   "300s",
				ResponseCode: 200,
				ResponseTime: 800,
			},
			{
				Url:         "http://something.com/v1/data",
				RequestType: "DELETE",
				FormParams: map[string]string{
					"name": "statusok",
				},
				CheckEvery:   "300s",
				ResponseCode: 200,
				ResponseTime: 800,
			},
			{
				Url:          "https://google.com",
				RequestType:  "GET",
				Headers:      map[string]string{},
				FormParams:   map[string]string(nil),
				CheckEvery:   "30s",
				ResponseCode: 200,
				ResponseTime: 800,
			},
		},
		Notifications: notify.NotificationTypes{
			MailNotify: notify.MailNotify{Username: "statusok@gmail.com", Password: "password", Host: "smtp.gmail.com", Port: 587, From: "statusok@gmail.com", To: "notify@gmail.com", Cc: ""},
			Mailgun:    notify.MailgunNotify{Email: "statusok@gmail.com", ApiKey: "key-a8215497fc0", Domain: "statusok.com", PublicApiKey: "pubkey-a225d8a8e7ee48"},
			Slack:      notify.SlackNotify{Username: "statusok", ChannelName: "", ChannelWebhookURL: "https://hooks.slack.com/services/T09ZQZhET2E5Tl7", IconUrl: ""},
			Http:       notify.HttpNotify{Url: "http://mywebsite.com", RequestType: "POST", Headers: map[string]string{"Authorization": "Bearer ac2168444f4de69c27d6384ea2ccf61a49669be5a2fb037ccc1f"}},
			Dingding:   notify.DingdingNotify{HttpNotify: notify.HttpNotify{Url: "https://oapi.dingtalk.com/robot/send?access_token=1b153686301db8662", RequestType: "POST", Headers: map[string]string{"Content-Type": "application/json"}}},
			Pagerduty:  notify.PagerdutyNotify{Url: "https://events.pagerduty.com/v2/enqueue", RoutingKey: "abcdefghijklmnopqrstuvwxyz123456", Severity: "info"},
		},
		Database:    database.DatabaseTypes{InfluxDb: database.InfluxDb{Host: "localhost", Port: 8086}},
		Concurrency: 0,
		Port:        0,
	}

	config, err := readConfig(testConfigFileName)
	assert.Nil(t, err)
	assert.Equal(t, expected, config)
}
