package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/pion/webrtc/v3"
	"github.com/thanhpk/randstr"
)

type VoiceChannel struct {
	Tracks map[string]webrtc.TrackLocal
	Users  map[string]*webrtc.PeerConnection
}

var channel = VoiceChannel{
	Tracks: make(map[string]webrtc.TrackLocal),
	Users:  make(map[string]*webrtc.PeerConnection),
}

var engine = &webrtc.MediaEngine{}
var mediaAPI *webrtc.API
var peerConnectionConfig = webrtc.Configuration{
	ICEServers: []webrtc.ICEServer{
		{
			URLs: []string{"stun:stun.l.google.com:19302"},
		},
	},
}

func SDPHandler(c *gin.Context) {
	reqUserID := randstr.Hex(16)

	body, err := ioutil.ReadAll(c.Request.Body)
	if err != nil {
		c.Status(http.StatusUnprocessableEntity)
		return
	}

	offer := webrtc.SessionDescription{}
	if err := json.Unmarshal(body, &offer); err != nil {
		fmt.Println("error parsing SDP", err)
		c.Status(http.StatusBadRequest)
		return
	}

	peerConnection, err := mediaAPI.NewPeerConnection(peerConnectionConfig)
	if err != nil {
		fmt.Println("error making peer connection", err)
		c.Status(http.StatusInternalServerError)
		return
	}
	// peerConnection.CreateDataChannel("application", &webrtc.DataChannelInit{})
	if _, err := peerConnection.AddTransceiverFromKind(webrtc.RTPCodecTypeAudio, webrtc.RtpTransceiverInit{
		Direction: webrtc.RTPTransceiverDirectionSendrecv,
	}); err != nil {
		fmt.Println("error adding transceiver", err)
		c.Status(http.StatusInternalServerError)
		return
	}
	peerConnection.OnTrack(OnTrackStart(peerConnection, reqUserID))
	peerConnection.OnICEConnectionStateChange(OnICEConnectionStateChange(peerConnection, reqUserID))

	if err := peerConnection.SetRemoteDescription(offer); err != nil {
		fmt.Println("error setting remote description", err)
		c.Status(http.StatusInternalServerError)
		return
	}

	answer, err := peerConnection.CreateAnswer(nil)
	if err != nil {
		fmt.Println("error making answer", err)
		c.Status(http.StatusInternalServerError)
		return
	}
	gatherComplete := webrtc.GatheringCompletePromise(peerConnection)
	if err := peerConnection.SetLocalDescription(answer); err != nil {
		fmt.Println("error setting local description", err)
		c.Status(http.StatusInternalServerError)
		return
	}

	for userID := range channel.Tracks {
		if _, err := peerConnection.AddTrack(channel.Tracks[userID]); err != nil {
			fmt.Println(err)
		}
		fmt.Println("add track!")
	}

	channel.Users[reqUserID] = peerConnection

	<-gatherComplete

	c.JSON(http.StatusOK, peerConnection.LocalDescription())
	return
}

func main() {
	engine.RegisterDefaultCodecs()
	mediaAPI = webrtc.NewAPI(webrtc.WithMediaEngine(engine))
	r := gin.Default()
	r.Use(cors.Default())
	r.POST("/sdp", SDPHandler)

	r.Run(":4000")
}
