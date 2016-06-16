package main

import (
	"fmt"
	"github.com/levenlabs/go-llog"
	"github.com/mediocregopher/lever"
	"github.com/mediocregopher/radix.v2/pool"
	"net/http"
)

const (
	Rock     = 0
	Paper    = 1
	Scissors = 2

	Win  = 0
	Lose = 1
	Draw = 2
)

// Given weapon1 and weapon2 are each one of the consts Rock, Paper, or Scissors, get the result
// for weapon1 of a game between those two weapons via resultChart[weapon1][weapon2]
var resultsChart = [][]int{
	[]int{Draw, Lose, Win},
	[]int{Win, Draw, Lose},
	[]int{Lose, Win, Draw},
}

// Slice for converting between the weapon consts and weapon strings
var weaponList = []string{"rock", "paper", "scissors"}
var statsPool *pool.Pool

// A Competitor holds the username and weapon chosen by a client, as well as a channel
// for informing the client of the result of their match
type Competitor struct {
	Username      string
	Weapon        int
	ResultChannel chan int
}

// Returns a string representation of the Competitor
func (c *Competitor) String() string {
	return fmt.Sprintf("{Username: %v, Weapon: %v}", c.Username, weaponList[c.Weapon])
}

// Given two weapons, return (result for the first weapon, result for the second weapon)
func compete(weapon1, weapon2 int) (int, int) {
	return resultsChart[weapon1][weapon2], resultsChart[weapon2][weapon1]
}

// Given two usernames, and the first username's result, update the leaderboard, and
// return the username of the winner. If the result was a draw, returns ""
func updateStats(username1, username2 string, result1 int) string {
	conn, err := statsPool.Get()
	if err != nil {
		llog.Error("Error getting connection from redis pool", llog.KV{"error": err})
	}

	var resultString string

	switch result1 {
	case Win:
		resultString = username1
		conn.Cmd("ZINCRBY", "rps-wins", 1, username1)
		conn.Cmd("ZINCRBY", "rps-losses", 1, username2)
		conn.Cmd("HINCRBY", "user-"+username1, "wins", 1)
		conn.Cmd("HINCRBY", "user-"+username2, "losses", 1)
	case Lose:
		resultString = username2
		conn.Cmd("ZINCRBY", "rps-losses", 1, username1)
		conn.Cmd("ZINCRBY", "rps-wins", 1, username2)
		conn.Cmd("HINCRBY", "user-"+username1, "losses", 1)
		conn.Cmd("HINCRBY", "user-"+username2, "wins", 1)
	default:
		resultString = ""
		conn.Cmd("ZINCRBY", "rps-draws", 1, username1)
		conn.Cmd("ZINCRBY", "rps-draws", 1, username2)
	}
	statsPool.Put(conn)
	return resultString
}

// Given a Competitor channel, matchmaker gets two Competitors from the channel, sends the
// Competitors' results to their respective ResultsChannel, and logs the match. This is repeated
// until the program is terminated.
func matchmaker(competitorChannel chan Competitor) {

	for {
		competitor1 := <-competitorChannel
		competitor2 := <-competitorChannel

		result1, result2 := compete(competitor1.Weapon, competitor2.Weapon)
		resultString := updateStats(competitor1.Username, competitor2.Username, result1)

		competitor1.ResultChannel <- result1
		competitor2.ResultChannel <- result2

		llog.Info("A grand battle has occured", llog.KV{
			"Competitor 1": competitor1.String(),
			"Competitor 2": competitor2.String(),
			"Winner":       resultString,
		})
	}
}

// Wrapper function for creating an endpoint for a weapon. If "iam" is defined in the query
// string, use its value as the Competitor's username. Otherwise use the client's ip.
// After a result is sent to the Competitor's ResultChannel, write an appropriate message
// to the client's ResponseWriter.
func rpsHandler(weapon int, competitorChannel chan Competitor) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		resultChannel := make(chan int)
		query := r.URL.Query()

		username := query.Get("iam")
		if username == "" {
			username = r.RemoteAddr
		}

		competitor := Competitor{username, weapon, resultChannel}
		competitorChannel <- competitor
		result := <-resultChannel
		switch result {
		case Win:
			fmt.Fprintf(w, "Congrats, %s, you won!", username)
		case Lose:
			fmt.Fprintf(w, "Sorry, %s, you lost!", username)
		default:
			fmt.Fprintf(w, "Well, %s, you didn't lose, but you didn't win either. It was a draw!", username)
		}
	}
}

// Initializes redis pool, spins up a matchmaker routine, and creates the endpoints for the three weapons.
// When starting rps-go, you can specify the port to listen on via the --port tag.
// (e.g. ./rps-go --port 3000)
// Default port is 8080
func main() {
	f := lever.New("rps-go", nil)
	f.Add(lever.Param{Name: "--port", Default: "8080"})
	f.Parse()
	var err error
	statsPool, err = pool.New("tcp", "localhost:6379", 10)
	if err != nil {
		llog.Error("Error getting redis pool", llog.KV{"error": err})
	}

	port, _ := f.ParamStr("--port")
	competitorChannel := make(chan Competitor)
	go matchmaker(competitorChannel)
	http.HandleFunc("/rock", rpsHandler(Rock, competitorChannel))
	http.HandleFunc("/paper", rpsHandler(Paper, competitorChannel))
	http.HandleFunc("/scissors", rpsHandler(Scissors, competitorChannel))
	http.ListenAndServe(":"+port, nil)
}
