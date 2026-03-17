# copilot2api-go 项目 Token 管理与上下文限制分析

## 项目概述
copilot2api-go 是一个 GitHub Copilot API 的反向代理，提供 OpenAI 和 Anthropic 兼容的 API 接口。项目主要功能包括多账户管理、负载均衡、协议转换等。

---

## 1. ModelLimits 定义与使用

### 1.1 ModelLimits 结构定义
**文件: config/config.go (第 88-93 行)**

```go
type ModelLimits struct {
    MaxContextWindowTokens int `json:"max_context_window_tokens,omitempty"`
    MaxOutputTokens        int `json:"max_output_tokens,omitempty"`
    MaxPromptTokens        int `json:"max_prompt_tokens,omitempty"`
    MaxInputs              int `json:"max_inputs,omitempty"`
}
```

- **MaxContextWindowTokens**: 模型的最大上下文窗口大小（输入+输出）
- **MaxOutputTokens**: 最大输出 token 数量
- **MaxPromptTokens**: 最大提示 token 数量
- **MaxInputs**: 最大输入数量

### 1.2 ModelLimits 使用位置
**文件: instance/request_prep.go (第 21-33 行)**

```go
func applyDefaultMaxTokens(payload *anthropic.ChatCompletionsPayload, models *config.ModelsResponse) {
    if payload == nil || payload.MaxTokens > 0 || models == nil {
        return
    }
    for _, model := range models.Data {
        if model.ID == payload.Model || model.ID == store.ToCopilotID(payload.Model) {
            if model.Capabilities.Limits.MaxOutputTokens > 0 {
                payload.MaxTokens = model.Capabilities.Limits.MaxOutputTokens
            }
            return
        }
    }
}
```

**用途**:
- 如果客户端请求中未指定 MaxTokens，则从模型配置中自动应用 MaxOutputTokens
- 仅用于设置默认的输出 token 限制
- 不检查上下文窗口大小是否超限

---

## 2. Token 计数逻辑

### 2.1 token_count.go 核心函数

#### 2.1.1 calculateAnthropicTokenCountOrDefault() - 估算请求的 token 数量
**文件: instance/token_count.go (第 33-44 行)**

```go
func calculateAnthropicTokenCountOrDefault(payload anthropic.AnthropicMessagesPayload, 
                                           anthropicBeta string, 
                                           models *config.ModelsResponse) int {
    model := findModelEntry(models, payload.Model)
    if model == nil {
        return 1  // 模型未找到，返回默认值 1
    }
    openAIPayload := anthropic.TranslateToOpenAI(payload)
    count, err := getTokenCount(openAIPayload, *model)
    if err != nil {
        return 1  // 计数出错，返回默认值 1
    }
    return applyAnthropicTokenAdjustments(count.Input+count.Output, payload, anthropicBeta)
}
```

#### 2.1.2 getTokenCount() - 详细计数逻辑
**文件: instance/token_count.go (第 92-125 行)**

```go
func getTokenCount(payload anthropic.ChatCompletionsPayload, model config.ModelEntry) (tokenCount, error) {
    codec, err := getCodecForEncoding(getTokenizerFromModel(model))
    constants := getModelConstants(model)

    // 分离输入和输出消息
    for _, msg := range payload.Messages {
        if msg.Role == "assistant" {
            outputMessages = append(outputMessages, msg)
        } else {
            inputMessages = append(inputMessages, msg)
        }
    }

    // 计算输入 token
    inputTokens, err := calculateTokens(inputMessages, codec, constants)
    
    // 如果有工具定义，计算工具 token
    if len(payload.Tools) > 0 {
        toolTokens, err := numTokensForTools(payload.Tools, codec, constants)
        inputTokens += toolTokens
    }
    
    // 计算输出 token
    outputTokens, err := calculateTokens(outputMessages, codec, constants)
    
    return tokenCount{Input: inputTokens, Output: outputTokens}, nil
}
```

#### 2.1.3 Token 调整函数 - applyAnthropicTokenAdjustments()
**文件: instance/token_count.go (第 46-75 行)**

```go
func applyAnthropicTokenAdjustments(total int, 
                                    payload anthropic.AnthropicMessagesPayload, 
                                    anthropicBeta string) int {
    adjusted := total
    
    // 1. 如果有工具定义，增加开销
    if len(payload.Tools) > 0 {
        mcpToolExists := false
        if strings.HasPrefix(anthropicBeta, "claude-code") {
            // 检查是否存在 MCP 工具（mcp__ 前缀）
            for _, tool := range payload.Tools {
                if strings.HasPrefix(tool.Name, "mcp__") {
                    mcpToolExists = true
                    break
                }
            }
        }
        if !mcpToolExists {
            // 非 MCP 工具的固定开销
            switch {
            case strings.HasPrefix(payload.Model, "claude"):
                adjusted += 346  // Claude 模型固定增加 346 token
            case strings.HasPrefix(payload.Model, "grok"):
                adjusted += 480  // Grok 模型固定增加 480 token
            }
        }
    }

    // 2. 模型特定的百分比调整
    switch {
    case strings.HasPrefix(payload.Model, "claude"):
        adjusted = int(math.Round(float64(adjusted) * 1.15))  // 增加 15%
    case strings.HasPrefix(payload.Model, "grok"):
        adjusted = int(math.Round(float64(adjusted) * 1.03))  // 增加 3%
    }
    
    return adjusted
}
```

**调整规则总结**:
1. **工具固定开销**:
   - Claude 模型: +346 tokens
   - Grok 模型: +480 tokens
   - MCP 工具(claude-code): 免除固定开销

2. **百分比调整**:
   - Claude: ×1.15 (增加15%)
   - Grok: ×1.03 (增加3%)

#### 2.1.4 Token 计算器工厂 - getTokenizerFromModel()
**文件: instance/token_count.go (第 127-132 行)**

```go
func getTokenizerFromModel(model config.ModelEntry) string {
    if model.Capabilities.Tokenizer == "" {
        return string(gotokenizer.O200kBase)  // 默认使用 o200k_base
    }
    return model.Capabilities.Tokenizer
}
```

#### 2.1.5 消息 Token 计算 - calculateMessageTokens()
**文件: instance/token_count.go (第 172-212 行)**

```go
func calculateMessageTokens(message anthropic.OpenAIMessage, 
                           codec gotokenizer.Codec, 
                           constants modelConstants) (int, error) {
    const tokensPerMessage = 3
    const tokensPerName = 1

    tokens := tokensPerMessage  // 每条消息基础 3 tokens
    
    // Role, Name, ToolCallID 各占 1 token
    for _, value := range []string{message.Role, message.Name, message.ToolCallID} {
        if value == "" {
            continue
        }
        count, err := codec.Count(value)
        tokens += count
    }
    
    if message.Name != "" {
        tokens += tokensPerName  // 名字额外 1 token
    }
    
    // 内容 token 计算
    if content, ok := message.Content.(string); ok {
        count, err := codec.Count(content)
        tokens += count
    }
    
    // 工具调用 token 计算
    if len(message.ToolCalls) > 0 {
        toolCallTokens, err := calculateToolCallsTokens(message.ToolCalls, codec, constants)
        tokens += toolCallTokens
    }
    
    // 多模态内容 (图片等) token 计算
    if contentParts, ok := asContentParts(message.Content); ok {
        contentTokens, err := calculateContentPartsTokens(contentParts, codec)
        tokens += contentTokens
    }
    
    return tokens, nil
}
```

#### 2.1.6 图片 Token 计算
**文件: instance/token_count.go (第 233-255 行)**

```go
func calculateContentPartsTokens(contentParts []anthropic.OpenAIContentPart, 
                                codec gotokenizer.Codec) (int, error) {
    tokens := 0
    for _, part := range contentParts {
        switch part.Type {
        case "image_url":
            if part.ImageURL == nil {
                continue
            }
            count, err := codec.Count(part.ImageURL.URL)
            tokens += count + 85  // 图片 URL token 数 + 85 固定开销
        case "text":
            count, err := codec.Count(part.Text)
            tokens += count
        }
    }
    return tokens, nil
}
```

---

## 3. Request 准备逻辑

### 3.1 request_prep.go 的核心功能
**文件: instance/request_prep.go**

#### 3.1.1 rewriteCompletionsPayload() - 请求重写
**第 41-53 行**:

```go
func rewriteCompletionsPayload(bodyBytes []byte, models *config.ModelsResponse) ([]byte, http.Header, bool, error) {
    var payload anthropic.ChatCompletionsPayload
    if err := json.Unmarshal(bodyBytes, &payload); err != nil {
        return bodyBytes, nil, false, nil
    }
    
    // 1. 应用默认的 MaxTokens
    applyDefaultMaxTokens(&payload, models)
    
    // 2. 转换模型 ID (DisplayID -> CopilotID)
    payload.Model = store.ToCopilotID(payload.Model)
    
    // 3. 编码并返回
    updatedBody, err := json.Marshal(payload)
    if err != nil {
        return nil, nil, false, err
    }
    
    return updatedBody, extraHeadersForMessages(payload.Messages), 
           hasOpenAIVisionContent(payload.Messages), nil
}
```

#### 3.1.2 初始化函数 - applyDefaultMaxTokens()
**第 21-33 行**: 
- 如果未指定 MaxTokens，使用模型的 MaxOutputTokens
- **重要**: 仅处理输出 token，不处理上下文窗口检查

---

## 4. 当前 Token 超限处理机制

### 4.1 **完全缺失**: 无 Token 超限的主动检查或处理

#### 现状总结:
1. **无上下文窗口检查**: 代码中完全没有检查请求的总 token 数是否超过 `MaxContextWindowTokens`
2. **无自动截断或压缩**: 不存在自动截断消息、压缩上下文、总结历史等逻辑
3. **被动错误处理**: 仅在上游返回错误时被动处理

### 4.2 错误处理流程

**文件: handler/upstreamerr/handler.go 和 error_map.go**

#### 4.2.1 错误映射规则
**文件: handler/upstreamerr/error_map.go (第 17-58 行)**

```go
var errorMap = map[int]UpstreamError{
    http.StatusBadRequest: {
        StatusCode: http.StatusBadRequest,
        Type:       "invalid_request_error",
        Message:    "Invalid request parameters",
    },
    http.StatusUnauthorized: {
        StatusCode: http.StatusUnauthorized,
        Type:       "authentication_error",
        Message:    "Authentication failed",
    },
    // ... 其他状态码 ...
}
```

**问题**: 
- 没有针对"token超限"(如 429, 413 等) 的特殊处理
- 没有检测上游返回的 "context_length_exceeded" 或类似错误消息
- 错误映射是通用的，无法区分 token 超限错误

#### 4.2.2 错误处理流程
**文件: instance/handler.go**

```go
// ForwardCompletionsResponse (第 36-88 行)
func ForwardCompletionsResponse(c *gin.Context, resp *http.Response) {
    defer func() { _ = resp.Body.Close() }()
    
    if resp.StatusCode != http.StatusOK {
        // 读取响应体
        body, _ := io.ReadAll(resp.Body)
        // 使用通用错误处理，不检查错误内容
        upstreamerr.HandleUpstreamError(c, resp.StatusCode, body, 
                                       upstreamerr.FormatOpenAI, "completions_stream")
        return
    }
    // ... 正常处理 ...
}
```

#### 4.2.3 上游错误处理
**文件: handler/upstreamerr/handler.go (第 16-28 行)**

```go
func HandleUpstreamError(c *gin.Context, upstreamStatus int, upstreamBody []byte, 
                        format ResponseFormat, endpoint string) {
    // 记录错误（有 2048 字节的截断）
    logUpstreamError(endpoint, upstreamStatus, upstreamBody, c.ClientIP(), c.Request.URL.Path)
    
    // 查找映射的错误
    ue := Lookup(upstreamStatus)
    
    // 构建响应（丢失原始上游错误信息）
    body := BuildErrorBody(ue, format)
    c.Data(ue.StatusCode, "application/json", body)
}
```

**问题**:
- 日志中截断到 2048 字节，可能丢失关键错误信息
- 使用通用错误消息，用户看不到真实的上游错误原因
- 无法区分是 token 超限还是其他错误

---

## 5. Token 计数的使用场景

### 5.1 Anthropic Messages 的 Token 计数端点
**文件: handler/proxy.go (第 341-348 行)**

```go
func proxyCountTokens(c *gin.Context) {
    resolved := resolveState(c, nil)
    if resolved == nil {
        return
    }
    instance.CountTokensHandler(c, resolved.State)
}
```

**文件: instance/handler.go (第 292-312 行)**

```go
func CountTokensHandler(c *gin.Context, state *config.State) {
    bodyBytes, err := io.ReadAll(c.Request.Body)
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read request body"})
        return
    }

    var payload anthropic.AnthropicMessagesPayload
    if err := json.Unmarshal(bodyBytes, &payload); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid request: %v", err)})
        return
    }

    state.RLock()
    models := state.Models
    state.RUnlock()

    // 估算 token 数量，但不检查是否超限
    inputTokens := calculateAnthropicTokenCountOrDefault(payload, c.GetHeader("anthropic-beta"), models)
    c.JSON(http.StatusOK, gin.H{"input_tokens": inputTokens})
}
```

**特点**:
- 仅返回估算的 input_tokens，不检查是否超限
- 估算值用于客户端决策，但代理本身不做限制
- 与模型配置中的 `MaxContextWindowTokens` 完全脱离

---

## 6. Token 限制的实际影响流程

```
客户端请求
    ↓
request_prep.go: rewriteCompletionsPayload()
  - 应用默认 MaxTokens (仅输出层面)
  - 不检查上下文窗口
    ↓
代理转发上游
    ↓
上游模型处理
  - 如果超限，上游返回 400/413/500 等错误
    ↓
handler.go: ForwardCompletionsResponse()
    ↓
upstreamerr/handler.go: HandleUpstreamError()
  - 映射为通用错误消息
  - 用户得不到有用的超限信息
```

---

## 7. 现有的防护措施

### 7.1 Rate Limiting (速率限制)
**文件: instance/rate_limiter.go**

```go
// TokenBucket 实现了令牌桶速率限制
// 限制的是 RPM (requests per minute)，不是 token 消耗
```

- 限制的是请求频率，不是 token 使用量

### 7.2 多账户负载均衡
**文件: handler/proxy.go (第 162-221 行)**

```go
// proxyCompletions 支持 3 次重试
if isPool == true {
    maxAttempts = 3
}

// 重试条件
if isRetryableStatus(resp.StatusCode) && attempt < maxAttempts-1 {
    exclude[resolved.AccountID] = true
    continue  // 尝试不同账户
}
```

- 当请求失败时，在多账户间重试
- 但仍然是被动的，而非主动预防

---

## 8. 关键发现总结

| 项目 | 状态 | 说明 |
|------|------|------|
| **ModelLimits 定义** | ✅ 已实现 | 结构定义在 config.go，包含上下文窗口和输出限制 |
| **Token 计数** | ✅ 已实现 | 详细的 token 计数逻辑，支持消息、工具、图片等 |
| **模型参数调整** | ✅ 已实现 | 15% Claude, 3% Grok 的调整系数 |
| **请求准备** | ⚠️ 部分 | 仅应用输出限制，不检查上下文窗口 |
| **超限主动检查** | ❌ 缺失 | 无任何代码检查请求是否超过上下文窗口 |
| **自动截断/压缩** | ❌ 缺失 | 完全没有实现消息截断或历史压缩 |
| **错误识别** | ❌ 缺失 | 无法识别和响应 token 超限错误 |
| **用户友好的错误** | ❌ 缺失 | 返回通用错误，不提示 token 超限具体信息 |

---

## 9. 建议的改进方案

### 9.1 主动检查机制
```go
func checkContextWindowLimit(payload ChatCompletionsPayload, 
                            model ModelEntry) (bool, string) {
    maxContext := model.Capabilities.Limits.MaxContextWindowTokens
    estimatedTokens := estimateRequestTokens(payload)
    
    if estimatedTokens > maxContext {
        return false, fmt.Sprintf(
            "Request exceeds context window: %d > %d tokens",
            estimatedTokens, maxContext)
    }
    return true, ""
}
```

### 9.2 自动截断策略
```go
// 优先级：
// 1. 根据优先级移除最早的消息
// 2. 保留最后 N 条消息 + 最新用户输入
// 3. 可选：总结历史内容
// 4. 可选：按重要性采样
```

### 9.3 错误响应增强
```go
// 识别上游错误类型
if strings.Contains(errorBody, "context_length") {
    return UpstreamError{
        Type: "context_length_exceeded",
        Message: "Request exceeds model context window",
    }
}
```

---

## 10. 源代码关键路径

### 核心文件依赖图
```
handler/proxy.go (路由)
    ↓
instance/handler.go (请求处理)
    ├── request_prep.go (请求转换)
    │   └── applyDefaultMaxTokens()
    ├── token_count.go (token 估算)
    │   ├── calculateAnthropicTokenCountOrDefault()
    │   ├── getTokenCount()
    │   └── applyAnthropicTokenAdjustments()
    └── upstreamerr/handler.go (错误处理)
        └── error_map.go (错误映射)

config/config.go (数据模型)
    ├── ModelLimits
    ├── ModelCapabilities
    └── ModelEntry
```

---
