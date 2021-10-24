package trackclient

import (
	"fmt"

	"time"
)
import "sync"
import "sync/atomic"

import "encoding/json"

type ClientRequset struct {
	RequestStart time.Time  `json:"start"`
	RequestEnd   *time.Time `json:"end,omitempty"`
	RequestBody  string     `json:"body"`
	Complete     bool       `json:"complete"`
}

type Client struct {
	ClientID   string `json:"id"`
	Credential string `json:"key"`
	//
	RequestStartCount    int  `json:"started"`
	RequestCompleteCount int  `json:"completed"`
	RequestAbortCount    int  `json:"aborted"`
	Blocked              bool `json:"blocked"`
	//
	ClientCreation time.Time      `json:"creationTime"`
	RequestsTime   time.Duration  `json:"processingTime"`
	Processing     *ClientRequset `json:"processing,omitempty"`
}

var clientStorage map[string]*Client
var clientKey map[string][]*Client
var idCount int64
var clientLock sync.RWMutex

func newClient() string {
	meID := atomic.AddInt64(&idCount, 1)
	return fmt.Sprintf("%08d", meID)
}

func getClientList() []byte {
	clientLock.RLock()
	defer clientLock.RUnlock()

	v := make([]*Client, 0, len(clientStorage))
	for _, value := range clientStorage {
		v = append(v, value)
	}

	b, err := json.Marshal(v)

	if err != nil {
		return []byte("[]")
	}

	return b
}

func getClientState(id string) []byte {
	clientLock.RLock()
	defer clientLock.RUnlock()

	v, ok := clientStorage[id]
	if !ok {
		return []byte("{}")
	}

	b, err := json.Marshal(v)

	if err != nil {
		return []byte("{}")
	}

	return b
}

func registerClient(key string, force bool) string {
	clientLock.Lock()
	defer clientLock.Unlock()

	myID := newClient()
	newClient := &Client{
		ClientID:   myID,
		Credential: key,
		//
		RequestStartCount:    0,
		RequestCompleteCount: 0,
		RequestAbortCount:    0,
		Blocked:              false,
		//
		ClientCreation: time.Now(),
		RequestsTime:   0 * time.Nanosecond,
		Processing:     nil,
	}

	clientKey[key] = append(clientKey[key], newClient)
	clientStorage[myID] = newClient

	return myID
}

func registerReqest(client string, request string) bool {
	clientLock.Lock()
	defer clientLock.Unlock()
	Client, ok := clientStorage[client]

	if !ok {
		return false
	}

	if Client.Processing != nil && Client.Processing.RequestEnd == nil {
		return false
	}

	Client.Processing = &ClientRequset{
		RequestStart: time.Now(),
		RequestEnd:   nil,
		RequestBody:  request,
		Complete:     false,
	}

	return true
}

func completeReqest(client string, request string) bool {
	clientLock.Lock()
	defer clientLock.Unlock()
	Client, ok := clientStorage[client]

	if !ok {
		return false
	}

	if Client.Processing == nil || Client.Processing.RequestEnd != nil || Client.Processing.RequestBody != request {
		return false
	}

	now := time.Now()
	Client.Processing.RequestEnd = &now
	Client.Processing.Complete = true
	return true
}

func abortReqest(client string) bool {
	clientLock.Lock()
	defer clientLock.Unlock()
	Client, ok := clientStorage[client]

	if !ok {
		return false
	}

	if Client.Processing == nil || Client.Processing.RequestEnd != nil {
		return false
	}

	now := time.Now()
	Client.Processing.RequestEnd = &now
	Client.Processing.Complete = false
	return true
}
