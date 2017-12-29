package jukybox

import (
	"bytes"
	"github.com/godbus/dbus"
	"github.com/google/uuid"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"
)

const (
	_ = iota
	Starting
	Playing
	Paused
	Finishing
	Finished
)

const playerDBusDomain = "re.mko.jukybox.player"

type PlayerEvent struct {
	State    int
	File     string
	Position time.Duration
}

type PlayerCommand interface{}
type PlayCommand struct {
	file        string
	position    time.Duration
	passthrough bool
}
type StopCommand struct{}
type PauseCommand struct{}

type Player struct {
	state                 int
	events                chan<- PlayerEvent
	commands              chan PlayerCommand
	nextPlayFile          string
	nextPlayPosition      time.Duration
	nextPlayPassthrough   bool
	playerFile            string
	playerDBusName        string
	playerCmd             *exec.Cmd
	playerFinishedChannel chan error
	playerPosition        time.Duration
	playerPositionUpdated time.Time
	dbusConnection        *dbus.Conn
	dbusChannel           chan *dbus.Signal
	positionPollTimer     *time.Ticker
	halting               bool
	done                  chan bool
}

func runCommand(cmd *exec.Cmd, finished chan<- error) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	log.Printf("Finished process %v stdout:%v stderr:%v\n", err, string(stdout.String()), string(stderr.String()))
	finished <- err
}

func NewPlayer(events chan<- PlayerEvent) *Player {
	conn, err := dbus.SessionBus()
	if err != nil {
		log.Fatalf("Failed to connect to session bus:", err)
	}

	player := Player{
		state:          Finished,
		events:         events,
		commands:       make(chan PlayerCommand, 10), // Sometimes there's a deadlock if a command comes in and the buffer is 0
		dbusConnection: conn,
		dbusChannel:    make(chan *dbus.Signal, 10),
		done:           make(chan bool),
		// positionPollTimer: time.NewTicker(1 * time.Second),
	}
	conn.Signal(player.dbusChannel)

	conn.BusObject().Call("org.freedesktop.DBus.AddMatch", 0, "type='signal',path='/org/freedesktop/DBus',interface='org.freedesktop.DBus',member='NameOwnerChanged',sender='org.freedesktop.DBus'")

	go player.run()
	return &player
}

func (p *Player) run() {
mainLoop:
	for {
		var positionPollChannel <-chan time.Time
		if p.positionPollTimer != nil {
			positionPollChannel = p.positionPollTimer.C
		}
		// log.Printf("Player: Waiting for event")
		select {
		case cmd := <-p.commands:
			log.Printf("Player command: %#v\n", cmd)
			switch cmd := cmd.(type) {
			case PlayCommand:
				// log.Printf("State: %v %v %v\n", p.playerFile, cmd.file, p.state)
				if (p.state == Playing || p.state == Paused) && p.playerFile == cmd.file {
					obj := p.dbusConnection.Object(p.playerDBusName, "/org/mpris/MediaPlayer2")

					// Set position
					if p.playerPosition != cmd.position {
						log.Printf("Set position\n")
						call := obj.Call("org.mpris.MediaPlayer2.Player.SetPosition", 0, dbus.ObjectPath("/not/used"), cmd.position/1000)
						if call.Err != nil {
							log.Printf("Failed to set position: %v\n", call.Err)
						} else {
							log.Printf("Finished set")
						}
						p.playerPosition = cmd.position
					}
					p.playerPositionUpdated = time.Now()
					if p.state == Paused {
						call := obj.Call("org.mpris.MediaPlayer2.Player.Play", 0)
						if call.Err != nil {
							log.Printf("Failed to start playing: %v\n", call.Err)
						}
						p.changeState(Playing)
					} else {
						p.emitEventWithPosition(cmd.position)
					}
				} else if p.state == Starting && p.playerFile == cmd.file {
					// Already starting new file
					log.Printf("Already starting file\n")
				} else if p.state == Finished {
					p.play(cmd.file, cmd.position, cmd.passthrough)
				} else {
					log.Printf("Queuing file %v\n", cmd.file)
					p.nextPlayFile = cmd.file
					p.nextPlayPosition = cmd.position
					p.nextPlayPassthrough = cmd.passthrough
					p.stop()
				}
			case PauseCommand:
				obj := p.dbusConnection.Object(p.playerDBusName, "/org/mpris/MediaPlayer2")
				call := obj.Call("org.mpris.MediaPlayer2.Player.Pause", 0)
				// call := obj.Call("org.mpris.MediaPlayer2.Player.Action", 0, int64(16))
				if call.Err != nil {
					log.Printf("Failed to pause player: %v\n", call.Err)
				}
				p.changeState(Paused)
			case StopCommand:
				if p.state == Finished {
					break mainLoop
				} else {
					p.halting = true
					p.stop()
				}
			}

		case err := <-p.playerFinishedChannel:
			log.Printf("Command Finished: %v\n", err)
			if p.halting {
				break mainLoop
			} else {
				p.changeState(Finished)
			}

		case signal := <-p.dbusChannel:
			log.Printf("DBus signal: %v; Path: %v; Name: %v; Body: %v\n", signal.Sender, signal.Path, signal.Name, signal.Body)
			switch signal.Name {
			case "org.freedesktop.DBus.NameOwnerChanged":
				if len(signal.Body) < 3 {
					log.Printf("Unexpected no arguments\n")
				} else {
					name, ok1 := signal.Body[0].(string)
					oldOwner, ok2 := signal.Body[1].(string)
					newOwner, ok3 := signal.Body[2].(string)
					if !ok1 || !ok2 || !ok3 {
						log.Printf("Unexpected argument type\n")
					}
					if name == p.playerDBusName {
						if oldOwner == "" {
							p.changeState(Playing)
						} else if newOwner == "" {
							// DBUs command may arrive after actual finish
							if p.state != Finished {
								p.changeState(Finishing)
							}
						}
					}
				}
			}
		case <-positionPollChannel:
			obj := p.dbusConnection.Object(p.playerDBusName, "/org/mpris/MediaPlayer2")
			position, err := obj.GetProperty("org.mpris.MediaPlayer2.Player.Position")
			if err != nil {
				log.Printf("Failed to get position: %v\n", err)
			} else {
				actualPosition := time.Duration(position.Value().(int64)) * time.Microsecond
				p.playerPositionUpdated = time.Now()
				p.playerPosition = actualPosition
				p.emitEventWithPosition(actualPosition)
			}

		}
	}

	log.Printf("Signaling done\n")
	p.done <- true
	log.Printf("Signaled done\n")
}

func (p *Player) stop() {
	if p.state == Finishing {
		return
	}
	log.Printf("Interrupting current player")
	p.changeState(Finishing)
	err := p.playerCmd.Process.Signal(os.Interrupt)
	if err != nil {
		log.Printf("Error: %v\n", err)
	}
	log.Printf("Interrupted current player")
}

func (p *Player) play(file string, position time.Duration, passthrough bool) {
	p.playerDBusName = playerDBusDomain + ".p" + strings.Replace(uuid.Must(uuid.NewRandom()).String(), "-", "", -1)
	command := playCommand(file, position, passthrough, p.playerDBusName)
	log.Printf("Executing command %v\n", command)
	p.playerCmd = exec.Command(command[0], command[1:]...)
	p.playerFile = file
	p.playerFinishedChannel = make(chan error)
	p.playerPosition = position
	p.playerPositionUpdated = time.Now()
	p.nextPlayFile = ""
	p.nextPlayPassthrough = false
	p.changeState(Starting)
	go runCommand(p.playerCmd, p.playerFinishedChannel)
}

func (p *Player) changeState(state int) {
	if p.state == state {
		return
	}
	log.Printf("Changing state: %v -> %v", p.state, state)

	if state != Playing && p.positionPollTimer != nil {
		log.Printf("Stopping poll timer")
		p.positionPollTimer.Stop()
		p.positionPollTimer = nil
	}

	switch state {
	case Playing:
		p.state = Playing
		p.positionPollTimer = time.NewTicker(1 * time.Second)
		p.playerPositionUpdated = time.Now()
		// p.playerPosition has been set when requesting play start
		p.emitEventWithPosition(p.playerPosition)
	case Finishing:
	case Finished:
		p.playerCmd = nil
		p.playerFinishedChannel = nil
		if len(p.nextPlayFile) != 0 {
			p.play(p.nextPlayFile, p.nextPlayPosition, p.nextPlayPassthrough)
		} else {
			p.setState(Finished)
		}
	default:
		p.setState(state)
	}
}

func (p *Player) setState(state int) {
	p.state = state
	p.emitEvent()
}

func (p *Player) emitEvent() {
	if p.halting {
		return
	}
	p.events <- PlayerEvent{State: p.state, File: p.playerFile, Position: p.playerPosition + time.Since(p.playerPositionUpdated)}
}

func (p *Player) emitEventWithPosition(position time.Duration) {
	if p.halting {
		return
	}
	// log.Printf("Player: emitting event")
	p.events <- PlayerEvent{State: p.state, File: p.playerFile, Position: position}
	// log.Printf("Player: finished emitting event")
}

////////////////////////////////////////////////////////////////////////////////

func (p *Player) Play(path string, position time.Duration, passthrough bool) {
	log.Printf("Sending play command")
	p.commands <- PlayCommand{file: path, position: position, passthrough: passthrough}
	log.Printf("Sent play command")
}

func (p *Player) Pause() {
	p.commands <- PauseCommand{}
}

func (p *Player) Stop() {
	p.commands <- StopCommand{}
	log.Printf("Waiting for player to stop\n")
	<-p.done
	log.Printf("Done!\n")
}
