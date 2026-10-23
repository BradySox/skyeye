package simpleradio

import (
	"fmt"

	"github.com/dharmab/skyeye/pkg/simpleradio/types"
	"github.com/dharmab/skyeye/pkg/simpleradio/voice"
	"github.com/martinlindhe/unit"
	"github.com/rs/zerolog/log"
)

// toRadio converts a RadioFrequency to an SRS radio.
func toRadio(f RadioFrequency) types.Radio {
	return types.Radio{
		Frequency:  f.Frequency.Hertz(),
		Modulation: f.Modulation,
	}
}

// toFrequency converts an SRS radio to a RadioFrequency.
func toFrequency(r types.Radio) RadioFrequency {
	return RadioFrequency{
		Frequency:  unit.Frequency(r.Frequency) * unit.Hertz,
		Modulation: r.Modulation,
	}
}

// AddListenRadios tunes extra radios the client hears on. Calls heard on them reach Receive like any
// other, but the client transmits on them only when a Transmission names them in Frequencies.
// Frequencies the client already has are skipped. The SRS server is sent the new radio list with a
// sync and the External AWACS Mode login: an SRS server ignores a radio update from an EAM client,
// so a radio added after connecting would never get any audio routed to it.
func (c *Client) AddListenRadios(frequencies []RadioFrequency) error {
	added := 0
	func() {
		c.radioLock.Lock()
		defer c.radioLock.Unlock()
		for _, frequency := range frequencies {
			radio := toRadio(frequency)
			known := false
			for _, existing := range c.clientInfo.RadioInfo.Radios {
				if existing.IsSameFrequency(radio) {
					known = true
					break
				}
			}
			if known {
				continue
			}
			c.clientInfo.RadioInfo.Radios = append(c.clientInfo.RadioInfo.Radios, radio)
			c.receivers[radio] = &receiver{}
			c.listenRadioCount++
			added++
		}
	}()
	if added == 0 {
		return nil
	}
	log.Info().Int("count", added).Msg("tuned listen radios")
	if err := c.resyncRadios(); err != nil {
		return fmt.Errorf("failed to send listen radios: %w", err)
	}
	return nil
}

// resyncRadios sends the client's full radio list to the SRS server the way a fresh connection
// does: a sync, then the External AWACS Mode login, both carrying every radio. It holds txLock so
// the re-login never lands while a call is on the air: a call that aired the same minute as one
// went unheard (2026-10-22).
func (c *Client) resyncRadios() error {
	c.txLock.Lock()
	defer c.txLock.Unlock()
	if err := c.sync(); err != nil {
		return err
	}
	return c.connectExternalAWACSMode()
}

// baseRadioInfo is a copy of the client's radio info holding only the configured frequencies.
func (c *Client) baseRadioInfo() types.RadioInfo {
	c.radioLock.RLock()
	defer c.radioLock.RUnlock()
	info := c.clientInfo.RadioInfo
	base := len(c.clientInfo.RadioInfo.Radios) - c.listenRadioCount
	info.Radios = append([]types.Radio(nil), c.clientInfo.RadioInfo.Radios[:base]...)
	return info
}

// allRadioInfo is a copy of the client's radio info including listen radios.
func (c *Client) allRadioInfo() types.RadioInfo {
	c.radioLock.RLock()
	defer c.radioLock.RUnlock()
	info := c.clientInfo.RadioInfo
	info.Radios = append([]types.Radio(nil), c.clientInfo.RadioInfo.Radios...)
	return info
}

// transmitFrequencies is the frequency list for an outgoing transmission: the transmission's own
// Frequencies if it names any, else the configured frequencies.
func (c *Client) transmitFrequencies(transmission Transmission) []voice.Frequency {
	radios := c.baseRadioInfo().Radios
	if len(transmission.Frequencies) > 0 {
		radios = make([]types.Radio, 0, len(transmission.Frequencies))
		for _, frequency := range transmission.Frequencies {
			radios = append(radios, toRadio(frequency))
		}
	}
	list := make([]voice.Frequency, 0, len(radios))
	for _, radio := range radios {
		list = append(list, voice.Frequency{
			Frequency:  radio.Frequency,
			Modulation: byte(radio.Modulation),
			Encryption: 0,
		})
	}
	return list
}

// heardFrequency is the first of the packet's frequencies that one of the client's radios is tuned to.
func (c *Client) heardFrequency(packet voice.Packet) *RadioFrequency {
	c.radioLock.RLock()
	defer c.radioLock.RUnlock()
	for _, frequency := range packet.Frequencies {
		heard := types.Radio{
			Frequency:   frequency.Frequency,
			Modulation:  types.Modulation(frequency.Modulation),
			IsEncrypted: frequency.Encryption != 0,
		}
		for _, radio := range c.clientInfo.RadioInfo.Radios {
			if radio.IsSameFrequency(heard) {
				f := toFrequency(radio)
				return &f
			}
		}
	}
	return nil
}

// IsListenFrequency is true if the frequency is one of the listen radios added with
// AddListenRadios, rather than a configured frequency.
func (c *Client) IsListenFrequency(frequency RadioFrequency) bool {
	radio := toRadio(frequency)
	for _, base := range c.baseRadioInfo().Radios {
		if base.IsSameFrequency(radio) {
			return false
		}
	}
	for _, listen := range c.allRadioInfo().Radios {
		if listen.IsSameFrequency(radio) {
			return true
		}
	}
	return false
}
