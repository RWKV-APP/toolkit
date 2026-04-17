package llm_toolkit

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

func LLMToolkitGetHardwareInfoJSON(input string) string {
	info, err := GetHardwareInfo()
	if err != nil {
		return marshalErrorOnly(err)
	}
	return info.JSONString()
}

func LLMToolkitGetHardwareUsageInfoJSON(input string) string {
	return writeJSON(func() (any, error) {
		info, err := GetHardwareUsageInfo()
		if err != nil {
			return nil, err
		}
		var payload any
		if err := json.Unmarshal([]byte(info.JSONString()), &payload); err != nil {
			return nil, err
		}
		return payload, nil
	})
}

func LLMToolkitInitProviderJSON(input string) string {
	return writeJSON(func() (any, error) {
		var req initProviderRequest
		if err := decodeInput(input, &req); err != nil {
			return nil, err
		}
		if strings.TrimSpace(req.ProviderID) == "" {
			return nil, fmt.Errorf("provider_id is required")
		}
		if strings.TrimSpace(req.APIKey) == "" {
			return nil, fmt.Errorf("api_key is required")
		}
		baseURL, providerType, err := normalizeProvider(req.ProviderID, req.BaseURL)
		if err != nil {
			return nil, err
		}

		item := &provider{
			ID:         req.ProviderID,
			Type:       providerType,
			BaseURL:    baseURL,
			APIKey:     req.APIKey,
			DefaultMod: strings.TrimSpace(req.DefaultModel),
		}

		globalManager.providersMu.Lock()
		globalManager.providers[item.ID] = item
		globalManager.providersMu.Unlock()

		return item, nil
	})
}

func LLMToolkitListModelsJSON(input string) string {
	return writeJSON(func() (any, error) {
		var req listModelsRequest
		if err := decodeInput(input, &req); err != nil {
			return nil, err
		}
		p, err := globalManager.getProvider(req.ProviderID)
		if err != nil {
			return nil, err
		}
		models, err := fetchOpenAIModels(p)
		if err != nil {
			return nil, err
		}
		return map[string]any{
			"provider_id": p.ID,
			"object":      "list",
			"data":        models,
		}, nil
	})
}

func LLMToolkitSubmitChatJSON(input string) string {
	return writeJSON(func() (any, error) {
		var req submitRequest
		if err := decodeInput(input, &req); err != nil {
			return nil, err
		}
		if strings.TrimSpace(req.ProviderID) == "" {
			return nil, fmt.Errorf("provider_id is required")
		}
		p, err := globalManager.getProvider(req.ProviderID)
		if err != nil {
			return nil, err
		}

		requestID := globalManager.nextRequestID()
		now := time.Now().UTC()
		state := &requestState{
			RequestID:  requestID,
			ProviderID: p.ID,
			Status:     "queued",
			CreatedAt:  now,
		}

		globalManager.requestsMu.Lock()
		globalManager.requests[requestID] = state
		globalManager.requestsMu.Unlock()

		globalManager.pushEvent(event{
			Type:       "request.queued",
			RequestID:  requestID,
			ProviderID: p.ID,
			Status:     "queued",
			Timestamp:  now,
		})

		go globalManager.runChatRequest(requestID, p, req)

		return map[string]any{
			"request_id":  requestID,
			"provider_id": p.ID,
			"status":      "queued",
		}, nil
	})
}

func LLMToolkitPollEventsJSON(input string) string {
	return writeJSON(func() (any, error) {
		var req pollEventsRequest
		if err := decodeInput(input, &req); err != nil {
			return nil, err
		}
		maxEvents := req.MaxEvents
		if maxEvents <= 0 {
			maxEvents = 64
		}
		events := globalManager.pollEvents(maxEvents)
		return map[string]any{
			"events": events,
		}, nil
	})
}

func LLMToolkitGetRequestStateJSON(input string) string {
	return writeJSON(func() (any, error) {
		var req requestStateQuery
		if err := decodeInput(input, &req); err != nil {
			return nil, err
		}
		globalManager.requestsMu.RLock()
		state, ok := globalManager.requests[req.RequestID]
		globalManager.requestsMu.RUnlock()
		if !ok {
			return nil, fmt.Errorf("request_id not found: %s", req.RequestID)
		}
		copied := *state
		return copied, nil
	})
}

func marshalErrorOnly(err error) string {
	resp := map[string]any{
		"ok":    false,
		"error": err.Error(),
	}
	bytes, marshalErr := json.Marshal(resp)
	if marshalErr != nil {
		return fmt.Sprintf(`{"ok":false,"error":%q}`, marshalErr.Error())
	}
	return string(bytes)
}
