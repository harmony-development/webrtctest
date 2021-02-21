package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"sync"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/pion/rtcp"
	"github.com/pion/webrtc/v3"
	"github.com/thanhpk/randstr"
)

type VoiceChannel struct {
	sync.RWMutex
	Tracks map[string]webrtc.TrackLocal
	Users  map[string]*webrtc.PeerConnection
}

var channel = VoiceChannel{
	Tracks:  make(map[string]webrtc.TrackLocal),
	Users:   make(map[string]*webrtc.PeerConnection),
	RWMutex: sync.RWMutex{},
}

var peerConnectionConfig = webrtc.Configuration{
	ICEServers: []webrtc.ICEServer{
		{
			URLs: []string{"stun:stun.l.google.com:19302"},
		},
	},
}

func readSessionDescription(body io.ReadCloser) (*webrtc.SessionDescription, error) {
	read, err := ioutil.ReadAll(body)
	if err != nil {
		return nil, err
	}

	offer := webrtc.SessionDescription{}
	if err := json.Unmarshal(read, &offer); err != nil {
		fmt.Println("error parsing SDP", err)
		return nil, err
	}
	return &offer, nil
}

func newPeer(userID string) (*webrtc.PeerConnection, error) {
	peerConnection, err := webrtc.NewPeerConnection(peerConnectionConfig)
	if err != nil {
		return nil, err
	}
	channel.Users[userID] = peerConnection
	if _, err := peerConnection.AddTransceiverFromKind(webrtc.RTPCodecTypeAudio, webrtc.RTPTransceiverInit{
		Direction: webrtc.RTPTransceiverDirectionSendrecv,
	}); err != nil {
		return nil, err
	}
	return peerConnection, nil
}

func SDPHandler(c *gin.Context) {
	// generate a new random ID for this track
	reqUserID := randstr.Hex(16)
	// no race conditions
	channel.Lock()
	defer channel.Unlock()

	// parses the SessionDescription from the request
	offer, err := readSessionDescription(c.Request.Body)
	if err != nil {
		c.Status(http.StatusUnprocessableEntity)
		return
	}

	// create a new peerConnection
	peerConnection, err := newPeer(reqUserID)
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}

	peerConnection.OnTrack(func(remoteTrack *webrtc.TrackRemote, r *webrtc.RTPReceiver) {
		// send a picture loss indication so a keyframe is pushed
		go func() {
			ticker := time.NewTicker(time.Second * 3)
			for range ticker.C {
				if rtcpSendErr := peerConnection.WriteRTCP([]rtcp.Packet{&rtcp.PictureLossIndication{MediaSSRC: uint32(remoteTrack.SSRC())}}); rtcpSendErr != nil {
					fmt.Println(rtcpSendErr)
				}
			}
		}()

		// make a local track that feeds into all clients
		localTrack, err := webrtc.NewTrackLocalStaticRTP(remoteTrack.Codec().RTPCodecCapability, remoteTrack.ID(), remoteTrack.StreamID())
		if err != nil {
			return
		}

		// add the track and add it to all existing clients
		channel.Lock()
		channel.Tracks[remoteTrack.ID()] = localTrack
		for _, track := range channel.Tracks {
			for _, conn := range channel.Users {
				if _, err := conn.AddTrack(track); err != nil {
					panic(err)
				}
			}
		}
		channel.Unlock()

		// read from the remote track and write to the local track
		buf := make([]byte, 1500)
		for {
			i, _, err := remoteTrack.Read(buf)
			if err != nil {
				return
			}

			if _, err = localTrack.Write(buf[:i]); err != nil {
				return
			}
		}
	})

	// add all existing tracks to the current connecton
	for _, track := range channel.Tracks {
		for _, conn := range channel.Users {
			if _, err := conn.AddTrack(track); err != nil {
				panic(err)
			}
		}
	}

	err = peerConnection.SetRemoteDescription(*offer)
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
