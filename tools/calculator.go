package tools

import (
	"context"
	"fmt"
	"strconv"
)

type CalculatorTool struct{}

func (t *CalculatorTool) Name() string { return "calculator" }

func (t *CalculatorTool) Description() string {
	return "Performs basic arithmetic: add, subtract, multiply, divide"
}

func (t *CalculatorTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"operation": map[string]any{
				"type":        "string",
				"enum":        []string{"add", "subtract", "multiply", "divide"},
				"description": "The arithmetic operation to perform",
			},
			"a": map[string]any{"type": "number", "description": "First operand"},
			"b": map[string]any{"type": "number", "description": "Second operand"},
		},
		"required": []string{"operation", "a", "b"},
	}
}

func (t *CalculatorTool) Execute(_ context.Context, input map[string]any) (string, error) {
	op, _ := input["operation"].(string)
	a, err := toFloat(input["a"])
	if err != nil {
		return "", fmt.Errorf("invalid a: %w", err)
	}
	b, err := toFloat(input["b"])
	if err != nil {
		return "", fmt.Errorf("invalid b: %w", err)
	}

	var result float64
	switch op {
	case "add":
		result = a + b
	case "subtract":
		result = a - b
	case "multiply":
		result = a * b
	case "divide":
		if b == 0 {
			return "", fmt.Errorf("division by zero")
		}
		result = a / b
	default:
		return "", fmt.Errorf("unknown operation: %s", op)
	}

	return fmt.Sprintf("%.6g", result), nil
}

func toFloat(v any) (float64, error) {
	switch n := v.(type) {
	case float64:
		return n, nil
	case int:
		return float64(n), nil
	case string:
		return strconv.ParseFloat(n, 64)
	}
	return 0, fmt.Errorf("cannot convert %T to float", v)
}
