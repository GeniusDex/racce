package accserver

import (
	"log"
	"sort"

	"github.com/geniusdex/racce/accdata"
)

// ServerState represents the current state of the server instance
type ServerState string

const (
	ServerStateOffline       ServerState = "offline"
	ServerStateStarting      ServerState = "starting"
	ServerStateNotRegistered ServerState = "not_registered"
	ServerStateOnline        ServerState = "online"
)

// Driver contains the information about a single driver
type Driver struct {
	ConnectionID int
	Name         string
	PlayerID     string
}

// CarState represents the current state of a single car
type CarState struct {
	CarID         int
	RaceNumber    int
	CarModel      *accdata.CarModel
	Drivers       []*Driver
	CurrentDriver *Driver
	Position      int
	BestLapMS     int
}

func newCarState() *CarState {
	return &CarState{
		Drivers: make([]*Driver, 0),
	}
}

// LiveStateEvents contains channels for all types of events sent
//
// All channels must always be fully read until they are closed, to avoid hanging the
// event generating goroutine. All channels will be closed at the same time, but there
// is no guarantee that there are no pending messages on any other channel when one is
// closed. There is helper method Flush() to empty all channels on shutdown.
type LiveStateEvents struct {
	ServerState chan ServerState
	NrClients   chan int
	Track       chan *accdata.Track
	CarState    chan *CarState
	CarPurged   chan int
}

// Flush reads all remaining events on all channels until they are closed.
//
// Never call this method before at least one of the event channels has been closed!
func (events *LiveStateEvents) Flush() {
	for range events.ServerState {
	}
	for range events.NrClients {
	}
	for range events.Track {
	}
	for range events.CarState {
	}
	for range events.CarPurged {
	}
}

// LiveState is the live state of the accServer
type LiveState struct {
	// ServerState is the current state of the server
	ServerState ServerState
	// NrClients is the number of clients currently connected to the server
	NrClients int
	// Track is the current track on the server; will never be nil
	Track *accdata.Track
	// CarState contains the current state for all cars, keyed on car ID
	CarState map[int]*CarState

	// eventListeners contains all active event listeners
	eventListeners []*LiveStateEvents
	// stopMonitoring is a channel used to indicate when the monitoring should stop
	stopMonitoring chan bool
	// connectionRequests contains the yet unhandled connection requests
	connectionRequests []*logEventNewConnectionRequest
	// driverPerConnection contains the driver associated with each connection
	driverPerConnection map[int]*Driver
	// carPerConnection maps connection IDs to the car ID they they are online for
	carPerConnection map[int]int
}

func newLiveState() *LiveState {
	return &LiveState{
		ServerState:         ServerStateOffline,
		NrClients:           0,
		Track:               accdata.Tracks[0],
		CarState:            make(map[int]*CarState),
		connectionRequests:  make([]*logEventNewConnectionRequest, 0),
		driverPerConnection: make(map[int]*Driver),
		carPerConnection:    make(map[int]int),
	}
}

// NewEventChannels creates new channels for the state events
func (ls *LiveState) NewEventChannels() *LiveStateEvents {
	events := &LiveStateEvents{
		ServerState: make(chan ServerState),
		NrClients:   make(chan int),
		Track:       make(chan *accdata.Track),
		CarState:    make(chan *CarState),
		CarPurged:   make(chan int),
	}

	ls.eventListeners = append(ls.eventListeners, events)

	return events
}

//--- Derived information ---//

// IsRunning indicates if the server is actually running (Online or NotRegistered)
func (ls *LiveState) IsRunning() bool {
	return ls.ServerState == ServerStateOnline || ls.ServerState == ServerStateNotRegistered
}

//--- State updates ---//

func (ls *LiveState) setServerState(value ServerState) {
	ls.ServerState = value
	for _, listener := range ls.eventListeners {
		listener.ServerState <- value
	}
}

func (ls *LiveState) setNrClients(value int) {
	ls.NrClients = value
	for _, listener := range ls.eventListeners {
		listener.NrClients <- value
	}
}

func (ls *LiveState) setTrack(track *accdata.Track) {
	ls.Track = track
	for _, listener := range ls.eventListeners {
		listener.Track <- track
	}
}

func (ls *LiveState) setCarState(carState *CarState) {
	ls.CarState[carState.CarID] = carState
	for _, listener := range ls.eventListeners {
		listener.CarState <- carState
	}
}

func (ls *LiveState) purgeCar(carID int) {
	delete(ls.CarState, carID)
	for _, listener := range ls.eventListeners {
		listener.CarPurged <- carID
	}
}

//--- Helper functions ---//

func (ls *LiveState) lookupDriverForNewCarConnection(carEvent logEventNewCarConnection) *Driver {
	for i, connEvent := range ls.connectionRequests {
		if connEvent.CarModelID == carEvent.CarModelID {
			lastIndex := len(ls.connectionRequests) - 1
			ls.connectionRequests[i] = ls.connectionRequests[lastIndex]
			ls.connectionRequests = ls.connectionRequests[:lastIndex]
			driver := &Driver{
				ConnectionID: connEvent.ConnectionID,
				Name:         connEvent.PlayerName,
				PlayerID:     connEvent.SteamID,
			}
			ls.driverPerConnection[driver.ConnectionID] = driver
			return driver
		}
	}
	return nil
}

func (ls *LiveState) recalculatePositions() {
	cars := make([]*CarState, 0, len(ls.CarState))
	for _, car := range ls.CarState {
		cars = append(cars, car)
	}

	sort.Slice(cars, func(i, j int) bool { return cars[i].Position < cars[j].Position })

	for i := 0; i < len(cars); i++ {
		if cars[i].Position != i+1 {
			cars[i].Position = i + 1
			ls.setCarState(cars[i])
		}
	}
}

//--- Event reading and handling ---//

func (ls *LiveState) newInstance(logEvents <-chan interface{}) {
	if ls.stopMonitoring != nil {
		ls.stopMonitoring <- true
	}
	ls.stopMonitoring = make(chan bool)

	go ls.monitorEvents(logEvents, ls.stopMonitoring)
}

func (ls *LiveState) monitorEvents(logEvents <-chan interface{}, stopMonitoring chan bool) {
	// stopMonitoring is passed in the arguments since the one in LiveState will change when a new instance
	// is started, and we can set it to nil to indicate that we are no longer the active instance

	ls.setServerState(ServerStateStarting)
	ls.setNrClients(0)

	for logEvents != nil || stopMonitoring != nil {
		select {
		case event, ok := <-logEvents:
			if !ok {
				if stopMonitoring != nil {
					ls.setServerState(ServerStateOffline)
				}
				logEvents = nil
			} else if stopMonitoring != nil {
				ls.handleLogEvent(event)
			}

		case <-stopMonitoring:
			stopMonitoring = nil
		}
	}
}

func (ls *LiveState) handleLogEvent(event interface{}) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Unable to handle log event (%v): %v", event, r)
		}
	}()

	if e, ok := event.(logEventServerStarting); ok {
		ls.handleServerStarting(e)
	} else if e, ok := event.(logEventLobbyConnectionSucceeded); ok {
		ls.handleLobbyConnectionSucceeded(e)
	} else if e, ok := event.(logEventLobbyConnectionFailed); ok {
		ls.handleLobbyConnectionFailed(e)
	} else if e, ok := event.(logEventNrClientsOnline); ok {
		ls.handleNrClientsOnline(e)
	} else if e, ok := event.(logEventTrack); ok {
		ls.handleTrack(e)
	} else if e, ok := event.(logEventNewConnectionRequest); ok {
		ls.handleNewConnectionRequest(e)
	} else if e, ok := event.(logEventNewCarConnection); ok {
		ls.handleNewCarConnection(e)
	} else if e, ok := event.(logEventDeadConnection); ok {
		ls.handleDeadConnection(e)
	} else if e, ok := event.(logEventCarPurged); ok {
		ls.handleCarPurged(e)
	} else if e, ok := event.(logEventNewLapTime); ok {
		ls.handleNewLapTime(e)
	}
}

func (ls *LiveState) handleServerStarting(event logEventServerStarting) {
	ls.setServerState(ServerStateNotRegistered)
}

func (ls *LiveState) handleLobbyConnectionSucceeded(event logEventLobbyConnectionSucceeded) {
	ls.setServerState(ServerStateOnline)
}

func (ls *LiveState) handleLobbyConnectionFailed(event logEventLobbyConnectionFailed) {
	ls.setServerState(ServerStateNotRegistered)
}

func (ls *LiveState) handleNrClientsOnline(event logEventNrClientsOnline) {
	ls.setNrClients(event.NrClients)
}

func (ls *LiveState) handleTrack(event logEventTrack) {
	track := accdata.TrackByLabel(event.Track)
	if track != nil {
		ls.setTrack(track)
	}
}

func (ls *LiveState) handleNewConnectionRequest(event logEventNewConnectionRequest) {
	ls.connectionRequests = append(ls.connectionRequests, &event)
}

func (ls *LiveState) handleNewCarConnection(event logEventNewCarConnection) {
	carState := ls.CarState[event.CarID]
	if carState == nil {
		carState = &CarState{}
		carState.Position = len(ls.CarState) + 1
	}
	carState.CarID = event.CarID
	carState.RaceNumber = event.RaceNumber
	carState.CarModel = accdata.CarModelByID(event.CarModelID)

	if driver := ls.lookupDriverForNewCarConnection(event); driver != nil {
		carState.Drivers = append(carState.Drivers, driver)
		if carState.CurrentDriver == nil {
			carState.CurrentDriver = driver
		}
		ls.carPerConnection[driver.ConnectionID] = carState.CarID
	}

	ls.setCarState(carState)
}

func (ls *LiveState) handleDeadConnection(event logEventDeadConnection) {
	// TODO: clean connection requests

	driver := ls.driverPerConnection[event.ConnectionID]
	carID := ls.carPerConnection[event.ConnectionID]

	if carState := ls.CarState[carID]; carState != nil {
		for i := 0; i < len(carState.Drivers); i++ {
			if carState.Drivers[i] == driver {
				copy(carState.Drivers[i:], carState.Drivers[i+1:])
				carState.Drivers = carState.Drivers[:len(carState.Drivers)-1]
				i--
			}
		}
		ls.setCarState(carState)
	}

	if driver != nil {
		delete(ls.driverPerConnection, event.ConnectionID)
	}
	if carID != 0 {
		delete(ls.carPerConnection, event.ConnectionID)
	}
}

func (ls *LiveState) handleCarPurged(event logEventCarPurged) {
	ls.purgeCar(event.CarID)
	ls.recalculatePositions()
}

func (ls *LiveState) handleNewLapTime(event logEventNewLapTime) {
	if carState := ls.CarState[event.CarID]; carState != nil {
		if event.Flags == 0 && (carState.BestLapMS <= 0 || event.LapTimeMS < carState.BestLapMS) {
			carState.BestLapMS = event.LapTimeMS
			ls.setCarState(carState)
			ls.recalculatePositions()
		}
	}
}
