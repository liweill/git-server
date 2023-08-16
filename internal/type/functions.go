package _type

// 失败结果
func FaildResult(err error) *ResultType {
	errMsg := "未知异常"
	if err != nil {
		errMsg = err.Error()
	}
	return &ResultType{
		Code:    500,
		Data:    nil,
		Success: false,
		Msg:     errMsg,
	}
}

// 成功结果
func SuccessResult(data any) *ResultType {
	return &ResultType{
		Code:    200,
		Data:    data,
		Success: true,
		Msg:     "success",
	}
}
