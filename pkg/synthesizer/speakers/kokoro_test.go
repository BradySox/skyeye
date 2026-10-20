//go:build !darwin

package speakers

import (
	"encoding/binary"
	"testing"

	"github.com/martinlindhe/unit"
	"github.com/stretchr/testify/require"
)

func TestWavPCM(t *testing.T) {
	t.Parallel()
	data := []byte{1, 2, 3, 4}
	wav := []byte("RIFF\x00\x00\x00\x00WAVEfmt ")
	wav = binary.LittleEndian.AppendUint32(wav, 16)
	wav = append(wav, 1, 0, 1, 0)
	wav = binary.LittleEndian.AppendUint32(wav, 24000)
	wav = binary.LittleEndian.AppendUint32(wav, 48000)
	wav = append(wav, 2, 0, 16, 0)
	wav = append(wav, []byte("data")...)
	wav = binary.LittleEndian.AppendUint32(wav, uint32(len(data)))
	wav = append(wav, data...)
	samples, rate, err := wavPCM(wav)
	require.NoError(t, err)
	require.Equal(t, data, samples)
	require.Equal(t, 24000*unit.Hertz, rate)

	_, _, err = wavPCM([]byte("nope"))
	require.Error(t, err)
}

func TestKokoroVoices(t *testing.T) {
	t.Parallel()
	_, err := NewKokoroSpeaker(t.TempDir(), "am_eric", 1)
	require.ErrorContains(t, err, "needs")
	_, err = NewKokoroSpeaker(t.TempDir(), "zz_nobody", 1)
	require.ErrorContains(t, err, "no voice")
	require.Equal(t, 13, indexOf("am_eric"))
	require.Equal(t, 21, indexOf("bf_emma"))
}

func indexOf(voice string) int {
	for i, v := range kokoroSpeakers {
		if v == voice {
			return i
		}
	}
	return -1
}
