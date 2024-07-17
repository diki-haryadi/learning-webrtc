package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/pion/ion-sfu/pkg/sfu"
	"github.com/pion/webrtc/v3"
	"github.com/redis/go-redis/v9"
	"github.com/spf13/viper"
)

type WSMessage struct {
	Type string `json:"type"`
	Data string `json:"data"`
}

type IceCandidateData struct {
	Candidates webrtc.ICECandidateInit `json:"candidates"`
	Target     int                     `json:"target"`
}

type ClientSession struct {
	conn     *websocket.Conn
	mutex    sync.Mutex
	peerConn *webrtc.PeerConnection
	offer    *webrtc.SessionDescription
	answer   *webrtc.SessionDescription
	iceCands []webrtc.ICECandidateInit
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

var conf sfu.Config
var redisClient *redis.Client
var ctx context.Context

func load() bool {
	viper.SetConfigFile("config.toml")
	viper.SetConfigType("toml")

	err := viper.ReadInConfig()
	if err != nil {
		log.Println(err, "config file read failed")
		return false
	}
	err = viper.GetViper().Unmarshal(&conf)
	if err != nil {
		log.Println(err, "sfu config file loaded failed")
		return false
	}
	// Initialize Redis client
	redisClient = redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "mypassword", // no password set
		DB:       0,            // use default DB
	})
	return true
}

func main() {
	load()

	conf.WebRTC.SDPSemantics = "unified-plan-with-fallback"

	s := sfu.NewSFU(conf)
	s.NewDatachannel(sfu.APIChannelLabel)

	// WebSocket server
	http.HandleFunc("/ws", NewWebsocketHandlerWithRedis(s))
	go func() {
		log.Println("WebSocket server listening on :7001")
		err := http.ListenAndServe(":7001", nil)
		if err != nil {
			log.Println("WebSocket server error:", err)
		}
	}()

	// SSE server
	//go broadcaster(done)
	http.HandleFunc("/sse", HandleSSE)
	http.HandleFunc("/log", logHTTPRequest)

	log.Println("SSE server listening on :7002")
	err := http.ListenAndServe(":7002", nil)
	if err != nil {
		log.Println("SSE server error:", err)
	}

}

func NewWebsocketHandlerWithRedis(s *sfu.SFU) func(w http.ResponseWriter, req *http.Request) {
	clientSessions := make(map[string]*ClientSession)
	var mutex sync.Mutex

	return func(w http.ResponseWriter, req *http.Request) {
		// Get the query parameters
		roomID := req.URL.Query().Get("room_id")
		if roomID == "" {
			roomID = "room_id"
		}

		conn, err := upgrader.Upgrade(w, req, nil)
		if err != nil {
			log.Println(err)
			return
		}

		//sessionID := uuid.NewString()
		clientSession := &ClientSession{
			conn: conn,
		}

		mutex.Lock()
		clientSessions[roomID] = clientSession
		mutex.Unlock()

		defer func() {
			mutex.Lock()
			delete(clientSessions, roomID)
			mutex.Unlock()

			if err := conn.Close(); err != nil {
				log.Println("there is conn error: ", err)
			}
		}()

		peer := sfu.NewPeer(s)
		err = peer.Join("room-id", "roomID")
		if err != nil {
			fmt.Println("error join session=>>", roomID)
			fmt.Println(err)
			return
		}
		fmt.Println("room-id has joined session=>>", roomID)
		//always called initially
		peer.OnOffer = func(sdp *webrtc.SessionDescription) {
			clientSession.mutex.Lock()
			defer clientSession.mutex.Unlock()

			if err := clientSession.conn.WriteJSON(sdp); err != nil {
				log.Println(err)
			}
		}

		//called when offer is created
		peer.OnIceCandidate = func(ii *webrtc.ICECandidateInit, i int) {
			clientSession.mutex.Lock()
			defer clientSession.mutex.Unlock()

			if err := clientSession.conn.WriteJSON(map[string]interface{}{
				"candidate": ii,
				"target":    i,
			}); err != nil {
				log.Println(err)
			}
		}

		for {
			wsMessage := WSMessage{}
			if err := clientSession.conn.ReadJSON(&wsMessage); err != nil {
				log.Println("err: ", err.Error())
				return
			}

			switch wsMessage.Type {
			case "offer":
				//1) set sent sdp as remote description
				//2) create answer(our sdp to sent to remote)
				answer, err := peer.Answer(webrtc.SessionDescription{
					Type: webrtc.SDPTypeOffer,
					SDP:  wsMessage.Data,
				})
				if err != nil {
					log.Println(err)
					break
				}
				clientSession.mutex.Lock()
				if err := clientSession.conn.WriteJSON(answer); err != nil {
					clientSession.mutex.Unlock()
					log.Println(err)
				}
				clientSession.mutex.Unlock()
			case "answer":
				//only 1)set sent sdp as remote description //nothing to send
				err = peer.SetRemoteDescription(webrtc.SessionDescription{
					Type: webrtc.SDPTypeAnswer,
					SDP:  wsMessage.Data,
				})
				if err != nil {
					log.Println(err)
					log.Println(err)
				}
			case "trickle":
				candidates := IceCandidateData{}
				_ = json.Unmarshal([]byte(wsMessage.Data), &candidates)
				//candidates.Target -> to know if you want to publish or get
				err := peer.Trickle(candidates.Candidates, candidates.Target)
				if err != nil {
					log.Println(err)
				}
			}
		}
	}
}

type SSEClient struct {
	Channel chan string
	Done    chan struct{}
}

var sseClients = make(map[*SSEClient]struct{})
var sseClientsMutex sync.Mutex

func HandleSSE(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported!", http.StatusInternalServerError)
		return
	}

	clientChan := make(chan string, 10) // Increase the buffer size
	clientDone := make(chan struct{})

	sseClientsMutex.Lock()
	sseClients[&SSEClient{
		Channel: clientChan,
		Done:    clientDone,
	}] = struct{}{}
	sseClientsMutex.Unlock()

	defer func() {
		sseClientsMutex.Lock()
		delete(sseClients, &SSEClient{
			Channel: clientChan,
			Done:    clientDone,
		})
		sseClientsMutex.Unlock()
		close(clientDone)
	}()

	for {
		data, ok := <-clientChan
		if !ok {
			return
		}

		fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()
	}
}

func logHTTPRequest(w http.ResponseWriter, r *http.Request) {
	buf := new(strings.Builder)
	if _, err := io.Copy(buf, r.Body); err != nil {
		fmt.Printf("Error: %v", err)
	}
	method := r.Method

	logMsg := fmt.Sprintf("Method: %v, Body: %v", method, buf.String())
	fmt.Println(logMsg)

	sseClientsMutex.Lock()
	for client := range sseClients {
		select {
		case client.Channel <- logMsg:
		default:
			// Channel is full, handle the case here
			fmt.Println("Client channel is full, dropping message")
		}
	}
	sseClientsMutex.Unlock()
}
