package api

import (
	"binchecker/credential"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sns"
)

func SendNotification(message string) {
	// AWS_ACCESS_KEY_ID := os.Getenv("AWS_ACCESS_KEY_ID")
	// AWS_SECRET_ACCESS_KEY := os.Getenv("AWS_SECRET_ACCESS_KEY")
	// AWS_REGION := os.Getenv("AWS_REGION")
	sess := session.Must(session.NewSession(&aws.Config{
		Region: aws.String(credential.AWS_REGION),
		Credentials: credentials.NewStaticCredentials(
			credential.AWS_ACCESS_KEY_ID,
			credential.AWS_SECRET_ACCESS_KEY,
			""),
	}))

	svc := sns.New(sess)
	params := &sns.PublishInput{
		Message:     aws.String(message),
		PhoneNumber: aws.String("+919999999999"),
		MessageAttributes: map[string]*sns.MessageAttributeValue{
			"AWS.SNS.SMS.SMSType": {StringValue: aws.String("Transactional"), DataType: aws.String("String")},
		},
	}
	_, err := svc.Publish(params)
	if err != nil {
		fmt.Println(err.Error())
		return
	}

}
