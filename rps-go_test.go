package main

import (
	"github.com/mediocregopher/radix.v2/pool"
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/http/httptest"
	"testing"
)

func beforeTest() chan Competitor {
	statsPool, _ = pool.New("tcp", "localhost:6379", 10)
	conn, _ := statsPool.Get()
	defer statsPool.Put(conn)

	conn.Cmd("DEL", "user-tester1")
	conn.Cmd("DEL", "user-tester2")
	conn.Cmd("ZREM", "rps-wins", "tester1", "tester2")
	conn.Cmd("ZREM", "rps-losses", "tester1", "tester2")
	conn.Cmd("ZREM", "rps-draws", "tester1", "tester2")
	compChan := make(chan Competitor)
	go matchmaker(compChan)

	return compChan
}

func TestPlayerTwoWins(t *testing.T) {
	compChan := beforeTest()
	conn, _ := statsPool.Get()
	defer statsPool.Put(conn)

	rpsHandle1 := rpsHandler(Rock, compChan)
	rpsHandle2 := rpsHandler(Paper, compChan)

	req1, _ := http.NewRequest("GET", "/rock?iam=tester1", nil)
	req2, _ := http.NewRequest("GET", "/paper?iam=tester2", nil)
	w1 := httptest.NewRecorder()
	w2 := httptest.NewRecorder()
	go rpsHandle1.ServeHTTP(w1, req1)
	rpsHandle2.ServeHTTP(w2, req2)

	r := conn.Cmd("HMGET", "user-tester2", "wins", "losses")
	elems, _ := r.Array()
	wins, _ := elems[0].Int()
	losses, _ := elems[1].Int()

	assert.Equal(t, wins, 1)
	assert.Equal(t, losses, 0)
	assert.Equal(t, w2.Body.String(), "Congrats, tester2, you won!")

	r = conn.Cmd("HMGET", "user-tester1", "wins", "losses")
	elems, _ = r.Array()
	wins, _ = elems[0].Int()
	losses, _ = elems[1].Int()

	assert.Equal(t, wins, 0)
	assert.Equal(t, losses, 1)
}

func TestPlayerOneWins(t *testing.T) {
	compChan := beforeTest()
	conn, _ := statsPool.Get()
	defer statsPool.Put(conn)

	rpsHandle1 := rpsHandler(Rock, compChan)
	rpsHandle2 := rpsHandler(Scissors, compChan)

	req1, _ := http.NewRequest("GET", "/rock?iam=tester1", nil)
	req2, _ := http.NewRequest("GET", "/scissors?iam=tester2", nil)
	w1 := httptest.NewRecorder()
	w2 := httptest.NewRecorder()
	go rpsHandle1.ServeHTTP(w1, req1)
	rpsHandle2.ServeHTTP(w2, req2)

	r := conn.Cmd("HMGET", "user-tester2", "wins", "losses")
	elems, _ := r.Array()
	wins, _ := elems[0].Int()
	losses, _ := elems[1].Int()

	assert.Equal(t, wins, 0)
	assert.Equal(t, losses, 1)
	assert.Equal(t, w2.Body.String(), "Sorry, tester2, you lost!")

	r = conn.Cmd("HMGET", "user-tester1", "wins", "losses")
	elems, _ = r.Array()
	wins, _ = elems[0].Int()
	losses, _ = elems[1].Int()

	assert.Equal(t, wins, 1)
	assert.Equal(t, losses, 0)
}

func TestDraw(t *testing.T) {
	compChan := beforeTest()
	conn, _ := statsPool.Get()
	defer statsPool.Put(conn)

	rpsHandle1 := rpsHandler(Paper, compChan)
	rpsHandle2 := rpsHandler(Paper, compChan)

	req1, _ := http.NewRequest("GET", "/paper?iam=tester1", nil)
	req2, _ := http.NewRequest("GET", "/paper?iam=tester2", nil)
	w1 := httptest.NewRecorder()
	w2 := httptest.NewRecorder()
	go rpsHandle1.ServeHTTP(w1, req1)
	rpsHandle2.ServeHTTP(w2, req2)

	r := conn.Cmd("HMGET", "user-tester2", "wins", "losses")
	elems, _ := r.Array()
	wins, _ := elems[0].Int()
	losses, _ := elems[1].Int()

	assert.Equal(t, wins, 0)
	assert.Equal(t, losses, 0)
	assert.Equal(t, w2.Body.String(), "Well, tester2, you didn't lose, but you didn't win either. It was a draw!")

	r = conn.Cmd("HMGET", "user-tester1", "wins", "losses")
	elems, _ = r.Array()
	wins, _ = elems[0].Int()
	losses, _ = elems[1].Int()

	assert.Equal(t, wins, 0)
	assert.Equal(t, losses, 0)
}
