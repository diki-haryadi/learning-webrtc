package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/google/uuid"
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

	http.HandleFunc("/ws", NewWebsocketHandlerWithRedis(s))
	log.Println("server listening in :7001")
	http.ListenAndServe(":7001", nil)
}

func NewWebsocketHandler(s *sfu.SFU) func(w http.ResponseWriter, req *http.Request) {
	return func(w http.ResponseWriter, req *http.Request) {
		conn, err := upgrader.Upgrade(w, req, nil)
		//fmt.Println(conn)
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
		// 2 PC(peer connection) per client session, in order to solve the issues raised of using a single PC on large sessions.

		// First PC will be the Publisher, this PC will only be used to publish tracks from clients, so the SFU will only receive offers from this PC.
		// Second PC will be the Subscriber, this PC will be used to subscribe to remote peers, in these case the SFU will generate the offers and get only answers from client.

		// peer.Subscriber()
		// peer.Publisher()

		peer.Join("room-id", uuid.NewString())

		//always called initially
		peer.OnOffer = func(sdp *webrtc.SessionDescription) {
			if err := conn.WriteJSON(sdp); err != nil {
				log.Println(err)
			}
		}
		///target
		//0 ->publish 1->subscribe

		//called when offer is created
		peer.OnIceCandidate = func(ii *webrtc.ICECandidateInit, i int) {
			if err := conn.WriteJSON(map[string]interface{}{
				"candidate": ii,
				"target":    i,
			}); err != nil {
				log.Println(err)
			}
		}

		for {
			wsMessage := WSMessage{}
			if err := conn.ReadJSON(&wsMessage); err != nil {
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
				if err := conn.WriteJSON(answer); err != nil {
					log.Println(err)
				}
				fmt.Println("type => offer")
				fmt.Println(wsMessage)
				fmt.Println("->>offer end message")
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
				fmt.Println("type => answer")
				fmt.Println(wsMessage)
				fmt.Println("->>answer end message")
			case "trickle":
				candidates := IceCandidateData{}
				_ = json.Unmarshal([]byte(wsMessage.Data), &candidates)
				//candidates.Target -> to know if you want to publish or get
				err := peer.Trickle(candidates.Candidates, candidates.Target)
				if err != nil {
					log.Println(err)
				}
				fmt.Println("type => trickle")
				fmt.Println(wsMessage)
				fmt.Println("->>trickle end message")
			}

		}
	}
}

func NewWebsocketHandlerWithRedis(s *sfu.SFU) func(w http.ResponseWriter, req *http.Request) {
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

		// 2 PC(peer connection) per client session, in order to solve the issues raised of using a single PC on large sessions.

		// First PC will be the Publisher, this PC will only be used to publish tracks from clients, so the SFU will only receive offers from this PC.
		// Second PC will be the Subscriber, this PC will be used to subscribe to remote peers, in these case the SFU will generate the offers and get only answers from client.

		// peer.Subscriber()
		// peer.Publisher()
		roomID := uuid.NewString()
		peer.Join("room-id", roomID)
		fmt.Println("roomID=>>>")
		fmt.Println(roomID)
		//always called initially
		peer.OnOffer = func(sdp *webrtc.SessionDescription) {
			if err := conn.WriteJSON(sdp); err != nil {
				log.Println(err)
			}
		}
		///target
		//0 ->publish 1->subscribe

		//called when offer is created
		peer.OnIceCandidate = func(ii *webrtc.ICECandidateInit, i int) {
			if err := conn.WriteJSON(map[string]interface{}{
				"candidate": ii,
				"target":    i,
			}); err != nil {
				log.Println(err)
			}
		}

		for {
			wsMessage := WSMessage{}
			if err := conn.ReadJSON(&wsMessage); err != nil {
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
				if err := conn.WriteJSON(answer); err != nil {
					log.Println(err)
				}
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

type SessionData struct {
	ID            string                     `json:"id"`
	Offer         *webrtc.SessionDescription `json:"offer,omitempty"`
	Answer        *webrtc.SessionDescription `json:"answer,omitempty"`
	ICECandidates []webrtc.ICECandidateInit  `json:"ice_candidates"`
}

func storeSessionData(sessionID string, data *SessionData) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return redisClient.Set(ctx, "session:"+sessionID, jsonData, 0).Err()
}

func getSessionData(sessionID string) (*SessionData, error) {
	data, err := redisClient.Get(ctx, "session:"+sessionID).Bytes()
	if err != nil {
		return nil, err
	}
	var sessionData SessionData
	if err := json.Unmarshal(data, &sessionData); err != nil {
		return nil, err
	}
	return &sessionData, nil
}

func storeICECandidate(sessionID string, candidate webrtc.ICECandidateInit) error {
	sessionData, err := getSessionData(sessionID)
	if err != nil {
		return err
	}
	sessionData.ICECandidates = append(sessionData.ICECandidates, candidate)
	return storeSessionData(sessionID, sessionData)
}

func getICECandidates(sessionID string) ([]webrtc.ICECandidateInit, error) {
	sessionData, err := getSessionData(sessionID)
	if err != nil {
		return nil, err
	}
	return sessionData.ICECandidates, nil
}

// sfuInstance := sfu.NewSFU(sfu.Config{})

// localPeer := sfu.NewPeer(sfuInstance)
// when there is new offer on the peer
// localPeer.OnOffer

// when new ice candidates for found for the peer
// localPeer.OnIceCandidate

// join a user to a given session/room
// localPeer.Join("session-id", "user-id")

// generate answer for a offer
// localPeer.Answer(offer)

// if there is a answer
// localPeer.SetRemoteDescription(description)

// for adding ice candidates in publisher/subscriber instance
// localPeer.Trickle("candidate", --add ice in the publisher instance or subscriber instance--)

// ion-sfu contains two peer connections per session
// first will always be used to publish tracks from clients
// another will always be used to subscribe to remote peers
