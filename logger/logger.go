package logger

import (
	"os"
	"statusok/model"

	"github.com/sirupsen/logrus"
)

var isLoggingEnabled = false // default

func EnableLogging(fileName string) {
	isLoggingEnabled = true

	// Log as JSON instead of the default ASCII formatter.
	logrus.SetFormatter(&logrus.JSONFormatter{})

	if len(fileName) == 0 {
		// Output to stderr instead of stdout, could also be a file.
		logrus.SetOutput(os.Stderr)
	} else {
		f, err := os.OpenFile(fileName, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
		if err != nil {
			println("Invalid File Path given for parameter --log")
			os.Exit(3)
		}

		logrus.SetOutput(f)
	}
}

func LogErrorInfo(errorInfo model.ErrorInfo) {
	if isLoggingEnabled {
		logrus.WithFields(logrus.Fields{
			"id":           errorInfo.Id,
			"url":          errorInfo.Url,
			"requestType":  errorInfo.RequestType,
			"responseCode": errorInfo.ResponseCode,
			"responseBody": errorInfo.ResponseBody,
			"reason":       errorInfo.Reason.Error(),
			"otherInfo":    errorInfo.Reason,
		}).Error("Status Ok Error occurred for url " + errorInfo.Url)
	}
}

func LogRequestInfo(requestInfo model.RequestInfo) {
	if isLoggingEnabled {
		logrus.WithFields(logrus.Fields{
			"id":                   requestInfo.Id,
			"url":                  requestInfo.Url,
			"requestType":          requestInfo.RequestType,
			"responseCode":         requestInfo.ResponseCode,
			"responseTimeMs":       requestInfo.ResponseTimeMs,
			"expectedResponseTime": requestInfo.ExpectedResponseTime,
		}).Info("")
	}
}
