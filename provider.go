package llm_toolkit

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/openai"
)

type provider struct {
	ID         string `json:"id"`
	Type       string `json:"type"`
	BaseURL    string `json:"base_url"`
	APIKey     string `json:"-"`
	DefaultMod string `json:"default_model,omitempty"`
}

type requestState struct {
	RequestID     string                 `json:"request_id"`
	ProviderID    string                 `json:"provider_id"`
	Status        string                 `json:"status"`
	CreatedAt     time.Time              `json:"created_at"`
	StartedAt     *time.Time             `json:"started_at,omitempty"`
	FinishedAt    *time.Time             `json:"finished_at,omitempty"`
	Model         string                 `json:"model,omitempty"`
	OutputText    string                 `json:"output_text,omitempty"`
	ReasoningText string                 `json:"reasoning_text,omitempty"`
	StopReason    string                 `json:"stop_reason,omitempty"`
	ErrorMessage  string                 `json:"error_message,omitempty"`
	Response      *openAIResponsePayload `json:"response,omitempty"`
}

type event struct {
	Sequence       int64                  `json:"sequence"`
	Type           string                 `json:"type"`
	RequestID      string                 `json:"request_id,omitempty"`
	ProviderID     string                 `json:"provider_id,omitempty"`
	Status         string                 `json:"status,omitempty"`
	Delta          string                 `json:"delta,omitempty"`
	ReasoningDelta string                 `json:"reasoning_delta,omitempty"`
	Error          string                 `json:"error,omitempty"`
	Response       *openAIResponsePayload `json:"response,omitempty"`
	Timestamp      time.Time              `json:"timestamp"`
}

type manager struct {
	providersMu sync.RWMutex
	providers   map[string]*provider

	requestsMu sync.RWMutex
	requests   map[string]*requestState

	eventsMu sync.Mutex
	events   []event

	requestSeq atomic.Uint64
	eventSeq   atomic.Int64
}

type initProviderRequest struct {
	ProviderID   string `json:"provider_id"`
	BaseURL      string `json:"base_url"`
	APIKey       string `json:"api_key"`
	DefaultModel string `json:"default_model,omitempty"`
}

type listModelsRequest struct {
	ProviderID string `json:"provider_id"`
}

type submitRequest struct {
	ProviderID string                  `json:"provider_id"`
	Model      string                  `json:"model,omitempty"`
	Stream     *bool                   `json:"stream,omitempty"`
	Metadata   map[string]any          `json:"metadata,omitempty"`
	Request    openAIChatRequest       `json:"request"`
	Options    *openAIChatRequestHints `json:"options,omitempty"`
}

type openAIChatRequest struct {
	Model            string          `json:"model,omitempty"`
	Messages         []openAIMessage `json:"messages,omitempty"`
	MaxTokens        *int            `json:"max_tokens,omitempty"`
	Temperature      *float64        `json:"temperature,omitempty"`
	TopP             *float64        `json:"top_p,omitempty"`
	N                *int            `json:"n,omitempty"`
	Stop             openAIStop      `json:"stop,omitempty"`
	PresencePenalty  *float64        `json:"presence_penalty,omitempty"`
	FrequencyPenalty *float64        `json:"frequency_penalty,omitempty"`
	Seed             *int            `json:"seed,omitempty"`
	Tools            []openAITool    `json:"tools,omitempty"`
	ToolChoice       any             `json:"tool_choice,omitempty"`
	ResponseFormat   map[string]any  `json:"response_format,omitempty"`
	ExtraBody        map[string]any  `json:"extra_body,omitempty"`
}

type openAIChatRequestHints struct {
	UseLegacyMaxTokens bool `json:"use_legacy_max_tokens,omitempty"`
}

type openAIStop []string

func (s *openAIStop) UnmarshalJSON(data []byte) error {
	if string(data) == "null" || len(data) == 0 {
		return nil
	}
	var single string
	if err := json.Unmarshal(data, &single); err == nil {
		*s = []string{single}
		return nil
	}
	var many []string
	if err := json.Unmarshal(data, &many); err != nil {
		return fmt.Errorf("stop must be string or string array: %w", err)
	}
	*s = many
	return nil
}

type openAIMessage struct {
	Role       string                 `json:"role"`
	Content    any                    `json:"content,omitempty"`
	Name       string                 `json:"name,omitempty"`
	ToolCalls  []openAIToolCall       `json:"tool_calls,omitempty"`
	ToolCallID string                 `json:"tool_call_id,omitempty"`
	Metadata   map[string]any         `json:"metadata,omitempty"`
	ExtraBody  map[string]interface{} `json:"extra_body,omitempty"`
}

type openAIContentPart struct {
	Type     string              `json:"type"`
	Text     string              `json:"text,omitempty"`
	ImageURL *openAIImageURLPart `json:"image_url,omitempty"`
}

type openAIImageURLPart struct {
	URL    string `json:"url"`
	Detail string `json:"detail,omitempty"`
}

type openAITool struct {
	Type     string                   `json:"type"`
	Function *llms.FunctionDefinition `json:"function,omitempty"`
}

type openAIToolCall struct {
	ID       string             `json:"id"`
	Type     string             `json:"type"`
	Function openAIFunctionCall `json:"function"`
}

type openAIFunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type openAIResponsePayload struct {
	ID      string         `json:"id,omitempty"`
	Object  string         `json:"object"`
	Created int64          `json:"created"`
	Model   string         `json:"model,omitempty"`
	Choices []openAIChoice `json:"choices"`
	Usage   map[string]any `json:"usage,omitempty"`
}

type openAIChoice struct {
	Index        int                   `json:"index"`
	Message      openAIResponseMessage `json:"message"`
	FinishReason string                `json:"finish_reason,omitempty"`
}

type openAIResponseMessage struct {
	Role             string           `json:"role"`
	Content          string           `json:"content,omitempty"`
	ReasoningContent string           `json:"reasoning_content,omitempty"`
	ToolCalls        []openAIToolCall `json:"tool_calls,omitempty"`
}

type listModelsResponse struct {
	Object string          `json:"object"`
	Data   []openAIModelID `json:"data"`
}

type openAIModelID struct {
	ID string `json:"id"`
}

type pollEventsRequest struct {
	MaxEvents int `json:"max_events,omitempty"`
}

type requestStateQuery struct {
	RequestID string `json:"request_id"`
}

var globalManager = newManager()

func newManager() *manager {
	return &manager{
		providers: make(map[string]*provider),
		requests:  make(map[string]*requestState),
		events:    make([]event, 0, 64),
	}
}

func (m *manager) runChatRequest(requestID string, p *provider, req submitRequest) {
	startedAt := time.Now().UTC()
	m.updateState(requestID, func(state *requestState) {
		state.Status = "running"
		state.StartedAt = &startedAt
	})
	m.pushEvent(event{
		Type:       "request.started",
		RequestID:  requestID,
		ProviderID: p.ID,
		Status:     "running",
		Timestamp:  startedAt,
	})

	modelName := strings.TrimSpace(req.Model)
	if modelName == "" {
		modelName = strings.TrimSpace(req.Request.Model)
	}
	if modelName == "" {
		modelName = strings.TrimSpace(p.DefaultMod)
	}
	if modelName == "" {
		m.failRequest(requestID, p.ID, fmt.Errorf("model is required"))
		return
	}
	m.updateState(requestID, func(state *requestState) {
		state.Model = modelName
	})

	llmClient, err := openai.New(
		openai.WithToken(p.APIKey),
		openai.WithBaseURL(p.BaseURL),
		openai.WithModel(modelName),
	)
	if err != nil {
		m.failRequest(requestID, p.ID, err)
		return
	}

	messages, err := convertMessages(req.Request.Messages)
	if err != nil {
		m.failRequest(requestID, p.ID, err)
		return
	}

	callOptions, err := buildCallOptions(req, modelName, requestID)
	if err != nil {
		m.failRequest(requestID, p.ID, err)
		return
	}

	resp, err := llmClient.GenerateContent(context.Background(), messages, callOptions...)
	if err != nil {
		m.failRequest(requestID, p.ID, err)
		return
	}

	payload := buildResponsePayload(requestID, modelName, resp)
	finishedAt := time.Now().UTC()
	output := ""
	reasoning := ""
	stopReason := ""
	if len(payload.Choices) > 0 {
		output = payload.Choices[0].Message.Content
		reasoning = payload.Choices[0].Message.ReasoningContent
		stopReason = payload.Choices[0].FinishReason
	}

	m.updateState(requestID, func(state *requestState) {
		state.Status = "completed"
		state.FinishedAt = &finishedAt
		state.OutputText = output
		state.ReasoningText = reasoning
		state.StopReason = stopReason
		state.Response = payload
	})
	m.pushEvent(event{
		Type:       "request.completed",
		RequestID:  requestID,
		ProviderID: p.ID,
		Status:     "completed",
		Response:   payload,
		Timestamp:  finishedAt,
	})
}

func buildCallOptions(req submitRequest, modelName string, requestID string) ([]llms.CallOption, error) {
	opts := []llms.CallOption{
		llms.WithModel(modelName),
	}
	if req.Request.MaxTokens != nil {
		opts = append(opts, llms.WithMaxTokens(*req.Request.MaxTokens))
	}
	if req.Request.Temperature != nil {
		opts = append(opts, llms.WithTemperature(*req.Request.Temperature))
	}
	if req.Request.TopP != nil {
		opts = append(opts, llms.WithTopP(*req.Request.TopP))
	}
	if req.Request.N != nil {
		opts = append(opts, llms.WithN(*req.Request.N))
	}
	if len(req.Request.Stop) > 0 {
		opts = append(opts, llms.WithStopWords(req.Request.Stop))
	}
	if req.Request.PresencePenalty != nil {
		opts = append(opts, llms.WithPresencePenalty(*req.Request.PresencePenalty))
	}
	if req.Request.FrequencyPenalty != nil {
		opts = append(opts, llms.WithFrequencyPenalty(*req.Request.FrequencyPenalty))
	}
	if req.Request.Seed != nil {
		opts = append(opts, llms.WithSeed(*req.Request.Seed))
	}
	if len(req.Request.Tools) > 0 {
		tools := make([]llms.Tool, 0, len(req.Request.Tools))
		for _, t := range req.Request.Tools {
			tools = append(tools, llms.Tool{
				Type:     t.Type,
				Function: t.Function,
			})
		}
		opts = append(opts, llms.WithTools(tools))
	}
	if req.Request.ToolChoice != nil {
		opts = append(opts, llms.WithToolChoice(req.Request.ToolChoice))
	}
	if req.Request.ResponseFormat != nil {
		if kind, _ := req.Request.ResponseFormat["type"].(string); kind == "json_object" {
			opts = append(opts, llms.WithJSONMode())
		}
	}

	metadata := make(map[string]any, len(req.Metadata)+len(req.Request.ExtraBody)+2)
	for k, v := range req.Metadata {
		metadata[k] = v
	}
	for k, v := range req.Request.ExtraBody {
		metadata[k] = v
	}
	metadata["request_id"] = requestID
	if len(metadata) > 0 {
		opts = append(opts, llms.WithMetadata(metadata))
	}

	if req.Options != nil && req.Options.UseLegacyMaxTokens {
		opts = append(opts, openai.WithLegacyMaxTokensField())
	}

	stream := true
	if req.Stream != nil {
		stream = *req.Stream
	}
	if stream {
		opts = append(opts, llms.WithStreamingReasoningFunc(func(ctx context.Context, reasoningChunk []byte, chunk []byte) error {
			now := time.Now().UTC()

			if len(reasoningChunk) > 0 {
				reasoningText := string(reasoningChunk)
				globalManager.updateState(requestID, func(state *requestState) {
					state.ReasoningText += reasoningText
				})
				globalManager.pushEvent(event{
					Type:           "response.reasoning_delta",
					RequestID:      requestID,
					Status:         "running",
					ReasoningDelta: reasoningText,
					Timestamp:      now,
				})
			}

			if len(chunk) > 0 {
				text := string(chunk)
				globalManager.updateState(requestID, func(state *requestState) {
					state.OutputText += text
				})
				globalManager.pushEvent(event{
					Type:      "response.delta",
					RequestID: requestID,
					Status:    "running",
					Delta:     text,
					Timestamp: now,
				})
			}

			return nil
		}))
	}

	return opts, nil
}

func buildResponsePayload(requestID string, modelName string, resp *llms.ContentResponse) *openAIResponsePayload {
	payload := &openAIResponsePayload{
		ID:      requestID,
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   modelName,
		Choices: make([]openAIChoice, 0, len(resp.Choices)),
	}

	for i, choice := range resp.Choices {
		message := openAIResponseMessage{
			Role:             "assistant",
			Content:          choice.Content,
			ReasoningContent: choice.ReasoningContent,
		}
		if len(choice.ToolCalls) > 0 {
			message.ToolCalls = make([]openAIToolCall, 0, len(choice.ToolCalls))
			for _, call := range choice.ToolCalls {
				message.ToolCalls = append(message.ToolCalls, openAIToolCall{
					ID:   call.ID,
					Type: call.Type,
					Function: openAIFunctionCall{
						Name:      call.FunctionCall.Name,
						Arguments: call.FunctionCall.Arguments,
					},
				})
			}
		}
		payload.Choices = append(payload.Choices, openAIChoice{
			Index:        i,
			Message:      message,
			FinishReason: choice.StopReason,
		})
		if choice.GenerationInfo != nil {
			payload.Usage = choice.GenerationInfo
		}
	}

	return payload
}

func convertMessages(in []openAIMessage) ([]llms.MessageContent, error) {
	result := make([]llms.MessageContent, 0, len(in))
	for _, msg := range in {
		role, err := toLangChainRole(msg.Role)
		if err != nil {
			return nil, err
		}
		parts, err := convertParts(msg)
		if err != nil {
			return nil, err
		}
		result = append(result, llms.MessageContent{
			Role:  role,
			Parts: parts,
		})
	}
	return result, nil
}

func convertParts(msg openAIMessage) ([]llms.ContentPart, error) {
	switch msg.Role {
	case "assistant":
		parts, err := decodeContentParts(msg.Content)
		if err != nil {
			return nil, err
		}
		for _, call := range msg.ToolCalls {
			fn := call.Function
			parts = append(parts, llms.ToolCall{
				ID:   call.ID,
				Type: call.Type,
				FunctionCall: &llms.FunctionCall{
					Name:      fn.Name,
					Arguments: fn.Arguments,
				},
			})
		}
		return parts, nil
	case "tool":
		text, err := coerceTextContent(msg.Content)
		if err != nil {
			return nil, err
		}
		return []llms.ContentPart{llms.ToolCallResponse{
			ToolCallID: msg.ToolCallID,
			Name:       msg.Name,
			Content:    text,
		}}, nil
	case "function":
		text, err := coerceTextContent(msg.Content)
		if err != nil {
			return nil, err
		}
		return []llms.ContentPart{llms.ToolCallResponse{
			Name:    msg.Name,
			Content: text,
		}}, nil
	default:
		return decodeContentParts(msg.Content)
	}
}

func decodeContentParts(raw any) ([]llms.ContentPart, error) {
	if raw == nil {
		return nil, nil
	}
	switch v := raw.(type) {
	case string:
		return []llms.ContentPart{llms.TextPart(v)}, nil
	case []any:
		parts := make([]llms.ContentPart, 0, len(v))
		for _, item := range v {
			partMap, ok := item.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("invalid content part")
			}
			partType, _ := partMap["type"].(string)
			switch partType {
			case "text":
				text, _ := partMap["text"].(string)
				parts = append(parts, llms.TextPart(text))
			case "image_url":
				imageRaw, ok := partMap["image_url"].(map[string]any)
				if !ok {
					return nil, fmt.Errorf("invalid image_url part")
				}
				imageURL, _ := imageRaw["url"].(string)
				detail, _ := imageRaw["detail"].(string)
				parts = append(parts, llms.ImageURLWithDetailPart(imageURL, detail))
			default:
				return nil, fmt.Errorf("unsupported content part type: %s", partType)
			}
		}
		return parts, nil
	default:
		return nil, fmt.Errorf("unsupported message content type")
	}
}

func coerceTextContent(raw any) (string, error) {
	switch v := raw.(type) {
	case nil:
		return "", nil
	case string:
		return v, nil
	default:
		return "", fmt.Errorf("content must be string")
	}
}

func toLangChainRole(role string) (llms.ChatMessageType, error) {
	switch strings.TrimSpace(role) {
	case "system":
		return llms.ChatMessageTypeSystem, nil
	case "user":
		return llms.ChatMessageTypeHuman, nil
	case "assistant":
		return llms.ChatMessageTypeAI, nil
	case "tool":
		return llms.ChatMessageTypeTool, nil
	case "function":
		return llms.ChatMessageTypeFunction, nil
	default:
		return "", fmt.Errorf("unsupported role: %s", role)
	}
}

func fetchOpenAIModels(p *provider) ([]openAIModelID, error) {
	endpoint, err := joinURLPath(p.BaseURL, "models")
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+p.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 16*1024))
		return nil, fmt.Errorf("list models failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var payload listModelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}
	return payload.Data, nil
}

func normalizeProvider(providerID string, rawBaseURL string) (string, string, error) {
	providerType := detectProviderType(providerID, rawBaseURL)
	if providerType != "openai" {
		return "", "", fmt.Errorf("unsupported provider type: %s", providerType)
	}
	baseURL := strings.TrimSpace(rawBaseURL)
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return "", "", fmt.Errorf("invalid base_url: %w", err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return "", "", fmt.Errorf("base_url must be an absolute URL")
	}
	if parsed.Path == "" || parsed.Path == "/" {
		parsed.Path = "/v1"
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	return parsed.String(), providerType, nil
}

func detectProviderType(providerID string, rawBaseURL string) string {
	text := strings.ToLower(strings.TrimSpace(providerID) + " " + strings.TrimSpace(rawBaseURL))
	switch {
	case strings.Contains(text, "openai"):
		return "openai"
	case strings.Contains(text, "/v1"):
		return "openai"
	default:
		return "openai"
	}
}

func joinURLPath(baseURL string, pathPart string) (string, error) {
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + "/" + strings.TrimLeft(pathPart, "/")
	return parsed.String(), nil
}

func (m *manager) getProvider(providerID string) (*provider, error) {
	m.providersMu.RLock()
	defer m.providersMu.RUnlock()
	p, ok := m.providers[providerID]
	if !ok {
		return nil, fmt.Errorf("provider_id not found: %s", providerID)
	}
	return p, nil
}

func (m *manager) nextRequestID() string {
	n := m.requestSeq.Add(1)
	return fmt.Sprintf("req_%d_%d", time.Now().UnixNano(), n)
}

func (m *manager) pushEvent(evt event) {
	evt.Sequence = m.eventSeq.Add(1)
	if evt.Timestamp.IsZero() {
		evt.Timestamp = time.Now().UTC()
	}
	m.eventsMu.Lock()
	m.events = append(m.events, evt)
	m.eventsMu.Unlock()
}

func (m *manager) pollEvents(maxEvents int) []event {
	m.eventsMu.Lock()
	defer m.eventsMu.Unlock()
	if len(m.events) == 0 {
		return []event{}
	}
	if maxEvents > len(m.events) {
		maxEvents = len(m.events)
	}
	items := append([]event(nil), m.events[:maxEvents]...)
	m.events = append([]event(nil), m.events[maxEvents:]...)
	return items
}

func (m *manager) updateState(requestID string, fn func(*requestState)) {
	m.requestsMu.Lock()
	defer m.requestsMu.Unlock()
	state, ok := m.requests[requestID]
	if !ok {
		return
	}
	fn(state)
}

func (m *manager) failRequest(requestID string, providerID string, err error) {
	finishedAt := time.Now().UTC()
	m.updateState(requestID, func(state *requestState) {
		state.Status = "failed"
		state.FinishedAt = &finishedAt
		state.ErrorMessage = err.Error()
	})
	m.pushEvent(event{
		Type:       "request.failed",
		RequestID:  requestID,
		ProviderID: providerID,
		Status:     "failed",
		Error:      err.Error(),
		Timestamp:  finishedAt,
	})
}

func decodeInput(input string, out any) error {
	raw := "{}"
	if strings.TrimSpace(input) != "" {
		raw = input
	}
	if strings.TrimSpace(raw) == "" {
		raw = "{}"
	}
	if err := json.Unmarshal([]byte(raw), out); err != nil {
		return fmt.Errorf("invalid json input: %w", err)
	}
	return nil
}

func writeJSON(fn func() (any, error)) string {
	payload, err := fn()
	resp := map[string]any{"ok": err == nil}
	if err != nil {
		resp["error"] = err.Error()
	} else {
		resp["data"] = payload
	}
	bytes, marshalErr := json.Marshal(resp)
	if marshalErr != nil {
		return fmt.Sprintf(`{"ok":false,"error":%q}`, marshalErr.Error())
	}
	return string(bytes)
}
