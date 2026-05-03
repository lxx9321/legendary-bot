package models

type ResponseResult struct {
	Code    int64
	Success bool
	Message string
	Data    interface{}
	Data62  string
	Debug   string
	ID      uint64
}

type ResponseResult2 struct {
	Code     int64
	Success  bool
	Message  string
	Data     interface{}
	Data62   string
	DeviceId string
}

type SessionidQRParam struct {
	Code     int64
	Success  bool
	Message  string
	Data     interface{}
	Data62   string
	DeviceId string
}
