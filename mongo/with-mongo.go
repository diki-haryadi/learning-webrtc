package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"

	"github.com/gorilla/websocket"
	"github.com/pion/ion-sfu/pkg/sfu"
	"github.com/pion/webrtc/v3"
	"github.com/spf13/viper"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type WSMessage struct {
	Type string `json:"type"`
	Data string `json:"data"`
}

type IceCandidateData struct {
	Candidates webrtc.ICECandidateInit `json:"candidates"`
	Target     int                     `json:"target"`
}

type SessionData struct {
	ID            string                     `bson:"_id"`
	Offer         *webrtc.SessionDescription `bson:"offer"`
	Answer        *webrtc.SessionDescription `bson:"answer"`
	ICECandidates []webrtc.ICECandidateInit  `bson:"ice_candidates"`
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

var conf sfu.Config
var mongoClient *mongo.Client

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

	// Initialize MongoDB connection
	mongoClient, err = mongo.Connect(context.TODO(), options.Client().ApplyURI("mongodb://localhost:27017"))
	if err != nil {
		log.Println("Failed to connect to MongoDB:", err)
		return false
	}

	return true
}

//func main() {
//	load()
//
//	conf.WebRTC.SDPSemantics = "unified-plan-with-fallback"
//
//	s := sfu.NewSFU(conf)
//	s.NewDatachannel(sfu.APIChannelLabel)
//
//	http.HandleFunc("/ws", NewWebsocketHandler(s))
//	log.Println("server listening in :7001")
//	http.ListenAndServe(":7001", nil)
//}

//func NewWebsocketHandler(s *sfu.SFU) func(w http.ResponseWriter, req *http.Request) {
//	return func(w http.ResponseWriter, req *http.Request) {
//		conn, err := upgrader.Upgrade(w, req, nil)
//		if err != nil {
//			log.Println(err)
//			return
//		}
//
//		defer func() {
//			if err := conn.Close(); err != nil {
//				log.Println("there is conn error: ", err)
//			}
//		}()
//
//		peer := sfu.NewPeer(s)
//		sessionID := uuid.NewString()
//		peer.Join("room-id", sessionID)
//
//		peer.OnOffer = func(sdp *webrtc.SessionDescription) {
//			// Store the offer in MongoDB
//			storeSessionData(sessionID, &SessionData{
//				ID:    sessionID,
//				Offer: sdp,
//			})
//
//			if err := conn.WriteJSON(sdp); err != nil {
//				log.Println(err)
//			}
//		}
//
//		peer.OnIceCandidate = func(ii *webrtc.ICECandidateInit, i int) {
//			// Store the ICE candidate in MongoDB
//			storeICECandidate(sessionID, ii)
//
//			if err := conn.WriteJSON(map[string]interface{}{
//				"candidate": ii,
//				"target":    i,
//			}); err != nil {
//				log.Println(err)
//			}
//		}
//
//		peer.OnAnswer = func(sdp *webrtc.SessionDescription) {
//			// Store the answer in MongoDB
//			storeSessionData(sessionID, &SessionData{
//				Answer: sdp,
//			})
//		}
//
//		for {
//			wsMessage := WSMessage{}
//			if err := conn.ReadJSON(&wsMessage); err != nil {
//				log.Println("err: ", err.Error())
//				return
//			}
//
//			switch wsMessage.Type {
//			case "offer":
//				var offer webrtc.SessionDescription
//				if err := json.Unmarshal([]byte(wsMessage.Data), &offer); err != nil {
//					log.Println("Failed to unmarshal offer:", err)
//					return
//				}
//				peer.SetRemoteDescription(&offer)
//
//				// Retrieve ICE candidates from MongoDB and add them to the peer connection
//				candidates, err := getICECandidates(sessionID)
//				if err != nil {
//					log.Println("Failed to retrieve ICE candidates:", err)
//					return
//				}
//				for _, candidate := range candidates {
//					peer.AddICECandidate(candidate)
//				}
//
//				answer, err := peer.CreateAnswer()
//				if err != nil {
//					log.Println("Failed to create answer:", err)
//					return
//				}
//				peer.SetLocalDescription(answer)
//
//				// Store the answer in MongoDB
//				storeSessionData(sessionID, &SessionData{
//					Answer: answer,
//				})
//
//				if err := conn.WriteJSON(answer); err != nil {
//					log.Println(err)
//				}
//
//			case "answer":
//				var answer webrtc.SessionDescription
//				if err := json.Unmarshal([]byte(wsMessage.Data), &answer); err != nil {
//					log.Println("Failed to unmarshal answer:", err)
//					return
//				}
//				peer.SetRemoteDescription(&answer)
//
//			case "trickle":
//				var iceCandidateData IceCandidateData
//				if err := json.Unmarshal([]byte(wsMessage.Data), &iceCandidateData); err != nil {
//					log.Println("Failed to unmarshal ICE candidate:", err)
//					return
//				}
//				peer.AddICECandidate(iceCandidateData.Candidates)
//
//				// Store the ICE candidate in MongoDB
//				storeICECandidate(sessionID, iceCandidateData.Candidates)
//			}
//		}
//	}
//}

func storeSessionData(sessionID string, data *SessionData) error {
	collection := mongoClient.Database("your_database_name").Collection("sessions")
	_, err := collection.UpdateOne(context.TODO(), bson.M{"_id": sessionID}, bson.M{"$set": data}, options.Update().SetUpsert(true))
	return err
}

func getSessionData(sessionID string) (*SessionData, error) {
	collection := mongoClient.Database("your_database_name").Collection("sessions")
	var sessionData SessionData
	err := collection.FindOne(context.TODO(), bson.M{"_id": sessionID}).Decode(&sessionData)
	if err != nil {
		return nil, err
	}
	return &sessionData, nil
}

func storeICECandidate(sessionID string, candidate webrtc.ICECandidateInit) error {
	collection := mongoClient.Database("your_database_name").Collection("sessions")
	_, err := collection.UpdateOne(context.TODO(), bson.M{"_id": sessionID}, bson.M{"$push": bson.M{"ice_candidates": candidate}})
	return err
}

func getICECandidates(sessionID string) ([]webrtc.ICECandidateInit, error) {
	collection := mongoClient.Database("your_database_name").Collection("sessions")
	var sessionData SessionData
	err := collection.FindOne(context.TODO(), bson.M{"_id": sessionID}).Decode(&sessionData)
	if err != nil {
		return nil, err
	}
	return sessionData.ICECandidates, nil
}

func NewRoom(roomID string) *sfu.Room {
	room := s.NewRoom(roomID)
	room.OnNewPeer = func(peer *sfu.Peer) {
		// Load session data from MongoDB
		sessionData, err := getSessionData(peer.ID())
		if err != nil {
			log.Println("Failed to load session data:", err)
			return
		}

		// Set the offer and answer for the peer
		if sessionData.Offer != nil {
			peer.SetRemoteDescription(sessionData.Offer)
		}
		if sessionData.Answer != nil {
			peer.SetLocalDescription(sessionData.Answer)
		}

		// Add the stored ICE candidates to the peer
		for _, candidate := range sessionData.ICECandidates {
			peer.AddICECandidate(candidate)
		}
	}
	return room
}

func main() {
	load()

	conf.WebRTC.SDPSemantics = "unified-plan-with-fallback"

	s := sfu.NewSFU(conf)
	s.NewDatachannel(sfu.APIChannelLabel)

	// Create a new room
	room := NewRoom("room-id")

	http.HandleFunc("/ws", NewWebsocketHandler(s, room))
	log.Println("server listening in :7001")
	http.ListenAndServe(":7001", nil)
}

func NewWebsocketHandler(s *sfu.SFU, room *sfu.Room) func(w http.ResponseWriter, req *http.Request) {
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
		peer.Join(room.ID(), peer.ID())

		peer.OnOffer = func(sdp *webrtc.SessionDescription) {
			// Store the offer in MongoDB
			storeSessionData(peer.ID(), &SessionData{
				ID:    peer.ID(),
				Offer: sdp,
			})

			if err := conn.WriteJSON(sdp); err != nil {
				log.Println(err)
			}
		}

		peer.OnIceCandidate = func(ii *webrtc.ICECandidateInit, i int) {
			// Store the ICE candidate in MongoDB
			storeICECandidate(peer.ID(), ii)

			if err := conn.WriteJSON(map[string]interface{}{
				"candidate": ii,
				"target":    i,
			}); err != nil {
				log.Println(err)
			}
		}

		peer.OnAnswer = func(sdp *webrtc.SessionDescription) {
			// Store the answer in MongoDB
			storeSessionData(peer.ID(), &SessionData{
				Answer: sdp,
			})
		}

		for {
			wsMessage := WSMessage{}
			if err := conn.ReadJSON(&wsMessage); err != nil {
				log.Println("err: ", err.Error())
				return
			}

			switch wsMessage.Type {
			case "offer":
				var offer webrtc.SessionDescription
				if err := json.Unmarshal([]byte(wsMessage.Data), &offer); err != nil {
					log.Println("Failed to unmarshal offer:", err)
					return
				}
				peer.SetRemoteDescription(&offer)

				// Retrieve ICE candidates from MongoDB and add them to the peer connection
				candidates, err := getICECandidates(peer.ID())
				if err != nil {
					log.Println("Failed to retrieve ICE candidates:", err)
					return
				}
				for _, candidate := range candidates {
					peer.AddICECandidate(candidate)
				}

				answer, err := peer.CreateAnswer()
				if err != nil {
					log.Println("Failed to create answer:", err)
					return
				}
				peer.SetLocalDescription(answer)

				// Store the answer in MongoDB
				storeSessionData(peer.ID(), &SessionData{
					Answer: answer,
				})

				if err := conn.WriteJSON(answer); err != nil {
					log.Println(err)
				}

			case "answer":
				var answer webrtc.SessionDescription
				if err := json.Unmarshal([]byte(wsMessage.Data), &answer); err != nil {
					log.Println("Failed to unmarshal answer:", err)
					return
				}
				peer.SetRemoteDescription(&answer)

			case "trickle":
				var iceCandidateData IceCandidateData
				if err := json.Unmarshal([]byte(wsMessage.Data), &iceCandidateData); err != nil {
					log.Println("Failed to unmarshal ICE candidate:", err)
					return
				}
				peer.AddICECandidate(iceCandidateData.Candidates)

				// Store the ICE candidate in MongoDB
				storeICECandidate(peer.ID(), iceCandidateData.Candidates)
			}
		}
	}
}
