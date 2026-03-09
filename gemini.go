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

// callGeminiAudio is the shared implementation that sends audio + a text prompt to Gemini.
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

	resp, err := http.Post(url, "application/json", bytes.NewReader(data)) //nolint:noctx
	if err != nil {
		logger.Error("gemini: HTTP request failed", "err", err, "elapsed", time.Since(start).String())
		return "", fmt.Errorf("error calling Gemini API: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("error reading response: %w", err)
	}

	var gemResp geminiResponse
	if err := json.Unmarshal(body, &gemResp); err != nil {
		return "", fmt.Errorf("error parsing response: %w", err)
	}

	if gemResp.Error != nil {
		logger.Error("gemini: API error", "code", gemResp.Error.Code, "message", gemResp.Error.Message, "elapsed", time.Since(start).String())
		return "", fmt.Errorf("Gemini API error %d: %s", gemResp.Error.Code, gemResp.Error.Message)
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

// transcribeAudio sends base64-encoded audio to Gemini API using the provided prompt.
func transcribeAudio(base64Audio, mimeType, apiKey, model, prompt string) (string, error) {
	return callGeminiAudio(base64Audio, mimeType, apiKey, model, prompt)
}

// askQuestion sends base64-encoded audio to Gemini using the built-in ask prompt.
func askQuestion(base64Audio, mimeType, apiKey, model string) (string, error) {
	return callGeminiAudio(base64Audio, mimeType, apiKey, model, askPrompt)
}
