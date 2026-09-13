package internal

type User struct {
	UserID      string `dynamodbav:"user_id"`
	DisplayName string `dynamodbav:"display_name"`
	Level       int32  `dynamodbav:"level"`
	Coins       int32  `dynamodbav:"coins"`
}

type TournamentParticipant struct {
	TournamentID string `dynamodbav:"tournament_id"`
	UserID       string `dynamodbav:"user_id"`
	GroupID      string `dynamodbav:"group_id"`
	Score        int32  `dynamodbav:"score"`
	Claimed      bool   `dynamodbav:"claimed"`
}
