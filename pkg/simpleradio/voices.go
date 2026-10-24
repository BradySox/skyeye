package simpleradio

import (
	"fmt"
	"sync"
	"time"

	"github.com/dharmab/skyeye/pkg/simpleradio/types"
	"github.com/dharmab/skyeye/pkg/simpleradio/voice"
	"github.com/rs/zerolog/log"
)

// A voice is a transmit-only client that speaks for another client under its own name, so the SRS
// client list and overlay show who is talking: "Hatzerim Ground [BOT]" rather than the GCI. The
// owner keeps listening and recognizing speech, and its transmit queue sends every call, so calls
// never overlap and keep their order. A voice is logged in on the frequencies it speaks on and
// throws away anything it hears.

// NewVoice connects a voice for c named name, on the given frequencies. Run it like c. A
// Transmission with From set to it goes out under its name.
func (c *Client) NewVoice(name string, frequencies []RadioFrequency) (*Client, error) {
	v := c.buildVoice(name, frequencies)
	if err := v.connectTCP(); err != nil {
		return nil, fmt.Errorf("failed to connect voice %q to SRS server: %w", name, err)
	}
	if err := v.connectUDP(); err != nil {
		_ = v.tcpConnection.Close()
		return nil, fmt.Errorf("failed to connect voice %q to SRS server: %w", name, err)
	}
	c.addOwnVoice(v.clientInfo.GUID)
	return v, nil
}

// buildVoice builds a voice for c without touching the network.
func (c *Client) buildVoice(name string, frequencies []RadioFrequency) *Client {
	owner := c.info()
	radios := make([]types.Radio, 0, len(frequencies))
	receivers := make(map[types.Radio]*receiver, len(frequencies))
	for _, frequency := range frequencies {
		radio := toRadio(frequency)
		known := false
		for _, existing := range radios {
			if existing.IsSameFrequency(radio) {
				known = true
				break
			}
		}
		if !known {
			radios = append(radios, radio)
			receivers[radio] = &receiver{}
		}
	}

	v := &Client{}
	v.owner = c
	v.address = c.address
	v.externalAWACSModePassword = c.externalAWACSModePassword
	v.clientInfo.Name = name
	v.clientInfo.GUID = types.NewGUID()
	v.clientInfo.Coalition = owner.Coalition
	v.clientInfo.RadioInfo.UnitID = owner.RadioInfo.UnitID
	v.clientInfo.RadioInfo.Unit = owner.RadioInfo.Unit
	v.clientInfo.RadioInfo.Radios = radios
	v.clientInfo.RadioInfo.IFF = types.NewIFF()
	v.clientInfo.RadioInfo.Ambient = types.NewAmbient()
	v.clientInfo.AllowRecording = owner.AllowRecording
	v.clientInfo.Position = &types.Position{}
	v.clients = make(map[types.GUID]types.ClientInfo)
	v.receivers = receivers
	v.packetNumber = 1
	v.mute = c.mute
	v.lastPing = time.Now()
	v.ready = make(chan struct{})
	return v
}

// addOwnVoice marks a GUID as one of c's voices: c ignores what it says and leaves it out of its
// peers.
func (c *Client) addOwnVoice(guid types.GUID) {
	c.clientsLock.Lock()
	defer c.clientsLock.Unlock()
	if c.ownVoices == nil {
		c.ownVoices = make(map[types.GUID]struct{})
	}
	c.ownVoices[guid] = struct{}{}
	delete(c.clients, guid)
}

// isOwnVoice is true if the GUID is one of c's voices.
func (c *Client) isOwnVoice(guid types.GUID) bool {
	c.clientsLock.RLock()
	defer c.clientsLock.RUnlock()
	_, ok := c.ownVoices[guid]
	return ok
}

// transmitLock is the lock held while a call is on the air, shared by a client and its voices, so a
// voice's re-login never lands during a call either.
func (c *Client) transmitLock() *sync.Mutex {
	if c.owner != nil {
		return c.owner.transmitLock()
	}
	return &c.txLock
}

// markReady records that the client is logged in to the SRS server.
func (c *Client) markReady() {
	if c.ready == nil {
		return
	}
	c.readyOnce.Do(func() { close(c.ready) })
}

// isReady is true once the client has logged in to the SRS server.
func (c *Client) isReady() bool {
	select {
	case <-c.ready:
		return true
	default:
		return false
	}
}

// WaitReady waits up to timeout for the client to log in to the SRS server, and is true if it has.
func (c *Client) WaitReady(timeout time.Duration) bool {
	if c.isReady() {
		return true
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-c.ready:
		return true
	case <-timer.C:
		return false
	}
}

// senderFor is the client a transmission goes out under: its voice, if it names one of c's that is
// logged in, else c.
func (c *Client) senderFor(transmission Transmission) *Client {
	from := transmission.From
	if from == nil || from == c {
		return c
	}
	if from.owner != c {
		log.Warn().Str("voice", from.Name()).Msg("voice belongs to another client, transmitting as this client")
		return c
	}
	if !from.isReady() {
		log.Warn().Str("voice", from.Name()).Msg("voice is not logged in yet, transmitting as this client")
		return c
	}
	return from
}

// discardVoice throws away the voice packets a voice hears.
func discardVoice(in <-chan []byte, done <-chan struct{}) {
	for {
		select {
		case <-in:
		case <-done:
			return
		}
	}
}

// Name is the client's name in the SRS client list.
func (c *Client) Name() string {
	c.radioLock.RLock()
	defer c.radioLock.RUnlock()
	return c.clientInfo.Name
}

// SetName renames the client in the SRS client list. A connected client sends the server an update,
// which an SRS server takes from a client at any time.
func (c *Client) SetName(name string) error {
	c.radioLock.Lock()
	changed := c.clientInfo.Name != name
	c.clientInfo.Name = name
	c.radioLock.Unlock()
	if !changed || c.tcpConnection == nil {
		return nil
	}
	if err := c.Send(c.newMessage(types.MessageUpdate)); err != nil {
		return fmt.Errorf("rename failed: %w", err)
	}
	log.Info().Str("name", name).Msg("renamed SRS client")
	return nil
}

// info is a copy of the client's information, with every radio.
func (c *Client) info() types.ClientInfo {
	c.radioLock.RLock()
	defer c.radioLock.RUnlock()
	info := c.clientInfo
	info.RadioInfo.Radios = append([]types.Radio(nil), c.clientInfo.RadioInfo.Radios...)
	return info
}

// outgoing is one transmission's voice packets and the client they go out under.
type outgoing struct {
	// from is the client whose GUID the packets carry and whose connection sends them.
	from *Client
	// packets of the transmission.
	packets []voice.Packet
}
