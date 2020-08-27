package main

import (
	"encoding/json"
	"net/http"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	serviceLambda "github.com/aws/aws-sdk-go/service/lambda"
	"github.com/joho/godotenv"
)

var client *serviceLambda.Lambda
var db *dynamodb.DynamoDB

var tableName = os.Getenv("DYNAMODB_TABLE_NAME")

const (
	statusCreated = "CREATED"
)

type task struct {
	RequestID string
	Result    string
	Status    string
}

// WriteToDB writes the given to the Database.
func (t *task) WriteToDB() error {
	request, err := dynamodbattribute.MarshalMap(t)
	if err != nil {
		return err
	}

	_, err = db.PutItem(&dynamodb.PutItemInput{
		TableName: aws.String(tableName),
		Item:      request,
	})
	if err != nil {
		return err
	}

	return nil
}

// Proxy invokes the given Lambda function with the given payload.
// When the result is arrived, writes it to the DynamoDB.
func Proxy(request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	var t = new(task)
	t.RequestID = request.Headers["RequestID"]

	// Invoke the lambda function as sync.
	payload, _ := json.Marshal(request)
	response, err := client.Invoke(
		&serviceLambda.InvokeInput{
			FunctionName: aws.String(os.Getenv("MAIN_LAMBDA_NAME")),
			Payload:      payload,
		},
	)
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
			Body:       `{"error": "There is an issue with Lambda Function."}`,
		}, nil
	}

	t.Status = statusCreated
	t.Result = string(response.Payload)

	err = t.WriteToDB()
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
			Body:       `{"error": "There is an issue with DynamoDB."}`,
		}, nil
	}

	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusCreated,
		Body:       `{"message": "The result was created."}`,
	}, nil
}

func main() {
	_ = godotenv.Load()

	sess := session.Must(session.NewSession())

	client = serviceLambda.New(sess)
	db = dynamodb.New(sess)

	lambda.Start(Proxy)
}
