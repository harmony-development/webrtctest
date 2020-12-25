package main

import (
	"github.com/pion/webrtc/v3"
)

// OnTrackStart handles when a track is being received from a peer
func OnTrackStart(peerConnection *webrtc.PeerConnection) func(remoteTrack *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
	return func(remoteTrack *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {

	}
}

// OnICEConnectionStateChange handles webrtc state changes such as timeouts
func OnICEConnectionStateChange(peerConnection *webrtc.PeerConnection) func(webrtc.ICEConnectionState) {
	return func(state webrtc.ICEConnectionState) {
		if state == webrtc.ICEConnectionStateDisconnected || state == webrtc.ICEConnectionStateClosed {
		}
	}
}
