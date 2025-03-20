package model

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"time"
)

// Result represents a common-used result struct.
type Result[T any] struct {
	Code int    `json:"code"` // return code
	Msg  string `json:"msg"`  // message
	Data T      `json:"data"` // data object
	Time string `json:"time"` // time
}

func NewResult[T any](code int, msg string, data *T) *Result[*T] {
	return &Result[*T]{
		Code: code,
		Msg:  msg,
		Data: data,
		Time: time.Now().Format("2006-01-02 15:04:05"),
	}
}

func Fail(c *gin.Context, msg string) {
	r := NewResult[any](500, msg, nil)
	c.AbortWithStatusJSON(http.StatusInternalServerError, r)
}

type T any

func Success[T any](c *gin.Context, data T) {
	r := NewResult[T](200, "OK", &data)
	c.JSON(http.StatusOK, r)
}

func BadRequest(c *gin.Context) {
	r := NewResult[any](400, "Bad Request", nil)
	c.AbortWithStatusJSON(http.StatusBadRequest, r)
}

func Forbidden(c *gin.Context) {
	r := NewResult[any](403, "Forbidden", nil)
	c.AbortWithStatusJSON(http.StatusForbidden, r)
}

func TooManyRequests(c *gin.Context) {
	r := NewResult[any](429, "Too Many Requests", nil)
	c.AbortWithStatusJSON(http.StatusTooManyRequests, r)
}
