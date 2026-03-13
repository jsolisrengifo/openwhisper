package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// ttsRequest matches the Google Cloud Text-to-Speech REST API body.
type ttsRequest struct {
	Input       ttsInput       `json:"input"`
	Voice       ttsVoice       `json:"voice"`
	AudioConfig ttsAudioConfig `json:"audioConfig"`
}

type ttsInput struct {
	Text string `json:"text"`
}

type ttsVoice struct {
	LanguageCode string `json:"languageCode"`
	SsmlGender   string `json:"ssmlGender"`
	Name         string `json:"name"`
}

type ttsAudioConfig struct {
	AudioEncoding    string   `json:"audioEncoding"`
	EffectsProfileId []string `json:"effectsProfileId"`
	SpeakingRate     float64  `json:"speakingRate"`
	Pitch            float64  `json:"pitch"`
}

type ttsResponse struct {
	AudioContent string `json:"audioContent"` // base64-encoded MP3
	Error        *struct {
		Message string `json:"message"`
		Code    int    `json:"code"`
	} `json:"error,omitempty"`
}

const ttsMaxChars = 4500 // Conservative limit below the 5 000-byte API maximum.

// synthesizeSpeech calls the Google Cloud Text-to-Speech REST API and returns
// the base64-encoded MP3 audio content.
// speakingRate must be between 0.25 and 4.0 (1.0 = normal speed).
func synthesizeSpeech(text, apiKey string, speakingRate float64) (string, error) {
	if text == "" {
		return "", fmt.Errorf("no hay texto para sintetizar")
	}
	// Clamp speaking rate to valid API range.
	if speakingRate < 0.25 {
		speakingRate = 0.25
	} else if speakingRate > 4.0 {
		speakingRate = 4.0
	}
	// Truncate silently to avoid API errors on very long selections.
	if len([]rune(text)) > ttsMaxChars {
		runes := []rune(text)
		text = string(runes[:ttsMaxChars])
		logger.Warn("tts: text truncated to avoid API limit", "originalChars", len([]rune(text)), "limit", ttsMaxChars)
	}

	logger.Debug("tts: request start", "chars", len(text), "speakingRate", speakingRate)
	start := time.Now()

	reqBody := ttsRequest{
		Input: ttsInput{Text: text},
		Voice: ttsVoice{
			LanguageCode: "es-US",
			SsmlGender:   "MALE",
			Name:         "es-US-Standard-B",
		},
		AudioConfig: ttsAudioConfig{
			AudioEncoding:    "MP3",
			EffectsProfileId: []string{"small-bluetooth-speaker-class-device"},
			SpeakingRate:     speakingRate,
			Pitch:            0,
		},
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("tts: error building request: %w", err)
	}

	url := fmt.Sprintf("https://texttospeech.googleapis.com/v1/text:synthesize?key=%s", apiKey)

	const maxAttempts = 3
	var lastErr error

	for attempt := range maxAttempts {
		if attempt > 0 {
			waitTime := time.Duration(1<<attempt) * time.Second
			logger.Warn("tts: retrying", "attempt", attempt, "of", maxAttempts-1, "waitSeconds", waitTime.Seconds())
			time.Sleep(waitTime)
		}

		resp, err := http.Post(url, "application/json", bytes.NewReader(data)) //nolint:noctx
		if err != nil {
			logger.Error("tts: HTTP request failed", "err", err, "attempt", attempt+1)
			lastErr = fmt.Errorf("tts: error calling API: %w", err)
			continue
		}

		statusCode := resp.StatusCode
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = fmt.Errorf("tts: error reading response: %w", err)
			continue
		}

		var ttsResp ttsResponse
		if err := json.Unmarshal(body, &ttsResp); err != nil {
			return "", fmt.Errorf("tts: error parsing response: %w", err)
		}

		if ttsResp.Error != nil {
			logger.Error("tts: API error", "code", ttsResp.Error.Code, "message", ttsResp.Error.Message, "attempt", attempt+1)
			lastErr = fmt.Errorf("Google TTS error %d: %s", ttsResp.Error.Code, ttsResp.Error.Message)
			if !isRetryable(ttsResp.Error.Code) {
				return "", lastErr
			}
			continue
		}

		if isRetryable(statusCode) {
			lastErr = fmt.Errorf("tts: HTTP error %d", statusCode)
			continue
		}

		if ttsResp.AudioContent == "" {
			return "", fmt.Errorf("tts: respuesta vacía de Google TTS")
		}

		elapsed := time.Since(start)
		logger.Info("tts: response ok",
			"elapsed", elapsed.String(),
			"elapsedMs", elapsed.Milliseconds(),
			"audioBase64Len", len(ttsResp.AudioContent),
		)
		return ttsResp.AudioContent, nil
	}

	return "", lastErr
}
