package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"sync"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/pion/webrtc/v3"
	"github.com/thanhpk/randstr"
)

// type VoiceChannel struct {
// 	*sync.RWMutex
// 	Tracks map[string]webrtc.TrackLocal
// 	Users  map[string]*webrtc.PeerConnection
// }

// var channel = VoiceChannel{
// 	Tracks:  make(map[string]webrtc.TrackLocal),
// 	Users:   make(map[string]*webrtc.PeerConnection),
// 	RWMutex: &sync.RWMutex{},
// }

var outTrack webrtc.TrackLocal
var clients = make(map[string]*webrtc.PeerConnection)
var trackMut = sync.RWMutex{}

var peerConnectionConfig = webrtc.Configuration{
	ICEServers: []webrtc.ICEServer{
		{
			URLs: []string{"stun:stun.l.google.com:19302"},
		},
	},
}

func SDPHandler(c *gin.Context) {
	reqUserID := randstr.Hex(16)
	trackMut.Lock()
	defer trackMut.Unlock()

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

	peerConnection, err := webrtc.NewPeerConnection(peerConnectionConfig)
	if err != nil {
		panic(err)
	}
	clients[reqUserID] = peerConnection
	if outTrack == nil {
		// Allow us to receive 1 audio track
		_, err = peerConnection.AddTransceiverFromKind(webrtc.RTPCodecTypeAudio)
		if err != nil {
			panic(err)
		}

		peerConnection.OnTrack(func(remoteTrack *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
			localTrack, newTrackErr := webrtc.NewTrackLocalStaticRTP(remoteTrack.Codec().RTPCodecCapability, "audio", "pion")
			if newTrackErr != nil {
				panic(newTrackErr)
			}
			outTrack = localTrack

			trackMut.Lock()
			for _, conn := range clients {
				rtpSender, err := conn.AddTrack(outTrack)
				if err != nil {
					panic(err)
				}

				go func() {
					rtcpBuf := make([]byte, 1500)
					for {
						if _, _, rtcpErr := rtpSender.Read(rtcpBuf); rtcpErr != nil {
							return
						}
					}
				}()
			}
			trackMut.Unlock()

			rtpBuf := make([]byte, 1400)
			for {
				i, _, readErr := remoteTrack.Read(rtpBuf)
				if readErr != nil {
					panic(readErr)
				}

				// ErrClosedPipe means we don't have any subscribers, this is ok if no peers have connected yet
				if _, err = localTrack.Write(rtpBuf[:i]); err != nil && !errors.Is(err, io.ErrClosedPipe) {
					panic(err)
				}
			}
		})
	} else {
		rtpSender, err := peerConnection.AddTrack(outTrack)
		if err != nil {
			panic(err)
		}

		go func() {
			rtcpBuf := make([]byte, 1500)
			for {
				if _, _, rtcpErr := rtpSender.Read(rtcpBuf); rtcpErr != nil {
					return
				}
			}
		}()
	}

	err = peerConnection.SetRemoteDescription(offer)
	if err != nil {
		panic(err)
	}

	answer, err := peerConnection.CreateAnswer(nil)
	if err != nil {
		panic(err)
	}

	gatherComplete := webrtc.GatheringCompletePromise(peerConnection)

	err = peerConnection.SetLocalDescription(answer)
	if err != nil {
		panic(err)
	}

	<-gatherComplete

	c.JSON(http.StatusOK, peerConnection.LocalDescription())
	return
}

func main() {
	r := gin.Default()
	r.Use(cors.Default())
	r.POST("/sdp", SDPHandler)

	r.Run(":4000")
}
