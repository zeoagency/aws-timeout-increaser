package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	serviceLambda "github.com/aws/aws-sdk-go/service/lambda"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
)

var client *serviceLambda.Lambda
var db *dynamodb.DynamoDB

var tableName = os.Getenv("DYNAMODB_TABLE_NAME")
var stageName = os.Getenv("STAGE_NAME")

const (
	statusCreated = "CREATED"
	statusPending = "PENDING"
)

type task struct {
	RequestID string
	Result    string
	Status    string
}

// InitTaskOnDB inits a new row for the given task with pending status.
func (t *task) InitTaskOnDB() error {
	t.RequestID = uuid.New().String()
	t.Status = statusPending
	t.Result = ""

	request, err := dynamodbattribute.MarshalMap(t)
	if err != nil {
		return errors.New("There is an issue with marshalling the task.")
	}

	_, err = db.PutItem(&dynamodb.PutItemInput{
		TableName: aws.String(tableName),
		Item:      request,
	})

	if err != nil {
		return errors.New("There is an issue with DynamoDB.")
	}

	return nil
}

// ReadFromDB reads from the Database for the given requestID.
func (t *task) ReadFromDB() (int, error) {
	result, err := db.GetItem(&dynamodb.GetItemInput{
		TableName: aws.String(tableName),
		Key: map[string]*dynamodb.AttributeValue{
			"RequestID": {
				S: aws.String(t.RequestID),
			},
		},
	})
	if err != nil {
		return http.StatusInternalServerError, errors.New("There is an issue with DynamoDB.")
	}
	if result.Item == nil {
		return http.StatusNotFound, errors.New("RequestID is wrong.")
	}

	err = dynamodbattribute.UnmarshalMap(result.Item, t)
	if err != nil {
		return http.StatusInternalServerError, errors.New("There is an issue with unmarshalling the task.")
	}

	return http.StatusOK, nil
}

// Delete deletes the task from the DynamoDB table.
func (t *task) Delete() error {
	input := &dynamodb.DeleteItemInput{
		TableName: aws.String(tableName),
		Key: map[string]*dynamodb.AttributeValue{
			"RequestID": {
				S: aws.String(t.RequestID),
			},
		},
	}

	_, err := db.DeleteItem(input)
	if err != nil {
		return err
	}

	return nil
}

// Proxy returns 303 until the response's status become CREATED.
// When a new request arrived, it creates a new row with PENDING status on DynamoDB.
// It checks the DB every 2 seconds, the duration is ~22-24 seconds at max.
func Proxy(request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	startTime := time.Now()

	var t = new(task)
	t.RequestID = request.QueryStringParameters["requestID"]

	if t.RequestID == "" {
		// That means that is new request.
		//
		// Create task on the DB with a new RequestID.
		err := t.InitTaskOnDB()
		if err != nil {
			return events.APIGatewayProxyResponse{
				StatusCode: http.StatusInternalServerError,
				Body:       `{"error": "` + err.Error() + `"}`,
			}, nil
		}

		// Invoke the lambda function as async.
		request.Headers["RequestID"] = t.RequestID
		payload, _ := json.Marshal(request)
		_, err = client.Invoke(
			&serviceLambda.InvokeInput{
				FunctionName:   aws.String(os.Getenv("LAMBDA_TALKER_NAME")),
				InvocationType: aws.String("Event"),
				Payload:        payload,
			},
		)
		if err != nil {
			return events.APIGatewayProxyResponse{
				StatusCode: http.StatusInternalServerError,
				Body:       `{"error": There is an issue with Lambda(Talker)."}`,
			}, nil
		}
	}

	if time.Since(startTime) > 16*time.Second {
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
			Body:       `{"error": There is an issue with DynamobDB."}`,
		}, nil
	}

	for ; time.Since(startTime) < 22*time.Second; time.Sleep(2 * time.Second) {
		status, err := t.ReadFromDB()
		if err != nil {
			return events.APIGatewayProxyResponse{
				StatusCode: status,
				Body:       `{"error": "` + err.Error() + `"}`,
			}, nil
		}

		if t.Status == statusCreated { // Return result if the status is CREATED.
			var resp events.APIGatewayProxyResponse
			_ = json.Unmarshal([]byte(t.Result), &resp)
			_ = t.Delete()

			return resp, nil
		}
	}

	request.Headers["Location"] = "/" + stageName + request.Path + "?requestID=" + t.RequestID
	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusSeeOther,
		Headers:    request.Headers,
		Body:       request.Body,
	}, nil
}

func main() {
	_ = godotenv.Load()

	sess := session.Must(session.NewSession())

	client = serviceLambda.New(sess)
	db = dynamodb.New(sess)

	lambda.Start(Proxy)
}
