package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/imraan-go/aws-step-order-service/entity"
	"github.com/labstack/echo/v4"
)

func setupRoutes(e *echo.Echo) {
	e.GET("/", func(c echo.Context) error {
		return c.JSON(200, "Order service running successfully!")
	})
	e.GET("/tables", tablesHandler)
	e.GET("/getItem/:itemId", getItemHandler)
	e.POST("/order", orderHandler)
}

func orderHandler(c echo.Context) error {
	data := &entity.CreateOrderRequest{}
	err := c.Bind(data)
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{
			"error":   "invalid.payload",
			"message": err.Error(),
		})
	}

	jsonData, err := json.Marshal(&data)
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{
			"error":   "payload.error",
			"message": err.Error(),
		})
	}
	// *Order Validation*
	// Invoke  order validation lambda
	output, err := lambdaClient.Invoke(context.Background(), &lambda.InvokeInput{
		FunctionName: aws.String("validateOrder"),
		Payload:      jsonData,
	})
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{
			"error":   "validateOrder.error",
			"message": err.Error(),
		})
	}
	// *Check Order Validation Result*

	if output.StatusCode != 200 {
		// Something went wrong
		return c.JSON(http.StatusBadRequest, echo.Map{
			"error":   "validateOrder.error",
			"message": "Something went wrong with status code " + strconv.Itoa(int(output.StatusCode)),
		})
	}

	// *Notify Order Creation Result*
	// publish to sns

	snsInput := &sns.PublishInput{
		Message:  aws.String(string(output.Payload)),
		TopicArn: aws.String("arn:aws:sns:us-west-2:157984242284:OrderCreation"),
	}

	result, err := snsClient.Publish(context.TODO(), snsInput)
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{
			"error":   "sns.error",
			"message": err.Error(),
		})
	}

	fmt.Println("OrderCreation Message ID: " + *result.MessageId)

	// *Charge Customer*
	// Invoke  chargeCustomer lambda
	output, err = lambdaClient.Invoke(context.Background(), &lambda.InvokeInput{
		FunctionName: aws.String("chargeCustomer"),
		Payload:      output.Payload,
	})
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{
			"error":   "chargeCustomer.error",
			"message": err.Error(),
		})
	}

	// *Check Charge Customer Result*

	if output.StatusCode != 200 {
		return c.JSON(http.StatusBadRequest, echo.Map{
			"error":   "chargeCustomer.error",
			"message": "Something went wrong with status code " + strconv.Itoa(int(output.StatusCode)),
		})
	}

	chargeCustomerResponse := entity.CreateOrderResponse{}
	err = json.Unmarshal(output.Payload, &chargeCustomerResponse)
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{
			"error":   "chargeCustomer.json.error",
			"message": err.Error(),
		})
	}

	if chargeCustomerResponse.PaymentStatus == "Pending" {
		// Retry
		log.Println("chargeCustomer Retry")
		return nil
	}

	if chargeCustomerResponse.PaymentStatus != "Paid" {
		log.Println("chargeCustomer Something went wrong")
		return nil
	}

	// *Save Order in Customer Bill History Table*
	// save to dynamodb CustomerBillHistoryTable
	_, err = dynamoClient.PutItem(context.TODO(), &dynamodb.PutItemInput{
		TableName: aws.String("CustomerBillHistoryTable"),
		Item: map[string]types.AttributeValue{
			"OrderId":           &types.AttributeValueMemberS{Value: data.Order.OrderID},
			"TransactionAmount": &types.AttributeValueMemberS{Value: chargeCustomerResponse.OrderTotal.Amount},
			"CardNumber":        &types.AttributeValueMemberS{Value: chargeCustomerResponse.Payment.CardNumber},
			"ChargeTimeStamp":   &types.AttributeValueMemberS{Value: chargeCustomerResponse.Payment.ChargeCustomerTimestamp},
			"Currency":          &types.AttributeValueMemberS{Value: chargeCustomerResponse.OrderTotal.CurrencyCode},
		},
	})

	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{
			"error":   "CustomerBillHistoryTable.error",
			"message": err.Error(),
		})
	}

	// *Notify Order Payment Result*
	// publish to sns
	snsInput = &sns.PublishInput{
		Message:  aws.String(string(output.Payload)),
		TopicArn: aws.String("arn:aws:sns:us-west-2:157984242284:OrderPayment"),
	}

	result, err = snsClient.Publish(context.TODO(), snsInput)
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{
			"error":   "sns.error",
			"message": err.Error(),
		})
	}

	fmt.Println("OrderPayment Message ID: " + *result.MessageId)

	// *Start Shipment*
	// Invoke  shipment lambda
	output, err = lambdaClient.Invoke(context.Background(), &lambda.InvokeInput{
		FunctionName: aws.String("shipment"),
		Payload:      output.Payload,
	})

	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{
			"error":   "shipment.error",
			"message": err.Error(),
		})
	}

	// *
	if output.StatusCode != 200 {
		return c.JSON(http.StatusBadRequest, echo.Map{
			"error":   "shipment.error",
			"message": "Something went wrong with status code " + strconv.Itoa(int(output.StatusCode)),
		})
	}

	shipmentResponse := entity.CreateOrderResponse{}
	err = json.Unmarshal(output.Payload, &shipmentResponse)
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{
			"error":   "shipment.json.error",
			"message": err.Error(),
		})
	}
	if shipmentResponse.ErrorMessage != "" {
		return c.JSON(http.StatusBadRequest, echo.Map{
			"error":   "shipment.lambda.error",
			"message": strings.Join(shipmentResponse.StackTrace, "-"),
		})
	}
	if shipmentResponse.StatusCode != 200 {
		return c.JSON(http.StatusBadRequest, echo.Map{
			"error":   "shipment.error",
			"message": shipmentResponse.Body,
		})
	}

	// *Save Shipment Order*
	// save to dynamodb ShipmentHistoryTable
	_, err = dynamoClient.PutItem(context.TODO(), &dynamodb.PutItemInput{
		TableName: aws.String("ShipmentHistoryTable"),
		Item: map[string]types.AttributeValue{
			"OrderId": &types.AttributeValueMemberS{Value: data.Order.OrderID},
			// "ShipmentId": &types.AttributeValueMemberS{Value: chargeCustomerResponse.DeliveryDetails},
			"CardNumber":      &types.AttributeValueMemberS{Value: chargeCustomerResponse.Payment.CardNumber},
			"ChargeTimeStamp": &types.AttributeValueMemberS{Value: chargeCustomerResponse.Payment.ChargeCustomerTimestamp},
			"Currency":        &types.AttributeValueMemberS{Value: chargeCustomerResponse.OrderTotal.CurrencyCode},
		},
	})

	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{
			"error":   "CustomerBillHistoryTable.error",
			"message": err.Error(),
		})
	}

	// success

	// *Notify Order Payment Result*
	// publish to sns
	snsInput = &sns.PublishInput{
		Message:  aws.String(string(output.Payload)),
		TopicArn: aws.String("arn:aws:sns:us-west-2:157984242284:OrderShipment"),
	}

	result, err = snsClient.Publish(context.TODO(), snsInput)
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{
			"error":   "sns.error",
			"message": err.Error(),
		})
	}

	fmt.Println("OrderShipment Message ID: " + *result.MessageId)

	return c.JSON(200, echo.Map{
		"success": true,
		"message": "Order palced successfully",
		"data":    json.RawMessage(output.Payload),
	})

}

func tablesHandler(c echo.Context) error {
	resp, err := dynamoClient.ListTables(context.TODO(), &dynamodb.ListTablesInput{
		Limit: aws.Int32(5),
	})
	if err != nil {
		log.Fatalf("failed to list tables, %v", err)
	}

	fmt.Println("Tables:")
	for _, tableName := range resp.TableNames {
		fmt.Println(tableName)
	}

	return c.JSON(200, resp)

}

func getItemHandler(c echo.Context) error {
	itemId := c.Param("itemId")
	fmt.Println(itemId)

	tableName := "Inventory"

	resp, err := dynamoClient.GetItem(context.TODO(), &dynamodb.GetItemInput{
		TableName: &tableName,
		Key: map[string]types.AttributeValue{
			"ItemId": &types.AttributeValueMemberS{Value: itemId},
		},
	})

	if err != nil {
		panic(err)
	}

	if err != nil {
		log.Fatalf("failed to list tables, %v", err)
	}

	return c.JSON(200, resp.Item)

}
