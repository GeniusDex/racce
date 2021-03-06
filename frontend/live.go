package frontend

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/geniusdex/racce/accserver"
)

type livePage struct {
	Server *accserver.Server
}

func (f *frontend) liveHandler(w http.ResponseWriter, r *http.Request) {
	page := &livePage{
		Server: f.server,
	}

	f.executeTemplate(w, r, "live.html", page)
}

func (f *frontend) liveWebSocketHandler(w http.ResponseWriter, r *http.Request) {
	ws, err := newWebSocketMessageMerger(w, r)
	if err != nil {
		log.Panicf("Failed to create websocket: %v", err)
	}

	go f.sendLiveStateUpdates(ws)
}

func writeMessageToWebSocket(ws webSocketWriter, msgType string, data interface{}) error {
	jsonMsg, err := json.Marshal(map[string]interface{}{"type": msgType, "data": data})
	if err != nil {
		log.Printf("cannot marshal message as JSON: %v", err)
	}

	return ws.WriteTextMessage(jsonMsg)
}

func (f *frontend) sendLiveStateUpdates(ws webSocketWriter) {
	log.Printf("Sending live state updates on websocket connection %v", ws.Name())

	events := f.server.LiveState.NewEventChannels()
	defer func() {
		ws.Close()
		events.Flush()
	}()

	for {
		select {
		case state, ok := <-events.ServerState:
			if !ok {
				return
			}
			writeMessageToWebSocket(ws, "serverState", state)

		case nrClients, ok := <-events.NrClients:
			if !ok {
				return
			}
			writeMessageToWebSocket(ws, "nrClients", nrClients)

		case track, ok := <-events.Track:
			if !ok {
				return
			}
			writeMessageToWebSocket(ws, "track", track)

		case state, ok := <-events.SessionState:
			if !ok {
				return
			}
			writeMessageToWebSocket(ws, "sessionState", state)

		case carState, ok := <-events.CarState:
			if !ok {
				return
			}
			writeMessageToWebSocket(ws, "carState", carState)

		case carID, ok := <-events.CarPurged:
			if !ok {
				return
			}
			writeMessageToWebSocket(ws, "carPurged", carID)
		}
	}
}
