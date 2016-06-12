package main

import (
	"github.com/mediocregopher/radix.v2/redis"
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCompeting(t *testing.T) {
	client, _ := redis.Dial("tcp", "localhost:6379")
	client.Cmd("DEL", "user-tester1")
	client.Cmd("DEL", "user-tester2")
	client.Cmd("DEL", "max-stats")
	compChan := make(chan Competitor)
	go matchmaker(compChan)
	rpsHandle1 := rpsHandler(Rock, compChan)
	rpsHandle2 := rpsHandler(Paper, compChan)
	rpsHandle3 := rpsHandler(Scissors, compChan)

	req1, _ := http.NewRequest("GET", "/rock?iam=tester1", nil)
	req2, _ := http.NewRequest("GET", "/paper?iam=tester2", nil)
	w1 := httptest.NewRecorder()
	w2 := httptest.NewRecorder()
	go rpsHandle1.ServeHTTP(w1, req1)
	rpsHandle2.ServeHTTP(w2, req2)

	r := client.Cmd("HMGET", "user-tester2", "wins", "losses")
	elems, _ := r.Array()
	wins, _ := elems[0].Int()
	losses, _ := elems[1].Int()

	assert.Equal(t, wins, 1)
	assert.Equal(t, losses, 0)
	assert.Equal(t, w2.Body.String(), "Congrats, tester2, you won!")

	req1, _ = http.NewRequest("GET", "/paper?iam=tester1", nil)
	req2, _ = http.NewRequest("GET", "/rock?iam=tester2", nil)
	w1 = httptest.NewRecorder()
	w2 = httptest.NewRecorder()
	go rpsHandle2.ServeHTTP(w1, req1)
	rpsHandle1.ServeHTTP(w2, req2)

	r = client.Cmd("HMGET", "user-tester2", "wins", "losses")
	elems, _ = r.Array()
	wins, _ = elems[0].Int()
	losses, _ = elems[1].Int()

	assert.Equal(t, wins, 1)
	assert.Equal(t, losses, 1)
	assert.Equal(t, w2.Body.String(), "Sorry, tester2, you lost!")

	req1, _ = http.NewRequest("GET", "/scissors?iam=tester1", nil)
	req2, _ = http.NewRequest("GET", "/scissors?iam=tester2", nil)
	w1 = httptest.NewRecorder()
	w2 = httptest.NewRecorder()
	go rpsHandle3.ServeHTTP(w1, req1)
	rpsHandle3.ServeHTTP(w2, req2)

	r = client.Cmd("HMGET", "user-tester2", "wins", "losses")
	elems, _ = r.Array()
	wins, _ = elems[0].Int()
	losses, _ = elems[1].Int()

	assert.Equal(t, wins, 1)
	assert.Equal(t, losses, 1)
	assert.Equal(t, w2.Body.String(), "Well, tester2, you didn't lose, but you didn't win either. It was a draw!")
}
