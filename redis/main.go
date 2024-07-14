package main

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
	"github.com/pion/ion-sfu/pkg/sfu"
	"log"
	"net/http"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type WSMessage struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

func NewWebsocketHandlerWithRedis(s *sfu.SFU, redisClient *redis.Client) func(w http.ResponseWriter, req *http.Request) {
	return func(w http.ResponseWriter, req *http.Request) {
		conn, err := upgrader.Upgrade(w, req, nil)
		if err != nil {
			log.Println(err)
			return
		}

		defer func() {
			if err := conn.Close(); err != nil {
				log.Println("there is conn error: ", err)
			}
		}()

		peer := sfu.NewPeer(s)
		fmt.Println("new peer")

		// Check if the client is the host
		isHost := checkIfHost(req)

		// Join the room
		roomID := "room-id"
		peerID := uuid.NewString()
		peer.Join(roomID, peerID)

		// Restore any existing streams from the data store
		restoreStreams(peer, roomID, redisClient)

		// Handle WebSocket messages
		for {
			wsMessage := WSMessage{}
			if err := conn.ReadJSON(&wsMessage); err != nil {
				log.Println("err: ", err.Error())
				return
			}

			switch wsMessage.Type {
			case "offer":
				// Handle offer message
				handleOffer(peer, wsMessage.Data, conn)
			case "answer":
				// Handle answer message
				handleAnswer(peer, wsMessage.Data)
			case "trickle":
				// Handle trickle message
				handleTrickle(peer, wsMessage.Data)
			}

			// Broadcast stream updates to all clients in the room
			broadcastStreamUpdate(roomID, peer.GetStreams(), redisClient)
		}
	}
}

func checkIfHost(req *http.Request) bool {
	// Implement your own logic to check if the client is the host
	// (e.g., based on a query parameter, session cookie, or other identifier)
	return false
}

func restoreStreams(peer *sfu.Peer, roomID string, redisClient *redis.Client) {
	// Retrieve any existing streams from the data store for the given room
	streams := getStreamsFromDataStore(roomID, redisClient)
	for _, stream := range streams {
		peer.AddStream(stream)
	}
}

func broadcastStreamUpdate(roomID string, streams []sfu.Stream, redisClient *redis.Client) {
	// Encode the stream information and send it to all clients in the room
	streamData, err := json.Marshal(streams)
	if err != nil {
		log.Println("error encoding streams:", err)
		return
	}

	// Store the updated stream information in the data store
	storeStreamsInDataStore(roomID, streamData, redisClient)

	// Send the stream update message to all clients in the room
	sendMessageToRoom(roomID, WSMessage{
		Type: "stream-update",
		Data: string(streamData),
	}, redisClient)
}

// Implement the functions to interact with the Redis data store
func getStreamsFromDataStore(roomID string, redisClient *redis.Client) []sfu.Stream {
	// Retrieve the streams from the Redis data store for the given room
	streamsJSON, err := redisClient.Get(context.Background(), fmt.Sprintf("room:%s:streams", roomID)).Result()
	if err != nil {
		log.Println("error retrieving streams from Redis:", err)
		return []sfu.Stream{}
	}

	var streams []sfu.Stream
	err = json.Unmarshal([]byte(streamsJSON), &streams)
	if err != nil {
		log.Println("error unmarshaling streams from Redis:", err)
		return []sfu.Stream{}
	}

	return streams
}

func storeStreamsInDataStore(roomID string, streams []byte, redisClient *redis.Client) {
	// Store the stream information in the Redis data store for the given room
	err := redisClient.Set(context.Background(), fmt.Sprintf("room:%s:streams", roomID), streams, 0).Err()
	if err != nil {
		log.Println("error storing streams in Redis:", err)
	}
}

func sendMessageToRoom(roomID string, message WSMessage, redisClient *redis.Client) {
	// Encode the message and send it to all clients in the room using Redis Pub/Sub
	messageJSON, err := json.Marshal(message)
	if err != nil {
		log.Println("error encoding message for Redis Pub/Sub:", err)
		return
	}

	err = redisClient.Publish(context.Background(), fmt.Sprintf("room:%s", roomID), messageJSON).Err()
	if err != nil {
		log.Println("error publishing message to Redis Pub/Sub:", err)
	}
}

func main() {
	// Initialize the Redis client
	redisClient := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "", // no password set
		DB:       0,  // use default DB
	})

	// Initialize the WebRTC SFU
	sfu := sfu.NewSFU(nil)

	// Create the WebSocket handler with Redis integration
	http.HandleFunc("/ws", NewWebsocketHandlerWithRedis(sfu, redisClient))

	log.Println("Starting server on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
