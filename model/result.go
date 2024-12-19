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

func Fail(c *gin.Context, msg string) {
	r := &Result[any]{
		Code: 500,
		Msg:  msg,
		Time: time.Now().Format("2006-01-02 15:04:05"),
	}
	c.AbortWithStatusJSON(http.StatusInternalServerError, r)
}

type T any

func Success[T any](c *gin.Context, data T) {
	r := &Result[T]{
		Code: 200,
		Msg:  "Success",
		Data: data,
		Time: time.Now().Format("2006-01-02 15:04:05"),
	}
	c.JSON(http.StatusOK, r)
}

func BadRequest(c *gin.Context) {
	r := &Result[any]{
		Code: 400,
		Msg:  "Bad Request",
		Time: time.Now().Format("2006-01-02 15:04:05"),
	}
	c.AbortWithStatusJSON(http.StatusBadRequest, r)
}

func Forbidden(c *gin.Context) {
	r := &Result[any]{
		Code: 403,
		Msg:  "Forbidden",
		Time: time.Now().Format("2006-01-02 15:04:05"),
	}
	c.AbortWithStatusJSON(http.StatusForbidden, r)
}
