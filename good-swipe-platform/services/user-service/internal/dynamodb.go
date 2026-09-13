package internal

import (
	"context"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

const UsersTableName = "Users"
const TournamentParticipantsTableName = "TournamentParticipants"

func NewDynamoClient() (*dynamodb.Client, error) {

	endpoint := os.Getenv("DYNAMODB_ENDPOINT")
	if endpoint == "" {
		endpoint = "http://localhost:8000"
	}

	customResolver := aws.EndpointResolverWithOptionsFunc(
		func(service, region string, options ...interface{}) (aws.Endpoint, error) {
			return aws.Endpoint{
				URL:           endpoint,
				SigningRegion: "us-east-1",
			}, nil
		},
	)

	cfg, err := config.LoadDefaultConfig(
		context.TODO(),
		config.WithRegion("us-east-1"),
		config.WithEndpointResolverWithOptions(customResolver),
		config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider("dummy", "dummy", ""),
		),
	)

	if err != nil {
		return nil, err
	}

	client := dynamodb.NewFromConfig(cfg)

	fmt.Println("Connected to DynamoDB - Sql başarılı")

	return client, nil
}

func CreateUsersTableIfNotExists(client *dynamodb.Client) error {

	_, err := client.DescribeTable(context.TODO(), &dynamodb.DescribeTableInput{
		TableName: aws.String(UsersTableName),
	})

	if err == nil {
		fmt.Println("Users table already exists- Kullanıcı tablosu zaten var")
		return nil
	}

	_, err = client.CreateTable(context.TODO(), &dynamodb.CreateTableInput{
		TableName: aws.String(UsersTableName),
		AttributeDefinitions: []types.AttributeDefinition{
			{
				AttributeName: aws.String("user_id"),
				AttributeType: types.ScalarAttributeTypeS,
			},
		},
		KeySchema: []types.KeySchemaElement{
			{
				AttributeName: aws.String("user_id"),
				KeyType:       types.KeyTypeHash,
			},
		},
		BillingMode: types.BillingModePayPerRequest,
	})

	if err != nil {
		return err
	}

	fmt.Println("Users table succesfully created - Ozgur")
	return nil
}

func SaveUser(client *dynamodb.Client, user *User) error {
	item, err := attributevalue.MarshalMap(user)
	if err != nil {
		return err
	}

	_, err = client.PutItem(context.TODO(), &dynamodb.PutItemInput{
		TableName: aws.String(UsersTableName),
		Item:      item,
	})

	if err != nil {
		return err
	}

	fmt.Println("KAYIT BAŞARILI - User saved to DynamoDB:", user.UserID)
	return nil
}

func GetUserByID(client *dynamodb.Client, userID string) (*User, error) {
	result, err := client.GetItem(context.TODO(), &dynamodb.GetItemInput{
		TableName: aws.String(UsersTableName),
		Key: map[string]types.AttributeValue{
			"user_id": &types.AttributeValueMemberS{Value: userID},
		},
	})

	if err != nil {
		return nil, err
	}

	if result.Item == nil {
		return nil, fmt.Errorf("user not found")
	}

	var user User
	err = attributevalue.UnmarshalMap(result.Item, &user)
	if err != nil {
		return nil, err
	}

	return &user, nil
}

func GetAllUsers(
	client *dynamodb.Client,
) ([]User, error) {

	result, err := client.Scan(
		context.TODO(),
		&dynamodb.ScanInput{
			TableName: aws.String(UsersTableName),
		},
	)

	if err != nil {
		return nil, err
	}

	var users []User

	err = attributevalue.UnmarshalListOfMaps(result.Items, &users)
	if err != nil {
		return nil, err
	}

	return users, nil
}

func UpdateUser(client *dynamodb.Client, user *User) error {
	return SaveUser(client, user)
}

func CreateTournamentParticipantsTableIfNotExists(
	client *dynamodb.Client,
) error {

	_, err := client.DescribeTable(
		context.TODO(),
		&dynamodb.DescribeTableInput{
			TableName: aws.String(TournamentParticipantsTableName),
		},
	)

	if err == nil {
		fmt.Println("TournamentParticipants table already exists")
		return nil
	}

	_, err = client.CreateTable(
		context.TODO(),
		&dynamodb.CreateTableInput{
			TableName: aws.String(TournamentParticipantsTableName),

			AttributeDefinitions: []types.AttributeDefinition{
				{
					AttributeName: aws.String("user_id"),
					AttributeType: types.ScalarAttributeTypeS, // S String N int B Binary
				},
				{
					AttributeName: aws.String("tournament_id"),
					AttributeType: types.ScalarAttributeTypeS,
				},
			},

			KeySchema: []types.KeySchemaElement{
				{
					AttributeName: aws.String("user_id"),
					KeyType:       types.KeyTypeHash, // LİKE PRİMARY KEY
				},
				{
					AttributeName: aws.String("tournament_id"),
					KeyType:       types.KeyTypeRange, // LİKE SORT KEY
				},
			},

			BillingMode: types.BillingModePayPerRequest, //PROD MANTIĞINDA KULLANIYORUZ FOR AWS
		},
	)

	if err != nil {
		return err
	}

	fmt.Println("TournamentParticipants table created")

	return nil
}

func SaveTournamentParticipant(
	client *dynamodb.Client,
	participant *TournamentParticipant,
) error {

	item, err := attributevalue.MarshalMap(participant)
	if err != nil {
		return err
	}

	_, err = client.PutItem(
		context.TODO(),
		&dynamodb.PutItemInput{
			TableName: aws.String(TournamentParticipantsTableName),
			Item:      item,
		},
	)

	if err != nil {
		return err
	}

	fmt.Println(
		"Tournament participant saved:",
		participant.UserID,
	)

	return nil
}

func GetTournamentParticipant(
	client *dynamodb.Client,
	userID string,
	tournamentID string,
) (*TournamentParticipant, error) {

	result, err := client.GetItem(
		context.TODO(),
		&dynamodb.GetItemInput{
			TableName: aws.String(TournamentParticipantsTableName),
			Key: map[string]types.AttributeValue{
				"user_id": &types.AttributeValueMemberS{
					Value: userID,
				},
				"tournament_id": &types.AttributeValueMemberS{
					Value: tournamentID,
				},
			},
		},
	)

	if err != nil {
		return nil, err
	}

	if result.Item == nil {
		return nil, fmt.Errorf("participant not found")
	}

	var participant TournamentParticipant

	err = attributevalue.UnmarshalMap(result.Item, &participant)
	if err != nil {
		return nil, err
	}

	return &participant, nil
}

func GetParticipantsByGroupID(
	client *dynamodb.Client,
	groupID string,
) ([]TournamentParticipant, error) {

	result, err := client.Scan(
		context.TODO(),
		&dynamodb.ScanInput{
			TableName: aws.String(TournamentParticipantsTableName),
		},
	)

	if err != nil {
		return nil, err
	}

	var participants []TournamentParticipant

	err = attributevalue.UnmarshalListOfMaps(result.Items, &participants)
	if err != nil {
		return nil, err
	}

	var sameGroup []TournamentParticipant

	for _, participant := range participants {
		if participant.GroupID == groupID {
			sameGroup = append(sameGroup, participant)
		}
	}

	return sameGroup, nil

}

func UpdateTournamentParticipant(
	client *dynamodb.Client,
	participant *TournamentParticipant,
) error {
	return SaveTournamentParticipant(client, participant)
}

func GetAllTournamentParticipants(
	client *dynamodb.Client,
) ([]TournamentParticipant, error) {

	result, err := client.Scan(
		context.TODO(),
		&dynamodb.ScanInput{
			TableName: aws.String(TournamentParticipantsTableName),
		},
	)

	if err != nil {
		return nil, err
	}

	var participants []TournamentParticipant

	err = attributevalue.UnmarshalListOfMaps(result.Items, &participants)
	if err != nil {
		return nil, err
	}

	return participants, nil
}

func FindAvailableTournamentGroup(
	client *dynamodb.Client,
	tournamentID string,
	levelBucket int32,
) (string, error) {

	for groupNumber := 1; groupNumber <= 1000; groupNumber++ {
		groupID := fmt.Sprintf(
			"%s-group-level-%d-%d",
			tournamentID,
			levelBucket,
			groupNumber,
		)

		participants, err := GetParticipantsByGroupID(client, groupID)
		if err != nil {
			return "Group içindeki clientlar çekilemedi", err
		}

		if len(participants) < 35 {
			return groupID, nil
		}
	}

	return "", fmt.Errorf("no available group found")
}
func GetTournamentParticipantsByUserID(
	client *dynamodb.Client,
	userID string,
) ([]TournamentParticipant, error) {

	result, err := client.Query(
		context.TODO(),
		&dynamodb.QueryInput{
			TableName:              aws.String(TournamentParticipantsTableName),
			KeyConditionExpression: aws.String("user_id = :user_id"),
			ExpressionAttributeValues: map[string]types.AttributeValue{
				":user_id": &types.AttributeValueMemberS{
					Value: userID,
				},
			},
		},
	)

	if err != nil {
		return nil, err
	}

	var participants []TournamentParticipant

	err = attributevalue.UnmarshalListOfMaps(result.Items, &participants)
	if err != nil {
		return nil, err
	}

	return participants, nil
}
