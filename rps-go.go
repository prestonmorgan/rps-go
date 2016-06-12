package main

import (
  "net/http"
  "fmt"
  "strconv"
  "github.com/levenlabs/go-llog"
  "github.com/mediocregopher/radix.v2/redis"
)

const (
  Rock = 0
  Paper = 1
  Scissors = 2

  Win = 0
  Lose = 1
  Draw = 2
)

var resultsChart = [][]int{
  []int{Draw, Lose, Win},
  []int{Win, Draw, Lose},
  []int{Lose, Win, Draw},
}

var weaponList = []string{"rock", "paper", "scissors"}

type Competitor struct {
  Username string
  Weapon int
  ResultChannel chan int
}

func (c *Competitor) String() string {
  return fmt.Sprintf("{Username: %v, Weapon: %v}", c.Username, weaponList[c.Weapon])
}

func compete(weapon1, weapon2 int) (int, int) {
  return resultsChart[weapon1][weapon2], resultsChart[weapon2][weapon1]
}

func matchmaker(competitorChannel chan Competitor) {
  client, err := redis.Dial("tcp", "localhost:6379")
  if err != nil {
    llog.Error("Error connecting to redis", llog.KV{"error": err})
  }

  for {
    competitor1 := <-competitorChannel
    competitor2 := <-competitorChannel

    result1, result2 := compete(competitor1.Weapon, competitor2.Weapon)

    r := client.Cmd("HMGET", "user-" + competitor1.Username, "wins", "losses")
    if r.Err != nil {
      llog.Error("Error fetching wins/losses from redis", llog.KV{"error": err})
    }
    arr, _ := r.Array()
    wins, _ := arr[0].Int()
    losses, _ := arr[1].Int()
    var stats1 = map[string]int{
      "wins": wins,
      "losses": losses,
    }

    r = client.Cmd("HMGET", "user-" + competitor2.Username, "wins", "losses")
    if r.Err != nil {
      llog.Error("Error fetching wins/losses from redis", llog.KV{"error": err})
    }
    arr, _ = r.Array()
    wins, _ = arr[0].Int()
    losses, _ = arr[1].Int()
    var stats2 = map[string]int{
      "wins": wins,
      "losses": losses,
    }

    r = client.Cmd("HMGET", "max-stats", "winner", "wins", "loser", "losses")
    if r.Err != nil {
      llog.Error("Error fetching max-stats from redis", llog.KV{"error": err})
    }
    arr, _ = r.Array()
    winner, _ := arr[0].Str()
    winsString, _ := arr[1].Str()
    loser, _ := arr[2].Str()
    lossesString, _ := arr[3].Str()
    var maxStats = map[string]string{
      "winner": winner,
      "wins": winsString,
      "loser": loser,
      "losses": lossesString,
    }
    statsChanged := false
    maxStatsChanged := false
    maxWins, _ := strconv.Atoi(maxStats["wins"])
    maxLosses, _ := strconv.Atoi(maxStats["losses"])
    var resultString string

    switch result1 {
    case Win:
      resultString = competitor1.Username
      stats1["wins"] += 1
      stats2["losses"] += 1
      statsChanged = true
      if stats1["wins"] > maxWins {
        maxStats["winner"] = competitor1.Username
        maxStats["wins"] = strconv.Itoa(stats1["wins"])
        maxStatsChanged = true
      }
      if stats2["losses"] > maxLosses {
        maxStats["loser"] = competitor2.Username
        maxStats["losses"] = strconv.Itoa(stats2["losses"])
        maxStatsChanged = true
      }
    case Lose:
      resultString = competitor2.Username
      stats1["losses"] += 1
      stats2["wins"] += 1
      statsChanged = true
      if stats2["wins"] > maxWins {
        maxStats["winner"] = competitor2.Username
        maxStats["wins"] = strconv.Itoa(stats2["wins"])
        maxStatsChanged = true
      }
      if stats1["losses"] > maxLosses {
        maxStats["loser"] = competitor1.Username
        maxStats["losses"] = strconv.Itoa(stats1["losses"])
        maxStatsChanged = true
      }
    default:
      resultString = ""
    }

    if statsChanged {
      client.Cmd("HMSET", "user-" + competitor1.Username, stats1)
      client.Cmd("HMSET", "user-" + competitor2.Username, stats2)
    }

    if maxStatsChanged {
      client.Cmd("HMSET", "max-stats", maxStats)
    }

    competitor1.ResultChannel <- result1
    competitor2.ResultChannel <- result2

    llog.Info("A grand battle has occured", llog.KV{
      "Competitor 1": competitor1.String(),
      "Competitor 2": competitor2.String(),
      "Winner": resultString,
      "Current High Scores": maxStats,
    })
  }
}

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

func main() {
  competitorChannel := make(chan Competitor)
  go matchmaker(competitorChannel)
  http.HandleFunc("/rock", rpsHandler(Rock, competitorChannel))
  http.HandleFunc("/paper", rpsHandler(Paper, competitorChannel))
  http.HandleFunc("/scissors", rpsHandler(Scissors, competitorChannel))
  http.ListenAndServe(":8080", nil)
}
