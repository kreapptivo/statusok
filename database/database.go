package database

import (
	"errors"
	"fmt"
	"os"
	"reflect"
	"sort"
	"statusok/notify"
	"strings"

	"github.com/sirupsen/logrus"
)

var (
	MinResponseCount = 3 // Default number of response times to calcuate median response time
	ErrorCount       = 1 // Default number of errors should occur to send notification

	dbList        []Database      // list of databases registered
	responseQueue map[int][]int64 // A map of queues to calculate mean response time

	ErrResponseCode  = errors.New("Response code do not Match")
	ErrTimeout       = errors.New("Request Time out Error")
	ErrCreateRequest = errors.New("Invalid Request Config. Not able to create request")
	ErrDoRequest     = errors.New("Request failed")

	isLoggingEnabled = false // default
)

type RequestInfo struct {
	Id                   int
	Url                  string
	RequestType          string
	ResponseCode         int
	ResponseTimeMs       int64
	ExpectedResponseTime int64
}

type ErrorInfo struct {
	Id           int
	Url          string
	RequestType  string
	ResponseCode int
	ResponseBody string
	Reason       error
	OtherInfo    string
}

type Database interface {
	Initialize() error
	GetDatabaseName() string
	AddRequestInfo(requestInfo RequestInfo) error
	AddErrorInfo(errorInfo ErrorInfo) error
}

type DatabaseTypes struct {
	InfluxDb InfluxDb `json:"influxDb"`
}

// Intialize responseMean app and counts
func Initialize(ids map[int]int64, mMinResponseCount int, mErrorCount int) {
	if mMinResponseCount != 0 {
		MinResponseCount = mMinResponseCount
	}

	if mErrorCount != 0 {
		ErrorCount = mErrorCount
	}
	// TODO: try to make all slices as pointers or adapt Storage
	initResponseQueue()

	for id := range ids {
		queue := make([]int64, 0)
		UpdateResponseQueue(id, queue)
	}
}

func ConfiguredDatabases() int {
	return len(dbList)
}

func ParseDBConfig(databases DatabaseTypes) error {
	v := reflect.ValueOf(databases)
	var errors []error
	for i := 0; i < v.NumField(); i++ {
		dbString := fmt.Sprint(v.Field(i).Interface().(Database))

		// Check whether notify object is empty. if its not empty add to the list
		if !isEmptyObject(dbString) {
			// dbList = append(dbList, v.Field(i).Interface().(Database))
			if err := AddNew(v.Field(i).Interface().(Database)); err != nil {
				errors = append(errors, err)
			}
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("Got %d errors during database init. Errors: %v", len(errors), errors)
	}
	if len(dbList) < 1 {
		fmt.Println("No Database selected.")
	}
	return nil
}

// Add database to the database List
func AddNew(database Database) error {
	// Intialize and database given by user by calling the initialize method
	if initErr := database.Initialize(); initErr != nil {
		return fmt.Errorf("Failed to Intialize Database %s, err: %s\n", database.GetDatabaseName(), initErr)
	}

	if writeErr := addTestErrorAndRequestInfo(database); writeErr != nil {
		return fmt.Errorf("Failed to access Database %s, err: %s\n", database.GetDatabaseName(), writeErr)
	}

	dbList = append(dbList, database)
	return nil
}

// Insert test data to database
func addTestErrorAndRequestInfo(db Database) error {
	fmt.Printf("Adding Test data to your database %s...\n", db.GetDatabaseName())

	requestInfo := RequestInfo{0, "http://test.com", "GET", 0, 0, 0}

	if reqErr := db.AddRequestInfo(requestInfo); reqErr != nil {
		return fmt.Errorf("InfluxDB: Failed to insert Error Info to database %s, error: %s. Please check whether database is installed properly!\n", db.GetDatabaseName(), reqErr)
	}

	errorInfo := ErrorInfo{0, "http://test.com", "GET", 0, "test response", errors.New("test error"), "test other info"}

	if errErr := db.AddErrorInfo(errorInfo); errErr != nil {
		return fmt.Errorf("InfluxDB: Failed to insert Error Info to database %s, error: %s. Please check whether database is installed properly!\n", db.GetDatabaseName(), errErr)
	}
	return nil
}

// This function is called by requests package when request has been successfully performed
// Request data is inserted to all the registered databases
func AddRequestInfo(requestInfo RequestInfo) {
	logRequestInfo(requestInfo)

	// Response time to queue
	AddResponseTimeToRequest(requestInfo.Id, requestInfo.ResponseTimeMs)

	// Insert to all configured db's
	for _, db := range dbList {
		go db.AddRequestInfo(requestInfo)
	}

	if CountResponsesInQueue(requestInfo.Id) < MinResponseCount {
		return
	}

	// calculate current mean response time . if its less than expected send notitifcation
	// mean, meanErr := GetMeanResponseTimeOfUrl(requestInfo.Id)
	mean, meanErr := GetMedianResponseTimeOfUrl(requestInfo.Id)

	if meanErr == nil {
		ClearQueue(requestInfo.Id)
		if mean > requestInfo.ExpectedResponseTime {
			// TODO :error retry  exponential?
			notify.SendResponseTimeNotification(notify.ResponseTimeNotification{
				Url:                    requestInfo.Url,
				RequestType:            requestInfo.RequestType,
				ExpectedResponsetimeMs: requestInfo.ExpectedResponseTime,
				MeanResponseTimeMs:     mean,
			})
		}
	}
}

// This function is called by requests package when a reuquest fails
// Error Information is inserted to all the registered databases
func AddErrorInfo(errorInfo ErrorInfo) {
	logErrorInfo(errorInfo)

	// Request failed send notification
	// TODO :error retry  exponential?
	notify.SendErrorNotification(notify.ErrorNotification{
		Url:          errorInfo.Url,
		RequestType:  errorInfo.RequestType,
		ResponseBody: errorInfo.ResponseBody,
		Error:        errorInfo.Reason.Error(),
		OtherInfo:    errorInfo.OtherInfo,
	})

	// Add Error information to database
	for _, db := range dbList {
		go db.AddErrorInfo(errorInfo)
	}
}

func initResponseQueue() {
	responseQueue = make(map[int][]int64)
}

func CountResponsesInQueue(id int) int {
	if len(responseQueue) > 0 {
		return len(responseQueue[id])
	}
	return 0
}

func GetResponseQueue(id int) []int64 {
	if responseQueue == nil {
		initResponseQueue()
	}

	if len(responseQueue) > 0 {
		return responseQueue[id]
	}
	return []int64{}
}

func UpdateResponseQueue(id int, queue []int64) {
	responseQueue[id] = queue
}

func AddResponseTimeToRequest(id int, responseTime int64) {
	if responseQueue == nil {
		return
	}
	queue := GetResponseQueue(id)

	if len(queue) == MinResponseCount {
		queue = queue[1:]
	}
	queue = append(queue, responseTime)

	UpdateResponseQueue(id, queue)
}

// Calculate current  mean response time for the given request id
func GetMeanResponseTimeOfUrl(id int) (int64, error) {
	if CountResponsesInQueue(id) < MinResponseCount {
		return 0, fmt.Errorf("The number of requests %d has not been reached the minResponseCount %d yet.", CountResponsesInQueue(id), MinResponseCount)
	}

	queue := GetResponseQueue(id)
	var sum int64

	for _, val := range queue {
		sum = sum + val
	}

	return sum / int64(len(queue)), nil
}

// Calculate current median response time for the given request id
func GetMedianResponseTimeOfUrl(id int) (int64, error) {
	if CountResponsesInQueue(id) < MinResponseCount {
		return 0, fmt.Errorf("The number of requests %d has not been reached the minResponseCount %d yet.", CountResponsesInQueue(id), MinResponseCount)
	}

	queue := GetResponseQueue(id)

	if len(queue) == 1 {
		return queue[0], nil
	}

	// sort the numbers
	sort.Slice(queue, func(i, j int) bool { return queue[i] < queue[j] })

	mNumber := len(queue) / 2

	if len(queue)%2 != 0 {
		return queue[mNumber], nil
	}

	return (queue[mNumber-1] + queue[mNumber]) / 2, nil
}

func ClearQueue(id int) {
	UpdateResponseQueue(id, make([]int64, 0))
}

func isEmptyObject(objectString string) bool {
	objectString = strings.Replace(objectString, "0", "", -1)
	objectString = strings.Replace(objectString, "map", "", -1)
	objectString = strings.Replace(objectString, "[]", "", -1)
	objectString = strings.Replace(objectString, " ", "", -1)

	if len(objectString) > 2 {
		return false
	} else {
		return true
	}
}

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

func logErrorInfo(errorInfo ErrorInfo) {
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

func logRequestInfo(requestInfo RequestInfo) {
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
