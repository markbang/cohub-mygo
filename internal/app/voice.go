package app

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/websocket"
)

// Voice input, as the SDK's voice-input.ts: 16 kHz mono PCM in 200 ms
// chunks to the gateway's ASR socket, whose partial and final text fill
// the composer as the user speaks. MyGo plays sound but does not record
// it, so the microphone comes from the system's recorder: PipeWire,
// PulseAudio, ALSA, SoX or ffmpeg, whichever is installed.

const (
	voiceRate  = 16000
	voiceChunk = voiceRate * 2 / 5 // 200 ms of 16-bit samples
)

type voiceInput struct {
	a      *app
	mu     sync.Mutex
	ws     *websocket.Conn
	rec    *exec.Cmd
	done   chan struct{}
	prefix string // the draft before the voice's text
	final  string
	active bool
}

// recorders are the commands that write raw 16 kHz mono s16le to stdout.
func recorders() [][]string {
	switch runtime.GOOS {
	case "darwin":
		return [][]string{
			{"rec", "-q", "-t", "raw", "-r", "16000", "-b", "16", "-c", "1", "-e", "signed-integer", "-"},
			{"ffmpeg", "-loglevel", "quiet", "-f", "avfoundation", "-i", ":0", "-ac", "1", "-ar", "16000", "-f", "s16le", "-"},
		}
	case "windows":
		return [][]string{
			{"sox", "-q", "-t", "waveaudio", "default", "-t", "raw", "-r", "16000", "-b", "16", "-c", "1", "-e", "signed-integer", "-"},
			{"ffmpeg", "-loglevel", "quiet", "-f", "dshow", "-i", "audio=default", "-ac", "1", "-ar", "16000", "-f", "s16le", "-"},
		}
	}
	return [][]string{
		{"pw-record", "--rate", "16000", "--channels", "1", "--format", "s16", "-"},
		{"parec", "--rate=16000", "--channels=1", "--format=s16le", "--raw"},
		{"arecord", "-q", "-f", "S16_LE", "-r", "16000", "-c", "1", "-t", "raw"},
		{"rec", "-q", "-t", "raw", "-r", "16000", "-b", "16", "-c", "1", "-e", "signed-integer", "-"},
		{"ffmpeg", "-loglevel", "quiet", "-f", "pulse", "-i", "default", "-ac", "1", "-ar", "16000", "-f", "s16le", "-"},
	}
}

func findRecorder() []string {
	if cmd := strings.Fields(os.Getenv("COHUB_VOICE_CMD")); len(cmd) > 0 {
		return cmd
	}
	for _, r := range recorders() {
		if _, err := exec.LookPath(r[0]); err == nil {
			return r
		}
	}
	return nil
}

func voiceURL(apiBase string) string {
	if strings.Contains(apiBase, "api-dev.") {
		return "wss://gateway-dev.cohub.live/asr/ws"
	}
	return "wss://gateway.cohub.live/asr/ws"
}

// toggleVoice starts listening, or stops and keeps what was heard.
func (a *app) toggleVoice() {
	if a.voice != nil && a.voice.active {
		a.voice.stop()
		return
	}
	if a.api == nil {
		a.sendErr = m("x_demo_no_send")
		return
	}
	cmd := findRecorder()
	if cmd == nil {
		a.sendErr = m("x_voice_no_recorder")
		return
	}
	prefix := a.draft
	if prefix != "" && !strings.HasSuffix(prefix, " ") && !strings.HasSuffix(prefix, "\n") {
		prefix += " "
	}
	v := &voiceInput{a: a, prefix: prefix, active: true, done: make(chan struct{})}
	a.voice, a.sendErr = v, ""
	go v.run(cmd)
}

func (v *voiceInput) fail(err error) {
	v.a.update(func() {
		if v.a.voice == v {
			v.a.sendErr = err.Error()
			v.active = false
		}
	})
	v.cleanup()
}

func (v *voiceInput) show(partial string) {
	v.a.update(func() {
		if v.a.voice == v {
			v.a.draft = v.prefix + v.final + partial
		}
	})
}

func (v *voiceInput) run(cmd []string) {
	cfg, err := websocket.NewConfig(voiceURL(v.a.api.Base()), "https://cohub.live")
	if err != nil {
		v.fail(err)
		return
	}
	ws, err := websocket.DialConfig(cfg)
	if err != nil {
		v.fail(errors.New(m("x_voice_unavailable")))
		return
	}
	v.mu.Lock()
	v.ws = ws
	v.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	token, err := v.a.api.AccessToken(ctx)
	cancel()
	if err != nil {
		v.fail(err)
		return
	}
	websocket.JSON.Send(ws, map[string]any{"type": "auth", "payload": map[string]any{"token": token}})
	started := make(chan struct{})
	go v.read(ws, started)
	select {
	case <-started:
	case <-v.done:
		return
	case <-time.After(15 * time.Second):
		v.fail(errors.New(m("x_voice_unavailable")))
		return
	}
	rec := exec.Command(cmd[0], cmd[1:]...)
	out, err := rec.StdoutPipe()
	if err == nil {
		err = rec.Start()
	}
	if err != nil {
		v.fail(err)
		return
	}
	v.mu.Lock()
	v.rec = rec
	v.mu.Unlock()
	buf := make([]byte, voiceChunk)
	for {
		if _, err := io.ReadFull(out, buf); err != nil {
			break
		}
		if websocket.JSON.Send(ws, map[string]any{"type": "asr.audio", "payload": map[string]any{"audio": base64.StdEncoding.EncodeToString(buf)}}) != nil {
			break
		}
	}
	rec.Wait()
}

func (v *voiceInput) read(ws *websocket.Conn, started chan struct{}) {
	defer v.cleanup()
	for {
		var raw string
		if err := websocket.Message.Receive(ws, &raw); err != nil {
			return
		}
		var ev struct {
			Type    string `json:"type"`
			Payload struct {
				Text    string `json:"text"`
				Code    string `json:"code"`
				Message string `json:"message"`
			} `json:"payload"`
		}
		if json.Unmarshal([]byte(raw), &ev) != nil {
			continue
		}
		switch ev.Type {
		case "system.auth.ok":
			websocket.JSON.Send(ws, map[string]any{"type": "asr.start"})
		case "asr.started":
			close(started)
		case "asr.partial":
			v.show(ev.Payload.Text)
		case "asr.final":
			v.final += ev.Payload.Text
			v.show("")
		case "asr.error":
			msg := ev.Payload.Message
			if msg == "" {
				msg = "Voice input failed"
			}
			v.fail(errors.New(msg))
			return
		case "asr.done":
			v.a.update(func() { v.active = false })
			return
		}
	}
}

// stop ends the recording; the socket closes once the last text came.
func (v *voiceInput) stop() {
	v.mu.Lock()
	rec, ws := v.rec, v.ws
	v.mu.Unlock()
	v.active = false
	if rec != nil && rec.Process != nil {
		rec.Process.Kill()
	}
	if ws != nil {
		websocket.JSON.Send(ws, map[string]any{"type": "asr.stop"})
		go func() {
			time.Sleep(5 * time.Second)
			v.cleanup()
		}()
	} else {
		v.cleanup()
	}
}

func (v *voiceInput) cleanup() {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.rec != nil && v.rec.Process != nil {
		v.rec.Process.Kill()
	}
	if v.ws != nil {
		v.ws.Close()
		v.ws = nil
	}
	select {
	case <-v.done:
	default:
		close(v.done)
	}
}
