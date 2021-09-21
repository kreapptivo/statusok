package model

type ErrorInfo struct {
	Id           int
	Url          string
	RequestType  string
	ResponseCode int
	ResponseBody string
	Reason       error
	OtherInfo    string
}
