package entity

type LambdaError struct {
	ErrorMessage string   `json:"errorMessage"`
	ErrorType    string   `json:"errorType"`
	RequestID    string   `json:"requestId"`
	StackTrace   []string `json:"stackTrace"`
}
