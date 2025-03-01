package requests

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"statusok/database"
	"statusok/model"
	"strconv"
	"time"
)

var (
	RequestsList   []RequestConfig
	requestChannel chan RequestConfig
	throttle       chan int
)

const (
	ContentType     = "Content-Type"
	ContentLength   = "Content-Length"
	UserAgent       = "User-Agent"
	FormContentType = "application/x-www-form-urlencoded"
	JsonContentType = "application/json"

	DefaultTime         = "300s"
	DefaultTimeout      = "10s"
	DefaultResponseCode = http.StatusOK
	DefaultConcurrency  = 1
	DefaultUserAgent    = "Kreapptivo/Monitoring v1.0b"
)

type RequestConfig struct {
	Id                  int
	Url                 string            `json:"url"`
	RequestType         string            `json:"requestType"`
	Headers             map[string]string `json:"headers"`
	FormParams          map[string]string `json:"formParams"`
	UrlParams           map[string]string `json:"urlParams"`
	ResponseCode        int               `json:"responseCode"`
	ResponseTime        int64             `json:"responseTime"`
	CheckEvery          string            `json:"checkEvery"`
	_checkEvery         time.Duration     `json:"-"`
	Timeout             string            `json:"timeout"`
	_timeout            time.Duration     `json:"-"`
	MedianResponseCount int               `json:"medianResponseCount"`
}

// Set Id for request
func (requestConfig *RequestConfig) SetId(id int) {
	requestConfig.Id = id
}

// check whether all requestConfig fields are valid
func (requestConfig *RequestConfig) Validate() error {
	if len(requestConfig.Url) == 0 {
		return errors.New("Invalid Url")
	}

	if _, err := url.Parse(requestConfig.Url); err != nil {
		return errors.New("Invalid Url")
	}

	if len(requestConfig.RequestType) == 0 {
		return errors.New("RequestType cannot be empty")
	}

	if requestConfig.ResponseTime == 0 {
		return errors.New("ResponseTime cannot be empty")
	}

	if requestConfig.ResponseCode == 0 {
		requestConfig.ResponseCode = DefaultResponseCode
	}

	if len(requestConfig.CheckEvery) == 0 {
		requestConfig.CheckEvery = DefaultTime
	}
	var err error
	if requestConfig._checkEvery, err = time.ParseDuration(requestConfig.CheckEvery); err != nil {
		return fmt.Errorf("CheckEvery format is invalid %s", err)
	}
	fmt.Printf("Check every: %s\n", fmtDuration(requestConfig._checkEvery))

	if len(requestConfig.Timeout) == 0 {
		requestConfig.Timeout = DefaultTimeout
	}
	if requestConfig._timeout, err = time.ParseDuration(requestConfig.Timeout); err != nil {
		return fmt.Errorf("Timeout format is invalid %s", err)
	}
	fmt.Printf("Request timeout: %s\n", fmtDuration(requestConfig._timeout))

	return nil
}

func fmtDuration(d time.Duration) string {
	d = d.Round(time.Second)
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	m -= m * time.Minute
	s := d / time.Second
	return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
}

// Initialize data from config file and check all requests
func RequestsInit(data []RequestConfig, concurrency int) {
	RequestsList = data

	// throttle channel is used to limit number of requests performed at a time
	if concurrency == 0 {
		throttle = make(chan int, DefaultConcurrency)
	} else {
		throttle = make(chan int, concurrency)
	}

	requestChannel = make(chan RequestConfig, len(data))

	if len(data) == 0 {
		fmt.Println("\nNo requests to monitor. Please add requests to you config file!")
		os.Exit(3)
	}
	// send requests to make sure every every request is valid
	fmt.Println("\nSending requests to apis.....making sure everything is right before we start monitoring")
	fmt.Println("Api Count: ", len(data))

	for i, requestConfig := range data {
		fmt.Printf("Request #%d:%s %s\n", i, requestConfig.RequestType, requestConfig.Url)

		// Perform request
		reqErr := PerformRequest(requestConfig, nil)

		if reqErr != nil {
			// Request Failed
			fmt.Println("Request Failed !!!! Not able to perfome below request:")
			fmt.Println("----Request Deatails---")
			fmt.Printf("Url: %s\nType: %s\nError: %s\n", requestConfig.Url, requestConfig.RequestType, reqErr)
			fmt.Println("Please check the config file and try again!")
			os.Exit(3)
		}
	}

	fmt.Println("All requests Successfull")
}

// Start monitoring by calling createTicker method for each request
func StartMonitoring() {
	fmt.Printf("Started Monitoring %d apis .....\n", len(RequestsList))

	go listenToRequestChannel()

	for _, requestConfig := range RequestsList {
		go createTicker(requestConfig)
	}
}

// A time ticker writes data to request channel for every request.CheckEvery seconds
func createTicker(requestConfig RequestConfig) {
	var ticker *time.Ticker = time.NewTicker(requestConfig._checkEvery)
	quit := make(chan struct{})
	for {
		select {
		case <-ticker.C:
			requestChannel <- requestConfig
		case <-quit:
			ticker.Stop()
			return
		}
	}
}

// all tickers write to request channel
// here we listen to request channel and perfom each request
func listenToRequestChannel() {
	// throttle is used to limit number of requests executed at a time
	for {
		select {
		case request := <-requestChannel:
			throttle <- 1
			go PerformRequest(request, throttle)
		}
	}
}

// takes the date from requestConfig and creates http request and executes it
func PerformRequest(requestConfig RequestConfig, throttle chan int) error {
	// Remove value from throttle channel when request is completed
	defer func() {
		if throttle != nil {
			<-throttle
		}
	}()

	var request *http.Request
	var reqErr error

	if len(requestConfig.FormParams) == 0 {
		// formParams create a request
		request, reqErr = http.NewRequest(requestConfig.RequestType,
			requestConfig.Url,
			nil)
	} else {
		if requestConfig.Headers[ContentType] == JsonContentType {
			// create a request using using formParams

			jsonBody, jsonErr := GetJsonParamsBody(requestConfig.FormParams)
			if jsonErr != nil {
				// Not able to create Request object.Add Error to Database
				go database.AddErrorInfo(model.ErrorInfo{
					Id:           requestConfig.Id,
					Url:          requestConfig.Url,
					RequestType:  requestConfig.RequestType,
					ResponseCode: 0,
					ResponseBody: "",
					Reason:       database.ErrCreateRequest,
					OtherInfo:    jsonErr.Error(),
				})

				return jsonErr
			}
			request, reqErr = http.NewRequest(requestConfig.RequestType,
				requestConfig.Url,
				jsonBody)

		} else {
			// create a request using formParams
			formParams := GetUrlValues(requestConfig.FormParams)

			request, reqErr = http.NewRequest(requestConfig.RequestType,
				requestConfig.Url,
				bytes.NewBufferString(formParams.Encode()))

			request.Header.Add(ContentLength, strconv.Itoa(len(formParams.Encode())))

			if requestConfig.Headers[ContentType] != "" {
				// Add content type to header if user doesnt mention it config file
				// Default content type application/x-www-form-urlencoded
				request.Header.Add(ContentType, FormContentType)
			}

		}
	}

	if requestConfig.Headers[UserAgent] != "" {
		request.Header.Add(UserAgent, requestConfig.Headers[UserAgent])
	} else {
		request.Header.Add(UserAgent, DefaultUserAgent)
	}

	if reqErr != nil {
		// Not able to create Request object.Add Error to Database
		go database.AddErrorInfo(model.ErrorInfo{
			Id:           requestConfig.Id,
			Url:          requestConfig.Url,
			RequestType:  requestConfig.RequestType,
			ResponseCode: 0,
			ResponseBody: "",
			Reason:       database.ErrCreateRequest,
			OtherInfo:    reqErr.Error(),
		})

		return reqErr
	}

	// add url parameters to query if present
	if len(requestConfig.UrlParams) != 0 {
		urlParams := GetUrlValues(requestConfig.UrlParams)
		request.URL.RawQuery = urlParams.Encode()
	}

	// Add headers to the request
	AddHeaders(request, requestConfig.Headers)

	client := &http.Client{
		Timeout: requestConfig._timeout,
	}
	start := time.Now()

	getResponse, respErr := client.Do(request)

	if respErr != nil {
		// Request failed . Add error info to database
		var statusCode int
		if getResponse == nil {
			statusCode = 0
		} else {
			statusCode = getResponse.StatusCode
		}
		go database.AddErrorInfo(model.ErrorInfo{
			Id:           requestConfig.Id,
			Url:          requestConfig.Url,
			RequestType:  requestConfig.RequestType,
			ResponseCode: statusCode,
			ResponseBody: convertResponseToString(getResponse),
			Reason:       database.ErrDoRequest,
			OtherInfo:    respErr.Error(),
		})
		return respErr
	}

	defer getResponse.Body.Close()

	if getResponse.StatusCode != requestConfig.ResponseCode {
		// Response code is not the expected one .Add Error to database
		go database.AddErrorInfo(model.ErrorInfo{
			Id:           requestConfig.Id,
			Url:          requestConfig.Url,
			RequestType:  requestConfig.RequestType,
			ResponseCode: getResponse.StatusCode,
			ResponseBody: convertResponseToString(getResponse),
			Reason:       errResponseCode(getResponse.StatusCode, requestConfig.ResponseCode),
			OtherInfo:    "",
		})
		return errResponseCode(getResponse.StatusCode, requestConfig.ResponseCode)
	}

	elapsed := time.Since(start)

	// Request succesfull. Add entry to Database
	go database.AddRequestInfo(model.RequestInfo{
		Id:                   requestConfig.Id,
		Url:                  requestConfig.Url,
		RequestType:          requestConfig.RequestType,
		ResponseCode:         getResponse.StatusCode,
		ResponseTimeMs:       elapsed.Milliseconds(),
		ExpectedResponseTime: requestConfig.ResponseTime,
	})

	return nil
}

// convert response body to string
func convertResponseToString(resp *http.Response) string {
	if resp == nil {
		return " "
	}
	buf := new(bytes.Buffer)
	_, bufErr := buf.ReadFrom(resp.Body)

	if bufErr != nil {
		return " "
	}

	return buf.String()
}

// Add header values from map to request
func AddHeaders(req *http.Request, headers map[string]string) {
	for key, value := range headers {
		req.Header.Add(key, value)
	}
}

// convert params in map to url.Values
func GetUrlValues(params map[string]string) url.Values {
	urlParams := url.Values{}
	i := 0
	for key, value := range params {
		if i == 0 {
			urlParams.Set(key, value)
		} else {
			urlParams.Add(key, value)
		}
	}

	return urlParams
}

// Creates body for request of type application/json from map
func GetJsonParamsBody(params map[string]string) (io.Reader, error) {
	data, jsonErr := json.Marshal(params)

	if jsonErr != nil {

		jsonErr = errors.New("Invalid Parameters for Content-Type application/json : " + jsonErr.Error())

		return nil, jsonErr
	}

	return bytes.NewBuffer(data), nil
}

// creates an error when response code from server is not equal to response code mentioned in config file
func errResponseCode(status int, expectedStatus int) error {
	return fmt.Errorf("Got Response code %v. Expected Response Code %v ", status, expectedStatus)
}
