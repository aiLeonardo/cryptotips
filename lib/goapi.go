package lib

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type DatalistReq struct {
	Page          int   `json:"page" binding:"required,gte=1"`
	MaxUpdatetime int64 `json:"maxId" binding:"gte=0"`

	Limit int    `json:"limit" binding:"required,gte=1,lte=100"`
	Sort  string `json:"sort"`
	Type  string `json:"type" binding:"required"`
	From  string `json:"from" binding:"required"`
	Sign  string `json:"sign" binding:"required"`
	Date  string `json:"date"`
}

// response 是一个统一的 API 响应结构体
type response struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data"` // 使用 interface{} 来支持任何类型的数据
}

// 定义一些业务状态码
const (
	codeSuccess = 0
	codeError   = 1
)

func JsonResponse(c *gin.Context, data any) {
	resp := response{
		Code:    codeSuccess,
		Message: "success",
		Data:    data,
	}

	c.JSON(http.StatusOK, resp)
}

func StrResponse(c *gin.Context) {
	c.String(http.StatusOK, "success")
}

func JsonError(c *gin.Context, err error) {
	resp := response{
		Code:    codeError,
		Message: err.Error(),
		Data:    nil,
	}
	c.JSON(http.StatusOK, resp)
}
