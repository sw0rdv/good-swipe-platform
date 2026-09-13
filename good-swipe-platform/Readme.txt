Good Swipe Platform

Start the system
run.bat for windwos
run.sh for mac/Linux

Manual Test
docker compose up --build

Requirments:
- Docker Desktop
- Go 1.26+ (optional for local development)

Proto File:
user.proto


Information:
- gRPC Server: localhost:50051
- Redis: localhost:6379
- DynamoDB Local: localhost:8000


gRPC Endpoints
--CreateUser
{
  "display_name": "Ali"
}

--UpdateProgress
{
  "user_id": "Created_userid",
  "progress_amount": 10 //Example 
}

--EnterTournament
{
  "user_id": "Created_userid" ////If you want the user to participate in the tournament, you can write userid
}

--ClaimReward
{
  "user_id": "USER_ID"
}

--GetGlobalLeaderboard
{}

--GetTournamentLeaderboard
{
  "user_id": "USER_ID" //The leaderboard of the tournament group in user .
}

--GetTournamentRank
{
  "user_id": "USER_ID"
}

--GetUserTournaments
{
  "user_id": "USER_ID"
}

Postman Workspace
Due to current Postman limitations, gRPC collections cannot be exported as JSON directly.
Public Postman Workspace:
https://www.postman.com/zvural02-4113677/goodjob/collection/6a0b877f03405d602f37cb17/good-swipe-platform-grpc?action=share&creator=54383000