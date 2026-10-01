package stt

import (
	"context"
	"encoding/binary"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/config"
)

func speechFixture(t *testing.T) []float32 {
	t.Helper()
	// Downloaded by make captions-assets, SHA256 pinned; not committed or embedded.
	data, err := os.ReadFile("assets/jfk.wav")
	require.NoError(t, err)
	require.Equal(t, "RIFF", string(data[:4]))
	require.Equal(t, "WAVE", string(data[8:12]))
	var audio []byte
	for pos := 12; pos+8 <= len(data); {
		n := int(binary.LittleEndian.Uint32(data[pos+4 : pos+8]))
		body := pos + 8
		require.LessOrEqual(t, body+n, len(data))
		switch string(data[pos : pos+4]) {
		case "fmt ":
			require.EqualValues(t, 1, binary.LittleEndian.Uint16(data[body:]))
			require.EqualValues(t, 1, binary.LittleEndian.Uint16(data[body+2:]))
			require.EqualValues(t, SampleRate, binary.LittleEndian.Uint32(data[body+4:]))
			require.EqualValues(t, 16, binary.LittleEndian.Uint16(data[body+14:]))
		case "data":
			audio = data[body : body+n]
		}
		pos = body + n + n%2
	}
	pcm := make([]float32, len(audio)/2)
	for i := range pcm {
		pcm[i] = float32(int16(binary.LittleEndian.Uint16(audio[i*2:]))) / 32768
	}
	return pcm
}
func TestWhisperSpeechWords(t *testing.T) {
	require.True(t, supportedCPU(), "integration test requires supported CPU")
	m := &whisperModel{info: ModelInfo{Name: "whisper-tiny-q5_1", Size: "tiny", Bundled: true, Multilingual: true}, path: "ggml-tiny-q5_1.bin", threads: 4}
	defer m.close()
	pcm := speechFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	result, err := m.Transcribe(ctx, pcm, Options{Language: "en-US", Prompt: "John F. Kennedy"})
	require.NoError(t, err)
	words := make([]string, len(result.Words))
	last := time.Duration(0)
	for i, w := range result.Words {
		words[i] = w.Text
		require.GreaterOrEqual(t, w.Start, last)
		require.GreaterOrEqual(t, w.End, w.Start)
		require.LessOrEqual(t, w.End, time.Duration(len(pcm))*time.Second/SampleRate)
		require.GreaterOrEqual(t, w.Prob, float32(0))
		require.LessOrEqual(t, w.Prob, float32(1))
		last = w.Start
	}
	text := strings.ToLower(strings.Join(words, " "))
	require.Contains(t, text, "ask not what your country")
	// The address continues through the second half of the clip. All-zero
	// timestamp output would satisfy ordering but would be unusable for captions.
	require.Greater(t, result.Words[len(result.Words)-1].End, time.Duration(len(pcm))*time.Second/SampleRate/2)
	require.Equal(t, "en", result.Language)
	require.Less(t, result.NoSpeechProb, float32(.5))
	detected, err := m.Transcribe(ctx, pcm, Options{})
	require.NoError(t, err)
	require.Equal(t, "en", detected.Language)
	silence, err := m.Transcribe(ctx, make([]float32, SampleRate), Options{})
	require.NoError(t, err)
	require.Empty(t, silence.Words)
	require.Equal(t, float32(1), silence.NoSpeechProb)
	canceled, stop := context.WithCancel(ctx)
	stop()
	_, err = m.Transcribe(canceled, pcm, Options{})
	require.ErrorIs(t, err, context.Canceled)
	_, err = m.Transcribe(ctx, pcm, Options{Language: "invalid"})
	require.ErrorContains(t, err, "unsupported speech language")
}
func TestEngineBenchmarkSmoke(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	engine, err := NewEngine(ctx, &config.CLI{CaptionsCPUBudget: .5})
	require.NoError(t, err)
	defer engine.Close()
	e := engine.(*scheduler)
	select {
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	case <-e.ready:
	}
	models := engine.Models()
	require.Len(t, models, 3)
	for _, m := range models {
		require.Greater(t, m.RealtimeFactor, 0.0)
		t.Logf("%s realtime factor %.6f", m.Name, m.RealtimeFactor)
	}
}
