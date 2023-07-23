package response

import (
	"encoding/json"
	"game-3-card-poker/server/constant"
	"game-3-card-poker/server/service"
	"io"
	"net/http"
)

func Response(code int, data any, message string, w http.ResponseWriter) {
	response := service.Response{
		Code:    code,
		Data:    data,
		Message: message,
	}
	marshal, _ := json.Marshal(response)
	io.WriteString(w, string(marshal))
}

// Success 返回成功
func Success(w http.ResponseWriter) {
	Response(constant.Code10000, nil, constant.OK, w)
}

func SuccessWithMsg(msg string, w http.ResponseWriter) {
	Response(constant.Code10000, nil, msg, w)
}

func SuccessWithData(data any, w http.ResponseWriter) {
	Response(constant.Code10000, data, constant.OK, w)
}

// Fail 返回失败
func Fail(code int, message string, w http.ResponseWriter) {
	Response(code, nil, message, w)
}

// Error 返回错误
func Error(err error, w http.ResponseWriter) {
	Response(constant.Code99999, nil, err.Error(), w)
}

// ParamError 参数错误
func ParamError(w http.ResponseWriter) {
	Response(constant.Code10001, nil, constant.ParamError, w)
}

func SystemError(w http.ResponseWriter) {
	Response(constant.Code99999, nil, constant.Error, w)
}
