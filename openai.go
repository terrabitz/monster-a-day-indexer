package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type OpenAIClient struct {
	http.Client
	Token string
}

func NewOpenAIClient(token string) *OpenAIClient {
	return &OpenAIClient{Token: token}
}

func doRequest[T any](ctx context.Context, client *OpenAIClient, method string, url string, body any) (T, error) {
	bodyBytes, _ := json.Marshal(body)
	req, _ := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(bodyBytes))
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", client.Token))

	var resObject T

	res, err := client.Do(req)
	if err != nil {
		return resObject, fmt.Errorf("error doing HTTP request: %w", err)
	}
	defer res.Body.Close()

	raw, _ := io.ReadAll(res.Body)
	fmt.Println(string(raw))

	if err := json.Unmarshal(raw, &resObject); err != nil {
		return resObject, fmt.Errorf("error parsing response body: %w", err)
	}

	return resObject, nil
}

func (c *OpenAIClient) GetChatCompletion(ctx context.Context, req GetChatCompletionRequest) (GetChatCompletionResponse, error) {
	ret, err := doRequest[GetChatCompletionResponse](ctx, c, http.MethodPost, "https://api.openai.com/v1/chat/completions", req)
	if err != nil {
		return GetChatCompletionResponse{}, fmt.Errorf("error making API request: %w", err)
	}

	if ret.Error != nil {
		return GetChatCompletionResponse{}, fmt.Errorf("error making API request: %s", ret.Error.Message)
	}

	return ret, nil
}

type GetChatCompletionRequest struct {
	Model     string                            `json:"model"`
	Messages  []GetChatCompletionRequestMessage `json:"messages"`
	MaxTokens int                               `json:"max_tokens"`
}

type GetChatCompletionRequestMessage struct {
	Role    string                         `json:"role"`
	Content []GetChatCompletionRequestPart `json:"content"`
}

type GetChatCompletionRequestPart struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	ImageURL struct {
		URL string `json:"url"`
	} `json:"image_url,omitempty"`
}

type GetChatCompletionResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int    `json:"created"`
	Model   string `json:"model"`
	Usage   struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
	Choices []struct {
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
		FinishDetails struct {
			Type string `json:"type"`
			Stop string `json:"stop"`
		} `json:"finish_details"`
		Index int `json:"index"`
	} `json:"choices"`
	Error *struct {
		Message string      `json:"message"`
		Type    string      `json:"type"`
		Param   interface{} `json:"param"`
		Code    interface{} `json:"code"`
	} `json:"error,omitempty"`
}
