package main

import (
	"fmt"
	"io"

	"github.com/pion/webrtc/v3"
)

// OnTrackStart handles when a track is being received from a peer
func OnTrackStart(peerConnection *webrtc.PeerConnection, userID string) func(remoteTrack *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
	return func(remoteTrack *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
		localTrack, err := webrtc.NewTrackLocalStaticRTP(webrtc.RTPCodecCapability{MimeType: "audio/opus"}, "voice", remoteTrack.ID())
		if err != nil {
			fmt.Println(err)
			return
		}
		channel.Tracks[userID] = localTrack
		for userID := range channel.Tracks {
			if _, err := channel.Users[userID].AddTrack(localTrack); err != nil {
				fmt.Println(err)
			}
			fmt.Println("add track! (after track start)")
		}
		rtpBuf := make([]byte, 1460)
		for {
			i, _, readErr := remoteTrack.Read(rtpBuf)
			if readErr != nil {
				fmt.Println(readErr)
				return
			}
			if _, err = localTrack.Write(rtpBuf[:i]); err != nil && err != io.ErrClosedPipe {
				fmt.Println(err)
				return
			}
		}
	}
}

// OnICEConnectionStateChange handles webrtc state changes such as timeouts
func OnICEConnectionStateChange(peerConnection *webrtc.PeerConnection, userID string) func(webrtc.ICEConnectionState) {
	return func(state webrtc.ICEConnectionState) {
		if state == webrtc.ICEConnectionStateDisconnected || state == webrtc.ICEConnectionStateClosed {
			fmt.Println("disconnect", userID, state)
			if err := peerConnection.Close(); err != nil {
				fmt.Println(err)
			}
			delete(channel.Tracks, userID)
			delete(channel.Users, userID)
		}
	}
}
