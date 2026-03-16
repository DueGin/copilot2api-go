package instance

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"sync"

	"copilot-go/anthropic"
	"copilot-go/config"
	"copilot-go/store"

	gotokenizer "github.com/tiktoken-go/tokenizer"
)

type tokenCount struct {
	Input  int
	Output int
}

type modelConstants struct {
	funcInit int
	propInit int
	propKey  int
	enumInit int
	enumItem int
	funcEnd  int
}

var codecCache sync.Map

func calculateAnthropicTokenCountOrDefault(payload anthropic.AnthropicMessagesPayload, anthropicBeta string, models *config.ModelsResponse) int {
	model := findModelEntry(models, payload.Model)
	if model == nil {
		return 1
	}
	openAIPayload := anthropic.TranslateToOpenAI(payload)
	count, err := getTokenCount(openAIPayload, *model)
	if err != nil {
		return 1
	}
	return applyAnthropicTokenAdjustments(count.Input+count.Output, payload, anthropicBeta)
}

func applyAnthropicTokenAdjustments(total int, payload anthropic.AnthropicMessagesPayload, anthropicBeta string) int {
	adjusted := total
	if len(payload.Tools) > 0 {
		mcpToolExists := false
		if strings.HasPrefix(anthropicBeta, "claude-code") {
			for _, tool := range payload.Tools {
				if strings.HasPrefix(tool.Name, "mcp__") {
					mcpToolExists = true
					break
				}
			}
		}
		if !mcpToolExists {
			switch {
			case strings.HasPrefix(payload.Model, "claude"):
				adjusted += 346
			case strings.HasPrefix(payload.Model, "grok"):
				adjusted += 480
			}
		}
	}

	switch {
	case strings.HasPrefix(payload.Model, "claude"):
		adjusted = int(math.Round(float64(adjusted) * 1.15))
	case strings.HasPrefix(payload.Model, "grok"):
		adjusted = int(math.Round(float64(adjusted) * 1.03))
	}
	return adjusted
}

func findModelEntry(models *config.ModelsResponse, modelID string) *config.ModelEntry {
	if models == nil {
		return nil
	}
	candidates := []string{modelID, anthropic.NormalizeAnthropicModelName(modelID), store.ToCopilotID(modelID)}
	for _, candidate := range candidates {
		for i := range models.Data {
			if models.Data[i].ID == candidate {
				return &models.Data[i]
			}
		}
	}
	return nil
}

func getTokenCount(payload anthropic.ChatCompletionsPayload, model config.ModelEntry) (tokenCount, error) {
	codec, err := getCodecForEncoding(getTokenizerFromModel(model))
	if err != nil {
		return tokenCount{}, err
	}
	constants := getModelConstants(model)

	inputMessages := make([]anthropic.OpenAIMessage, 0, len(payload.Messages))
	outputMessages := make([]anthropic.OpenAIMessage, 0, len(payload.Messages))
	for _, msg := range payload.Messages {
		if msg.Role == "assistant" {
			outputMessages = append(outputMessages, msg)
		} else {
			inputMessages = append(inputMessages, msg)
		}
	}

	inputTokens, err := calculateTokens(inputMessages, codec, constants)
	if err != nil {
		return tokenCount{}, err
	}
	if len(payload.Tools) > 0 {
		toolTokens, err := numTokensForTools(payload.Tools, codec, constants)
		if err != nil {
			return tokenCount{}, err
		}
		inputTokens += toolTokens
	}
	outputTokens, err := calculateTokens(outputMessages, codec, constants)
	if err != nil {
		return tokenCount{}, err
	}
	return tokenCount{Input: inputTokens, Output: outputTokens}, nil
}

func getTokenizerFromModel(model config.ModelEntry) string {
	if model.Capabilities.Tokenizer == "" {
		return string(gotokenizer.O200kBase)
	}
	return model.Capabilities.Tokenizer
}

func getCodecForEncoding(encoding string) (gotokenizer.Codec, error) {
	if cached, ok := codecCache.Load(encoding); ok {
		return cached.(gotokenizer.Codec), nil
	}
	codec, err := gotokenizer.Get(gotokenizer.Encoding(encoding))
	if err != nil {
		codec, err = gotokenizer.Get(gotokenizer.O200kBase)
		if err != nil {
			return nil, err
		}
	}
	codecCache.Store(encoding, codec)
	return codec, nil
}

func getModelConstants(model config.ModelEntry) modelConstants {
	if model.ID == "gpt-3.5-turbo" || model.ID == "gpt-4" {
		return modelConstants{funcInit: 10, propInit: 3, propKey: 3, enumInit: -3, enumItem: 3, funcEnd: 12}
	}
	return modelConstants{funcInit: 7, propInit: 3, propKey: 3, enumInit: -3, enumItem: 3, funcEnd: 12}
}

func calculateTokens(messages []anthropic.OpenAIMessage, codec gotokenizer.Codec, constants modelConstants) (int, error) {
	if len(messages) == 0 {
		return 0, nil
	}
	numTokens := 0
	for _, message := range messages {
		messageTokens, err := calculateMessageTokens(message, codec, constants)
		if err != nil {
			return 0, err
		}
		numTokens += messageTokens
	}
	numTokens += 3
	return numTokens, nil
}

func calculateMessageTokens(message anthropic.OpenAIMessage, codec gotokenizer.Codec, constants modelConstants) (int, error) {
	const tokensPerMessage = 3
	const tokensPerName = 1

	tokens := tokensPerMessage
	for _, value := range []string{message.Role, message.Name, message.ToolCallID} {
		if value == "" {
			continue
		}
		count, err := codec.Count(value)
		if err != nil {
			return 0, err
		}
		tokens += count
	}
	if message.Name != "" {
		tokens += tokensPerName
	}
	if content, ok := message.Content.(string); ok {
		count, err := codec.Count(content)
		if err != nil {
			return 0, err
		}
		tokens += count
	}
	if len(message.ToolCalls) > 0 {
		toolCallTokens, err := calculateToolCallsTokens(message.ToolCalls, codec, constants)
		if err != nil {
			return 0, err
		}
		tokens += toolCallTokens
	}
	if contentParts, ok := asContentParts(message.Content); ok {
		contentTokens, err := calculateContentPartsTokens(contentParts, codec)
		if err != nil {
			return 0, err
		}
		tokens += contentTokens
	}
	return tokens, nil
}

func asContentParts(content interface{}) ([]anthropic.OpenAIContentPart, bool) {
	switch v := content.(type) {
	case []anthropic.OpenAIContentPart:
		return v, true
	case []interface{}:
		data, err := json.Marshal(v)
		if err != nil {
			return nil, false
		}
		var parts []anthropic.OpenAIContentPart
		if err := json.Unmarshal(data, &parts); err != nil {
			return nil, false
		}
		return parts, true
	default:
		return nil, false
	}
}

func calculateContentPartsTokens(contentParts []anthropic.OpenAIContentPart, codec gotokenizer.Codec) (int, error) {
	tokens := 0
	for _, part := range contentParts {
		switch part.Type {
		case "image_url":
			if part.ImageURL == nil {
				continue
			}
			count, err := codec.Count(part.ImageURL.URL)
			if err != nil {
				return 0, err
			}
			tokens += count + 85
		case "text":
			count, err := codec.Count(part.Text)
			if err != nil {
				return 0, err
			}
			tokens += count
		}
	}
	return tokens, nil
}

func calculateToolCallsTokens(toolCalls []anthropic.ToolCall, codec gotokenizer.Codec, constants modelConstants) (int, error) {
	tokens := 0
	for _, toolCall := range toolCalls {
		data, err := json.Marshal(toolCall)
		if err != nil {
			return 0, err
		}
		count, err := codec.Count(string(data))
		if err != nil {
			return 0, err
		}
		tokens += constants.funcInit + count
	}
	tokens += constants.funcEnd
	return tokens, nil
}

func numTokensForTools(tools []anthropic.OpenAITool, codec gotokenizer.Codec, constants modelConstants) (int, error) {
	funcTokenCount := 0
	for _, tool := range tools {
		tokens, err := calculateToolTokens(tool, codec, constants)
		if err != nil {
			return 0, err
		}
		funcTokenCount += tokens
	}
	funcTokenCount += constants.funcEnd
	return funcTokenCount, nil
}

func calculateToolTokens(tool anthropic.OpenAITool, codec gotokenizer.Codec, constants modelConstants) (int, error) {
	tokens := constants.funcInit
	function := tool.Function
	fDesc := strings.TrimSuffix(function.Description, ".")
	line := function.Name + ":" + fDesc
	count, err := codec.Count(line)
	if err != nil {
		return 0, err
	}
	tokens += count
	if function.Parameters != nil {
		parameterTokens, err := calculateParametersTokens(function.Parameters, codec, constants)
		if err != nil {
			return 0, err
		}
		tokens += parameterTokens
	}
	return tokens, nil
}

func calculateParametersTokens(parameters interface{}, codec gotokenizer.Codec, constants modelConstants) (int, error) {
	params, ok := asMap(parameters)
	if !ok || len(params) == 0 {
		return 0, nil
	}
	tokens := 0
	for key, value := range params {
		if key == "properties" {
			properties, ok := asMap(value)
			if !ok || len(properties) == 0 {
				continue
			}
			tokens += constants.propInit
			for propKey, propValue := range properties {
				parameterTokens, err := calculateParameterTokens(propKey, propValue, codec, constants)
				if err != nil {
					return 0, err
				}
				tokens += parameterTokens
			}
			continue
		}
		count, err := codec.Count(fmt.Sprintf("%s:%s", key, stringifyValue(value)))
		if err != nil {
			return 0, err
		}
		tokens += count
	}
	return tokens, nil
}

func calculateParameterTokens(key string, prop interface{}, codec gotokenizer.Codec, constants modelConstants) (int, error) {
	tokens := constants.propKey
	param, ok := asMap(prop)
	if !ok {
		return tokens, nil
	}
	paramType, _ := param["type"].(string)
	if paramType == "" {
		paramType = "string"
	}
	paramDesc, _ := param["description"].(string)
	paramDesc = strings.TrimSuffix(paramDesc, ".")

	if enumValues, ok := param["enum"].([]interface{}); ok {
		tokens += constants.enumInit
		for _, item := range enumValues {
			count, err := codec.Count(fmt.Sprintf("%v", item))
			if err != nil {
				return 0, err
			}
			tokens += constants.enumItem + count
		}
	}

	line := fmt.Sprintf("%s:%s:%s", key, paramType, paramDesc)
	count, err := codec.Count(line)
	if err != nil {
		return 0, err
	}
	tokens += count

	for propertyName, propertyValue := range param {
		if propertyName == "type" || propertyName == "description" || propertyName == "enum" {
			continue
		}
		count, err := codec.Count(fmt.Sprintf("%s:%s", propertyName, stringifyValue(propertyValue)))
		if err != nil {
			return 0, err
		}
		tokens += count
	}
	return tokens, nil
}

func asMap(value interface{}) (map[string]interface{}, bool) {
	switch v := value.(type) {
	case map[string]interface{}:
		return v, true
	default:
		data, err := json.Marshal(v)
		if err != nil {
			return nil, false
		}
		var result map[string]interface{}
		if err := json.Unmarshal(data, &result); err != nil {
			return nil, false
		}
		return result, true
	}
}

func stringifyValue(value interface{}) string {
	if s, ok := value.(string); ok {
		return s
	}
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Sprintf("%v", value)
	}
	return string(data)
}
