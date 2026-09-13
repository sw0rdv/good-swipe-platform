package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"sort"
	"time"

	"good-swipe-platform/services/user-service/internal"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"

	"github.com/redis/go-redis/v9"

	pb "good-swipe-platform/proto"

	"github.com/google/uuid"
	"google.golang.org/grpc"
)

var dynamoClient = initDynamo()
var redisClient *redis.Client

func initDynamo() *dynamodb.Client {
	client, err := internal.NewDynamoClient()
	if err != nil {
		log.Fatalf("failed to connect dynamodb: %v", err)
	}

	err = internal.CreateUsersTableIfNotExists(client)
	if err != nil {
		log.Fatalf("failed to create users table: %v", err)
	}

	err = internal.CreateTournamentParticipantsTableIfNotExists(client)
	if err != nil {
		log.Fatalf("failed to create tournament participants table: %v", err)
	}

	return client
}

type server struct {
	pb.UnimplementedUserServiceServer
}

func getTodayTournamentID() string { ///HELPER!!!! METHOD
	return fmt.Sprintf(
		"daily-tournament-%s",
		time.Now().UTC().Format("2006-01-02"),
	)
}

func globalLeaderboardKey() string { //HELPER METHOD REDIS
	return "leaderboard:global"
}

func tournamentLeaderboardKey(tournamentID string, groupID string) string { //HELPER METHOD REDIS
	return fmt.Sprintf(
		"leaderboard:%s:%s",
		tournamentID,
		groupID,
	)
}

func (s *server) CreateUser(
	ctx context.Context,
	req *pb.CreateUserRequest,
) (*pb.CreateUserResponse, error) {

	userID := uuid.New().String()

	user := &internal.User{
		UserID:      userID,
		DisplayName: req.DisplayName,
		Level:       1,
		Coins:       1000,
	}

	fmt.Println("SaveUser çağrılacak")
	err := internal.SaveUser(dynamoClient, user)
	if err != nil {
		return nil, fmt.Errorf("failed to save user: %v", err)
	}
	fmt.Println("SaveUser çağrıldı")
	fmt.Println("User created:", user.DisplayName)

	return &pb.CreateUserResponse{
		UserId: user.UserID,
		Level:  user.Level,
		Coins:  user.Coins,
	}, nil
}

func (s *server) UpdateProgress(
	ctx context.Context,
	req *pb.UpdateProgressRequest,
) (*pb.UpdateProgressResponse, error) {

	user, err := internal.GetUserByID(dynamoClient, req.UserId)
	if err != nil {
		return nil, fmt.Errorf("user not found")
	}

	user.Level += req.ProgressAmount
	user.Coins += req.ProgressAmount * 100

	tournamentID := getTodayTournamentID()

	participant, err := internal.GetTournamentParticipant(
		dynamoClient,
		user.UserID,
		tournamentID,
	)
	if err == nil {
		participant.Score += req.ProgressAmount

		err = internal.UpdateTournamentParticipant(dynamoClient, participant) //Turnuvaya katılan user güncellenmedi.!
		if err == nil {
			leaderboardKey := tournamentLeaderboardKey(
				participant.TournamentID,
				participant.GroupID,
			)

			err = redisClient.ZAdd(
				context.TODO(),
				leaderboardKey,
				redis.Z{
					Score:  float64(participant.Score),
					Member: participant.UserID,
				},
			).Err()

			fmt.Println(
				"Redis tournament leaderboard updated:",
				participant.UserID,
				"Score:",
				participant.Score,
			)

			if err != nil {
				return nil, fmt.Errorf("failed to update tournament redis leaderboard")
			}
		}

		if err != nil {
			return nil, fmt.Errorf("failed to update tournament score") //Manuel check gerekiyor.
		}
	}

	err = internal.UpdateUser(dynamoClient, user)
	if err != nil {
		return nil, fmt.Errorf("failed to update user: %v", err)
	}

	err = redisClient.ZAdd(
		context.TODO(),
		globalLeaderboardKey(),
		redis.Z{
			Score:  float64(user.Level),
			Member: user.UserID,
		},
	).Err()

	fmt.Println(
		"Redis global leaderboard updated:",
		user.DisplayName,
		"Level:",
		user.Level,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to update global redis leaderboard")
	}

	fmt.Println("User progress updated:", user.DisplayName)

	return &pb.UpdateProgressResponse{
		UserId: user.UserID,
		Level:  user.Level,
		Coins:  user.Coins,
	}, nil
}

func (s *server) EnterTournament(
	ctx context.Context,
	req *pb.EnterTournamentRequest,
) (*pb.EnterTournamentResponse, error) {

	user, err := internal.GetUserByID(dynamoClient, req.UserId)
	if err != nil {
		return nil, fmt.Errorf("user not found")
	}
	tournamentID := getTodayTournamentID()

	existingParticipant, err := internal.GetTournamentParticipant(
		dynamoClient,
		user.UserID,
		tournamentID,
	)

	if err == nil {
		fmt.Println("User already entered tournament:", user.DisplayName)

		return &pb.EnterTournamentResponse{
			TournamentId:   existingParticipant.TournamentID,
			RemainingCoins: user.Coins,
		}, nil
	}

	if user.Level < 10 {
		return nil, fmt.Errorf("user level must be at least 10 to enter tournament")
	}

	if user.Coins < 500 {
		return nil, fmt.Errorf("not enough coins to enter tournament")
	}

	now := time.Now().UTC()
	cutoff := time.Date(
		now.Year(),
		now.Month(),
		now.Day(),
		12, 0, 0, 0,
		time.UTC,
	)

	if now.After(cutoff) {
		return nil, fmt.Errorf("tournament entry is closed after 12:00 UTC")
	}

	user.Coins -= 500
	levelBucket := (user.Level / 10) * 10

	groupID, err := internal.FindAvailableTournamentGroup(
		dynamoClient,
		tournamentID,
		levelBucket,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to assign tournament group")
	}

	participant := &internal.TournamentParticipant{
		TournamentID: tournamentID,
		UserID:       user.UserID,
		GroupID:      groupID,
		Score:        0,
		Claimed:      false,
	}

	err = internal.SaveTournamentParticipant(
		dynamoClient,
		participant,
	)

	if err != nil {
		return nil, fmt.Errorf(
			"failed to save tournament participant",
		)
	}

	err = internal.UpdateUser(dynamoClient, user)
	if err != nil {
		return nil, fmt.Errorf("failed to update user-Kullanıcı güncellenemedi! Manuel check required!")
	}

	fmt.Println("User entered tournament:", user.DisplayName)

	return &pb.EnterTournamentResponse{
		TournamentId:   tournamentID,
		RemainingCoins: user.Coins,
	}, nil

}

func (s *server) ClaimReward(
	ctx context.Context,
	req *pb.ClaimRewardRequest,
) (*pb.ClaimRewardResponse, error) {

	user, err := internal.GetUserByID(dynamoClient, req.UserId)
	if err != nil {
		return nil, fmt.Errorf("user not found")
	}

	tournamentID := getTodayTournamentID()

	participant, err := internal.GetTournamentParticipant(
		dynamoClient,
		user.UserID,
		tournamentID,
	)
	if err != nil {
		return nil, fmt.Errorf("user did not participate in tournament")
	}

	if participant.Claimed {
		return &pb.ClaimRewardResponse{
			UserId:      user.UserID,
			RewardCoins: 0,
			TotalCoins:  user.Coins,
			Message:     "Reward already claimed", //başarısız dönecek altta var zaten if response
		}, nil
	}

	participants, err := internal.GetParticipantsByGroupID(
		dynamoClient,
		participant.GroupID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get tournament participants")
	}

	sort.Slice(participants, func(i, j int) bool {
		return participants[i].Score > participants[j].Score
	})

	rank := 0
	for index, p := range participants {
		if p.UserID == user.UserID {
			rank = index + 1
			break
		}
	}

	if rank == 0 {
		return nil, fmt.Errorf("rank could not be calculated")
	}

	reward := calculateReward(rank)

	user.Coins += reward
	participant.Claimed = true

	err = internal.UpdateUser(dynamoClient, user)
	if err != nil {
		return nil, fmt.Errorf("failed to update user reward")
	}

	err = internal.UpdateTournamentParticipant(dynamoClient, participant)
	if err != nil {
		return nil, fmt.Errorf("failed to update participant claim status")
	}

	fmt.Println("Reward claimed:", user.DisplayName, "Rank:", rank, "Reward:", reward)

	return &pb.ClaimRewardResponse{
		UserId:      user.UserID,
		RewardCoins: reward,
		TotalCoins:  user.Coins,
		Message:     "Reward claimed successfully", //başarılı dönecek
	}, nil

}

func calculateReward(rank int) int32 { //Verilecek ödüller sıralamya göre
	if rank == 1 {
		return 5000
	}
	if rank == 2 {
		return 3000
	}
	if rank == 3 {
		return 2000
	}
	if rank >= 4 && rank <= 10 {
		return 1000
	}
	return 0
}

func (s *server) GetTournamentLeaderboard(
	ctx context.Context,
	req *pb.GetTournamentLeaderboardRequest,
) (*pb.GetTournamentLeaderboardResponse, error) {

	tournamentID := getTodayTournamentID()

	participant, err := internal.GetTournamentParticipant(
		dynamoClient,
		req.UserId,
		tournamentID,
	)
	if err != nil {
		return nil, fmt.Errorf("user did not participate in tournament")
	}

	leaderboardKey := tournamentLeaderboardKey(
		participant.TournamentID,
		participant.GroupID,
	)

	// OLD DYNAMODB + MEMORY SORT APPROACH
	/*
		participants, err := internal.GetParticipantsByGroupID(
			dynamoClient,
			participant.GroupID,
		)

		if err != nil {
			return nil, fmt.Errorf("failed to get tournament leaderboard")
		}

		sort.Slice(participants, func(i, j int) bool {
			return participants[i].Score > participants[j].Score
		})
	*/

	// NEW REDIS SORTED SET APPROACH
	results, err := redisClient.ZRevRangeWithScores(
		context.TODO(),
		leaderboardKey,
		0,
		34,
	).Result()

	if err != nil {
		return nil, fmt.Errorf("failed to get tournament leaderboard from redis")
	}

	var entries []*pb.LeaderboardEntry

	for _, result := range results {
		userID := result.Member.(string)

		user, err := internal.GetUserByID(dynamoClient, userID)
		if err != nil {
			continue
		}

		entries = append(entries, &pb.LeaderboardEntry{
			UserId:      user.UserID,
			DisplayName: user.DisplayName,
			Score:       int32(result.Score),
		})
	}

	return &pb.GetTournamentLeaderboardResponse{
		Entries: entries,
	}, nil
}

func (s *server) GetTournamentRank(
	ctx context.Context,
	req *pb.GetTournamentRankRequest,
) (*pb.GetTournamentRankResponse, error) {

	tournamentID := getTodayTournamentID()

	participant, err := internal.GetTournamentParticipant(
		dynamoClient,
		req.UserId,
		tournamentID,
	)
	if err != nil {
		return nil, fmt.Errorf("user did not participate in tournament")
	}

	leaderboardKey := tournamentLeaderboardKey(
		participant.TournamentID,
		participant.GroupID,
	)

	// OLD DYNAMODB + MEMORY SORT APPROACH
	/*
		participants, err := internal.GetParticipantsByGroupID(
			dynamoClient,
			participant.GroupID,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to get tournament participants")
		}

		sort.Slice(participants, func(i, j int) bool {
			return participants[i].Score > participants[j].Score
		})

		rank := 0
		for index, p := range participants {
			if p.UserID == req.UserId {
				rank = index + 1
				break
			}
		}

		if rank == 0 {
			return nil, fmt.Errorf("rank could not be calculated")
		}
	*/

	// NEW REDIS SORTED SET APPROACH
	rank, err := redisClient.ZRevRank(
		context.TODO(),
		leaderboardKey,
		req.UserId,
	).Result()

	if err != nil {
		return nil, fmt.Errorf("rank could not be calculated from redis")
	}

	score, err := redisClient.ZScore(
		context.TODO(),
		leaderboardKey,
		req.UserId,
	).Result()

	if err != nil {
		return nil, fmt.Errorf("score could not be calculated from redis")
	}

	return &pb.GetTournamentRankResponse{
		UserId: req.UserId,
		Rank:   int32(rank + 1),
		Score:  int32(score),
	}, nil
}

func (s *server) GetGlobalLeaderboard(
	ctx context.Context,
	req *pb.GetGlobalLeaderboardRequest,
) (*pb.GetGlobalLeaderboardResponse, error) {

	results, err := redisClient.ZRevRangeWithScores(
		context.TODO(),
		globalLeaderboardKey(),
		0,
		999, ///Top 1000 kişi getirecek !
	).Result()

	if err != nil {
		return nil, fmt.Errorf("failed to get global leaderboard from redis")
	}

	var entries []*pb.LeaderboardEntry

	for _, result := range results {
		userID := result.Member.(string)

		user, err := internal.GetUserByID(dynamoClient, userID)
		if err != nil {
			continue
		}

		entries = append(entries, &pb.LeaderboardEntry{
			UserId:      user.UserID,
			DisplayName: user.DisplayName,
			Score:       int32(result.Score),
		})
	}

	return &pb.GetGlobalLeaderboardResponse{
		Entries: entries,
	}, nil
}

func (s *server) GetUserTournaments(
	ctx context.Context,
	req *pb.GetUserTournamentsRequest,
) (*pb.GetUserTournamentsResponse, error) {

	participants, err := internal.GetTournamentParticipantsByUserID(
		dynamoClient,
		req.UserId,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get user tournaments")
	}

	var tournaments []*pb.UserTournamentEntry

	for _, participant := range participants {
		groupParticipants, err := internal.GetParticipantsByGroupID(
			dynamoClient,
			participant.GroupID,
		)
		if err != nil {
			continue
		}

		sort.Slice(groupParticipants, func(i, j int) bool {
			return groupParticipants[i].Score > groupParticipants[j].Score
		})

		rank := 0
		for index, p := range groupParticipants {
			if p.UserID == req.UserId && p.TournamentID == participant.TournamentID {
				rank = index + 1
				break
			}
		}

		tournaments = append(tournaments, &pb.UserTournamentEntry{
			TournamentId: participant.TournamentID,
			GroupId:      participant.GroupID,
			Score:        participant.Score,
			Rank:         int32(rank),
			Claimed:      participant.Claimed,
		})
	}

	return &pb.GetUserTournamentsResponse{
		Tournaments: tournaments,
	}, nil
}

func main() {

	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}

	redisClient = redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})

	_, err := redisClient.Ping(context.TODO()).Result()
	if err != nil {
		log.Fatalf("failed to connect redis: %v", err)
	}

	fmt.Println("Connected to Redis")

	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer()

	pb.RegisterUserServiceServer(
		grpcServer,
		&server{},
	)

	fmt.Println("User Service running on port 50051")

	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
