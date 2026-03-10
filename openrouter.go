package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// openRouterMessage is a single message in the OpenAI-compatible chat format.
// Content can be a plain string (text-only turns) or a []openRouterContent slice (multimodal turns).
type openRouterMessage struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"`
}

type openRouterContent struct {
	Type       string           `json:"type"`
	Text       string           `json:"text,omitempty"`
	InputAudio *openRouterAudio `json:"input_audio,omitempty"`
}

type openRouterAudio struct {
	Data   string `json:"data"`
	Format string `json:"format"`
}

type openRouterRequest struct {
	Model    string              `json:"model"`
	Messages []openRouterMessage `json:"messages"`
}

type openRouterResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
		Code    int    `json:"code"`
	} `json:"error"`
}

// mimeToAudioFormat converts a MIME type (e.g. "audio/webm") to the short format
// string expected by the OpenRouter input_audio field.
func mimeToAudioFormat(mimeType string) string {
	switch mimeType {
	case "audio/wav", "audio/wave":
		return "wav"
	case "audio/mp4", "audio/m4a":
		return "mp4"
	case "audio/ogg":
		return "ogg"
	case "audio/mp3", "audio/mpeg":
		return "mp3"
	default:
		return "webm"
	}
}

// doOpenRouterPost executes a single HTTP POST to the OpenRouter chat completions endpoint.
// Returns (statusCode, body, error).
func doOpenRouterPost(apiKey string, payload []byte) (int, []byte, error) {
	req, err := http.NewRequest("POST", "https://openrouter.ai/api/v1/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return 0, nil, fmt.Errorf("error creating openrouter request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("X-Title", "OpenWhisper")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("error calling OpenRouter API: %w", err)
	}
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return resp.StatusCode, nil, fmt.Errorf("error reading openrouter response: %w", err)
	}
	return resp.StatusCode, body, nil
}

// extractOpenRouterText parses the response body and returns the text content.
func extractOpenRouterText(body []byte, attempt int, elapsed time.Duration) (string, error, bool) {
	var orResp openRouterResponse
	if err := json.Unmarshal(body, &orResp); err != nil {
		return "", fmt.Errorf("error parsing openrouter response: %w", err), false
	}
	if orResp.Error != nil {
		logger.Error("openrouter: API error",
			"code", orResp.Error.Code,
			"message", orResp.Error.Message,
			"attempt", attempt,
			"elapsed", elapsed.String(),
		)
		err := fmt.Errorf("OpenRouter API error %d: %s", orResp.Error.Code, orResp.Error.Message)
		return "", err, isRetryable(orResp.Error.Code)
	}
	if len(orResp.Choices) == 0 || orResp.Choices[0].Message.Content == "" {
		return "", fmt.Errorf("respuesta vacía de OpenRouter"), false
	}
	return orResp.Choices[0].Message.Content, nil, false
}

// callOpenRouterAudio sends audio + a text prompt to OpenRouter (OpenAI-compatible API).
// Retries up to 3 times on transient errors with exponential backoff.
func callOpenRouterAudio(base64Audio, mimeType, apiKey, model, prompt string) (string, error) {
	if mimeType == "" {
		mimeType = "audio/webm"
	}

	logger.Debug("openrouter: request start", "model", model, "mimeType", mimeType, "audioBytes", len(base64Audio))
	start := time.Now()

	reqBody := openRouterRequest{
		Model: model,
		Messages: []openRouterMessage{
			{
				Role: "user",
				Content: []openRouterContent{
					{
						Type: "input_audio",
						InputAudio: &openRouterAudio{
							Data:   base64Audio,
							Format: mimeToAudioFormat(mimeType),
						},
					},
					{Type: "text", Text: prompt},
				},
			},
		},
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("error building openrouter request: %w", err)
	}

	const maxAttempts = 3
	var lastErr error

	for attempt := range maxAttempts {
		if attempt > 0 {
			waitTime := time.Duration(1<<attempt) * time.Second
			logger.Warn("openrouter: reintento", "attempt", attempt, "of", maxAttempts-1, "waitSeconds", waitTime.Seconds())
			time.Sleep(waitTime)
		}

		statusCode, body, err := doOpenRouterPost(apiKey, data)
		if err != nil {
			logger.Error("openrouter: HTTP request failed", "err", err, "attempt", attempt+1, "elapsed", time.Since(start).String())
			lastErr = err
			continue
		}

		if isRetryable(statusCode) && len(body) == 0 {
			lastErr = fmt.Errorf("OpenRouter HTTP error %d", statusCode)
			continue
		}

		text, parseErr, shouldRetry := extractOpenRouterText(body, attempt+1, time.Since(start))
		if parseErr != nil {
			lastErr = parseErr
			if !shouldRetry {
				return "", lastErr
			}
			continue
		}

		elapsed := time.Since(start)
		logger.Info("openrouter: response ok",
			"model", model,
			"elapsed", elapsed.String(),
			"elapsedMs", elapsed.Milliseconds(),
			"chars", len(text),
		)
		return text, nil
	}

	return "", lastErr
}

// continueOpenRouterChat sends a multi-turn conversation with an audio user turn to OpenRouter.
// Previous turns in history are sent as plain-text messages.
func continueOpenRouterChat(base64Audio, mimeType, apiKey, model string, history []ChatTurn) (string, error) {
	if mimeType == "" {
		mimeType = "audio/webm"
	}

	logger.Debug("openrouter: continueChat", "model", model, "historyLen", len(history))
	start := time.Now()

	messages := make([]openRouterMessage, 0, len(history)+1)
	for _, t := range history {
		role := t.Role
		// OpenRouter/OpenAI uses "assistant" instead of Gemini's "model"
		if role == "model" {
			role = "assistant"
		} else if role != "user" && role != "assistant" {
			role = "user"
		}
		messages = append(messages, openRouterMessage{Role: role, Content: t.Text})
	}
	// New user turn with audio
	messages = append(messages, openRouterMessage{
		Role: "user",
		Content: []openRouterContent{
			{
				Type: "input_audio",
				InputAudio: &openRouterAudio{
					Data:   base64Audio,
					Format: mimeToAudioFormat(mimeType),
				},
			},
			{Type: "text", Text: askPrompt},
		},
	})

	reqBody := openRouterRequest{Model: model, Messages: messages}
	data, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("error building openrouter chat request: %w", err)
	}

	const maxAttempts = 3
	var lastErr error

	for attempt := range maxAttempts {
		if attempt > 0 {
			waitTime := time.Duration(1<<attempt) * time.Second
			logger.Warn("openrouter: continueChat reintento", "attempt", attempt, "waitSeconds", waitTime.Seconds())
			time.Sleep(waitTime)
		}

		statusCode, body, err := doOpenRouterPost(apiKey, data)
		if err != nil {
			logger.Error("openrouter: continueChat HTTP failed", "err", err, "attempt", attempt+1)
			lastErr = err
			continue
		}

		if isRetryable(statusCode) && len(body) == 0 {
			lastErr = fmt.Errorf("OpenRouter HTTP error %d", statusCode)
			continue
		}

		text, parseErr, shouldRetry := extractOpenRouterText(body, attempt+1, time.Since(start))
		if parseErr != nil {
			lastErr = parseErr
			if !shouldRetry {
				return "", lastErr
			}
			continue
		}

		elapsed := time.Since(start)
		logger.Info("openrouter: continueChat response ok",
			"model", model,
			"elapsed", elapsed.String(),
			"elapsedMs", elapsed.Milliseconds(),
			"chars", len(text),
		)
		return text, nil
	}

	return "", lastErr
}
