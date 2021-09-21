package model

type RequestInfo struct {
	Id                   int
	Url                  string
	RequestType          string
	ResponseCode         int
	ResponseTimeMs       int64
	ExpectedResponseTime int64
}
