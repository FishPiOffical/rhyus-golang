package model

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"time"
)

// Result represents a common-used result struct.
type Result struct {
	Code int    `json:"code"` // return code
	Msg  string `json:"msg"`  // message
	Data any    `json:"data"` // data object
	Time string `json:"time"` // time
}

func Fail(msg string) *Result {
	return &Result{
		Code: 500,
		Msg:  msg,
		Time: time.Now().Format("2006-01-02 15:04:05"),
	}
}

func Success(data any) *Result {
	return &Result{
		Code: 200,
		Msg:  "Success",
		Data: data,
		Time: time.Now().Format("2006-01-02 15:04:05"),
	}
}

func BadRequest(c *gin.Context) {
	r := &Result{
		Code: 400,
		Msg:  "Bad Request",
		Time: time.Now().Format("2006-01-02 15:04:05"),
	}
	c.AbortWithStatusJSON(http.StatusBadRequest, r)
}

func Forbidden(c *gin.Context) {
	r := &Result{
		Code: 403,
		Msg:  "Forbidden",
		Time: time.Now().Format("2006-01-02 15:04:05"),
	}
	c.AbortWithStatusJSON(http.StatusForbidden, r)
}
