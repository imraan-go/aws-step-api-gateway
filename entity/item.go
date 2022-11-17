package entity

type Item struct {
	ItemId string `dynamodbav:"ItemId" json:"ItemId"`
	Count  int    `dynamodbav:"Count" json:"Count"`
}
