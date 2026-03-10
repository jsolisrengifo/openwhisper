package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const askPrompt = "Listen to the audio and answer the question asked. Provide a clear and helpful response in the same language used in the question. Do not add preambles or meta-comments, just answer directly."

// Gemini API request/response types

type geminiRequest struct {
	Contents []geminiContent `json:"contents"`
}

type geminiContent struct {
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text       string            `json:"text,omitempty"`
	InlineData *geminiInlineData `json:"inline_data,omitempty"`
}

type geminiInlineData struct {
	MimeType string `json:"mime_type"`
	Data     string `json:"data"`
}

type geminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
	Error *struct {
		Message string `json:"message"`
		Code    int    `json:"code"`
	} `json:"error"`
}

// isRetryable returns true for transient server-side error codes that warrant a retry.
func isRetryable(code int) bool {
	return code == 429 || code == 500 || code == 502 || code == 503 || code == 504
}

// callGeminiAudio is the shared implementation that sends audio + a text prompt to Gemini.
// It retries up to 3 times total on transient errors, with exponential backoff (2s, 4s).
func callGeminiAudio(base64Audio, mimeType, apiKey, model, prompt string) (string, error) {
	if mimeType == "" {
		mimeType = "audio/webm"
	}

	logger.Debug("gemini: request start", "model", model, "mimeType", mimeType, "audioBytes", len(base64Audio))
	start := time.Now()

	reqBody := geminiRequest{
		Contents: []geminiContent{
			{
				Parts: []geminiPart{
					{
						InlineData: &geminiInlineData{
							MimeType: mimeType,
							Data:     base64Audio,
						},
					},
					{Text: prompt},
				},
			},
		},
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("error building request: %w", err)
	}

	url := fmt.Sprintf(
		"https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s",
		model, apiKey,
	)

	const maxAttempts = 3
	var lastErr error

	for attempt := range maxAttempts {
		if attempt > 0 {
			waitTime := time.Duration(1<<attempt) * time.Second // 2s on attempt 1, 4s on attempt 2
			logger.Warn("gemini: reintento", "attempt", attempt, "of", maxAttempts-1, "waitSeconds", waitTime.Seconds())
			time.Sleep(waitTime)
		}

		resp, err := http.Post(url, "application/json", bytes.NewReader(data)) //nolint:noctx
		if err != nil {
			logger.Error("gemini: HTTP request failed", "err", err, "attempt", attempt+1, "elapsed", time.Since(start).String())
			lastErr = fmt.Errorf("error calling Gemini API: %w", err)
			continue
		}

		statusCode := resp.StatusCode
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = fmt.Errorf("error reading response: %w", err)
			continue
		}

		var gemResp geminiResponse
		if err := json.Unmarshal(body, &gemResp); err != nil {
			return "", fmt.Errorf("error parsing response: %w", err)
		}

		if gemResp.Error != nil {
			logger.Error("gemini: API error", "code", gemResp.Error.Code, "message", gemResp.Error.Message, "attempt", attempt+1, "elapsed", time.Since(start).String())
			lastErr = fmt.Errorf("Gemini API error %d: %s", gemResp.Error.Code, gemResp.Error.Message)
			if !isRetryable(gemResp.Error.Code) {
				return "", lastErr
			}
			continue
		}

		if isRetryable(statusCode) {
			logger.Warn("gemini: retryable HTTP status", "statusCode", statusCode, "attempt", attempt+1, "elapsed", time.Since(start).String())
			lastErr = fmt.Errorf("Gemini API HTTP error %d", statusCode)
			continue
		}

		if len(gemResp.Candidates) == 0 || len(gemResp.Candidates[0].Content.Parts) == 0 {
			logger.Warn("gemini: empty response", "elapsed", time.Since(start).String())
			return "", fmt.Errorf("respuesta vacía de Gemini")
		}

		result := gemResp.Candidates[0].Content.Parts[0].Text
		elapsed := time.Since(start)
		logger.Info("gemini: response ok",
			"model", model,
			"elapsed", elapsed.String(),
			"elapsedMs", elapsed.Milliseconds(),
			"chars", len(result),
		)
		return result, nil
	}

	return "", lastErr
}

// transcribeAudio sends base64-encoded audio to the configured provider using the provided prompt.
func transcribeAudio(base64Audio, mimeType, apiKey, model, prompt, provider string) (string, error) {
	if provider == "openrouter" {
		return callOpenRouterAudio(base64Audio, mimeType, apiKey, model, prompt)
	}
	return callGeminiAudio(base64Audio, mimeType, apiKey, model, prompt)
}

// askQuestion sends base64-encoded audio to the configured provider using the built-in ask prompt.
func askQuestion(base64Audio, mimeType, apiKey, model, provider string) (string, error) {
	if provider == "openrouter" {
		return callOpenRouterAudio(base64Audio, mimeType, apiKey, model, askPrompt)
	}
	return callGeminiAudio(base64Audio, mimeType, apiKey, model, askPrompt)
}

// ChatTurn represents a single message in a multi-turn conversation.
// Role must be "user" or "model".
type ChatTurn struct {
	Role string `json:"role"`
	Text string `json:"text"`
}

// geminiChatContent is a single turn in the Gemini chat API format.
type geminiChatContent struct {
	Role  string       `json:"role"`
	Parts []geminiPart `json:"parts"`
}

// geminiChatRequest is the payload for multi-turn generateContent requests.
type geminiChatRequest struct {
	Contents []geminiChatContent `json:"contents"`
}

// editTextWithAudio sends selected text + audio instruction to the configured provider.
// The provider is expected to return only the modified version of the selected text.
func editTextWithAudio(base64Audio, mimeType, apiKey, model, selectedText, provider string) (string, error) {
	prompt := fmt.Sprintf(
		"The user has selected the following text:\n---\n%s\n---\nListen to their audio instruction and apply it to the text above. Return ONLY the resulting text without explanations, preambles, or formatting marks.",
		selectedText,
	)
	if provider == "openrouter" {
		return callOpenRouterAudio(base64Audio, mimeType, apiKey, model, prompt)
	}
	return callGeminiAudio(base64Audio, mimeType, apiKey, model, prompt)
}

// continueChat sends a follow-up audio message to the configured provider with the full conversation history.
// history contains previous user/model turns (text only); the new user turn carries the audio.
func continueChat(base64Audio, mimeType, apiKey, model string, history []ChatTurn, provider string) (string, error) {
	if provider == "openrouter" {
		return continueOpenRouterChat(base64Audio, mimeType, apiKey, model, history)
	}
	if mimeType == "" {
		mimeType = "audio/webm"
	}

	logger.Debug("gemini: continueChat", "model", model, "historyLen", len(history))
	start := time.Now()

	contents := make([]geminiChatContent, 0, len(history)+1)
	for _, t := range history {
		role := t.Role
		if role != "user" && role != "model" {
			role = "user"
		}
		contents = append(contents, geminiChatContent{
			Role: role,
			Parts: []geminiPart{
				{Text: t.Text},
			},
		})
	}
	// Append the new user turn with audio
	contents = append(contents, geminiChatContent{
		Role: "user",
		Parts: []geminiPart{
			{InlineData: &geminiInlineData{MimeType: mimeType, Data: base64Audio}},
			{Text: askPrompt},
		},
	})

	reqBody := geminiChatRequest{Contents: contents}
	data, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("error building chat request: %w", err)
	}

	url := fmt.Sprintf(
		"https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s",
		model, apiKey,
	)

	const maxAttempts = 3
	var lastErr error

	for attempt := range maxAttempts {
		if attempt > 0 {
			waitTime := time.Duration(1<<attempt) * time.Second
			logger.Warn("gemini: continueChat reintento", "attempt", attempt, "waitSeconds", waitTime.Seconds())
			time.Sleep(waitTime)
		}

		resp, err := http.Post(url, "application/json", bytes.NewReader(data)) //nolint:noctx
		if err != nil {
			logger.Error("gemini: continueChat HTTP failed", "err", err, "attempt", attempt+1)
			lastErr = fmt.Errorf("error calling Gemini API: %w", err)
			continue
		}

		statusCode := resp.StatusCode
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = fmt.Errorf("error reading response: %w", err)
			continue
		}

		var gemResp geminiResponse
		if err := json.Unmarshal(body, &gemResp); err != nil {
			return "", fmt.Errorf("error parsing response: %w", err)
		}

		if gemResp.Error != nil {
			logger.Error("gemini: continueChat API error", "code", gemResp.Error.Code, "message", gemResp.Error.Message, "attempt", attempt+1)
			lastErr = fmt.Errorf("Gemini API error %d: %s", gemResp.Error.Code, gemResp.Error.Message)
			if !isRetryable(gemResp.Error.Code) {
				return "", lastErr
			}
			continue
		}

		if isRetryable(statusCode) {
			lastErr = fmt.Errorf("Gemini API HTTP error %d", statusCode)
			continue
		}

		if len(gemResp.Candidates) == 0 || len(gemResp.Candidates[0].Content.Parts) == 0 {
			logger.Warn("gemini: continueChat empty response", "elapsed", time.Since(start).String())
			return "", fmt.Errorf("respuesta vacía de Gemini")
		}

		result := gemResp.Candidates[0].Content.Parts[0].Text
		elapsed := time.Since(start)
		logger.Info("gemini: continueChat ok", "model", model, "elapsed", elapsed.String(), "chars", len(result))
		return result, nil
	}

	return "", lastErr
}
