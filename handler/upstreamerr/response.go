package upstreamerr

import "encoding/json"

// ResponseFormat identifies the error envelope expected by the downstream client.
type ResponseFormat int

const (
	// FormatOpenAI produces: {"error":{"message":"...","type":"...","param":null,"code":null}}
	FormatOpenAI ResponseFormat = iota
	// FormatAnthropic produces: {"type":"error","error":{"type":"...","message":"..."}}
	FormatAnthropic
)

// ---- OpenAI error envelope ----

type openAIErrorResponse struct {
	Error openAIErrorDetail `json:"error"`
}

type openAIErrorDetail struct {
	Message string  `json:"message"`
	Type    string  `json:"type"`
	Param   *string `json:"param"`
	Code    *string `json:"code"`
}

// ---- Anthropic error envelope ----

type anthropicErrorResponse struct {
	Type  string               `json:"type"`
	Error anthropicErrorDetail `json:"error"`
}

type anthropicErrorDetail struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// BuildErrorBody serialises a formatted error response in the requested format.
func BuildErrorBody(ue UpstreamError, format ResponseFormat) []byte {
	var payload any

	switch format {
	case FormatAnthropic:
		payload = anthropicErrorResponse{
			Type: "error",
			Error: anthropicErrorDetail{
				Type:    ue.Type,
				Message: ue.Message,
			},
		}
	default: // FormatOpenAI
		payload = openAIErrorResponse{
			Error: openAIErrorDetail{
				Message: ue.Message,
				Type:    ue.Type,
			},
		}
	}

	body, err := json.Marshal(payload)
	if err != nil {
		// Fallback: should never happen, but be defensive.
		if format == FormatAnthropic {
			return []byte(`{"type":"error","error":{"type":"upstream_error","message":"internal error"}}`)
		}
		return []byte(`{"error":{"message":"internal error","type":"upstream_error","param":null,"code":null}}`)
	}
	return body
}

// BuildSSEErrorData returns the JSON payload for an Anthropic SSE error event.
//
//	event: error
//	data: <this value>
func BuildSSEErrorData(ue UpstreamError) []byte {
	data := anthropicErrorResponse{
		Type: "error",
		Error: anthropicErrorDetail{
			Type:    ue.Type,
			Message: ue.Message,
		},
	}
	body, _ := json.Marshal(data)
	return body
}
