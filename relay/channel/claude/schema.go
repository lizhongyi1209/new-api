package claude

// FunctionParametersToInputSchema converts OpenAI function parameters to Claude InputSchema.
// Preserves parameterless tools by providing default object schema when parameters is nil or empty.
func FunctionParametersToInputSchema(parameters any) map[string]any {
	params, _ := parameters.(map[string]any)
	schema := make(map[string]any, len(params)+2)
	for key, value := range params {
		schema[key] = value
	}
	if schema["type"] == nil {
		schema["type"] = "object"
	}
	if schema["properties"] == nil {
		schema["properties"] = map[string]any{}
	}
	return schema
}
