package cel

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
)

// Evaluator handles CEL expression evaluation for REST API responses
type Evaluator struct {
	env *cel.Env
}

// NewEvaluator creates a new CEL evaluator with standard functions and variables
func NewEvaluator() (*Evaluator, error) {
	env, err := cel.NewEnv(
		// Standard CEL library functions
		cel.Lib(&timeLibrary{}),
		// Declare available variables
		cel.Variable("response", cel.AnyType), // The JSON response object
		cel.Variable("data", cel.AnyType),     // Extracted data from DataExpression
		cel.Variable("now", cel.IntType),      // Current Unix timestamp
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create CEL environment: %w", err)
	}

	return &Evaluator{env: env}, nil
}

// EvaluateCondition evaluates a CEL condition expression against a JSON response
// Returns true if the condition passes, false otherwise
func (e *Evaluator) EvaluateCondition(condition string, responseData []byte) (bool, error) {
	if condition == "" {
		return true, nil // No condition means always proceed
	}

	// Parse JSON response
	var response interface{}
	if err := json.Unmarshal(responseData, &response); err != nil {
		return false, fmt.Errorf("failed to parse JSON response: %w", err)
	}

	// Compile CEL expression
	ast, issues := e.env.Compile(condition)
	if issues != nil && issues.Err() != nil {
		return false, fmt.Errorf("CEL compilation error: %w", issues.Err())
	}

	// Create program
	prg, err := e.env.Program(ast)
	if err != nil {
		return false, fmt.Errorf("CEL program creation error: %w", err)
	}

	// Evaluate with variables
	result, _, err := prg.Eval(map[string]interface{}{
		"response": response,
		"now":      time.Now().Unix(),
	})
	if err != nil {
		return false, fmt.Errorf("CEL evaluation error: %w", err)
	}

	// Convert result to boolean
	boolResult, ok := result.Value().(bool)
	if !ok {
		return false, fmt.Errorf("condition expression must return boolean, got %T", result.Value())
	}

	return boolResult, nil
}

// EvaluateDataExpression evaluates a CEL data extraction expression
// Returns the extracted data as a JSON string
func (e *Evaluator) EvaluateDataExpression(expression string, responseData []byte) (string, error) {
	if expression == "" {
		return "", nil // No expression means no data extraction
	}

	// Parse JSON response
	var response interface{}
	if err := json.Unmarshal(responseData, &response); err != nil {
		return "", fmt.Errorf("failed to parse JSON response: %w", err)
	}

	// Compile CEL expression
	ast, issues := e.env.Compile(expression)
	if issues != nil && issues.Err() != nil {
		return "", fmt.Errorf("CEL compilation error: %w", issues.Err())
	}

	// Create program
	prg, err := e.env.Program(ast)
	if err != nil {
		return "", fmt.Errorf("CEL program creation error: %w", err)
	}

	// Evaluate with variables
	result, _, err := prg.Eval(map[string]interface{}{
		"response": response,
		"now":      time.Now().Unix(),
	})
	if err != nil {
		return "", fmt.Errorf("CEL evaluation error: %w", err)
	}

	// Convert CEL result to a Go value we can work with
	resultValue := convertCELValue(result)

	if str, ok := resultValue.(string); ok {
		// If result is already a string, return it directly
		return str, nil
	}

	// Otherwise, marshal to JSON
	jsonData, err := json.Marshal(resultValue)
	if err != nil {
		return "", fmt.Errorf("failed to marshal result to JSON: %w", err)
	}

	return string(jsonData), nil
}

// EvaluateOutputFormat evaluates a CEL output formatting expression
// Takes the extracted data and formats it into the final output string
func (e *Evaluator) EvaluateOutputFormat(expression string, extractedData string, includeTimestamp bool) (string, error) {
	if expression == "" {
		// If no output format specified, use extracted data directly
		if includeTimestamp {
			timestamp := time.Now().Format(time.RFC3339)
			if extractedData == "" {
				return timestamp, nil
			}
			return timestamp + "," + extractedData, nil
		}
		return extractedData, nil
	}

	// Parse extracted data back to interface{} for CEL evaluation
	var data interface{}
	if extractedData != "" {
		if err := json.Unmarshal([]byte(extractedData), &data); err != nil {
			// If it's not valid JSON, treat as string
			data = extractedData
		}
	}

	// Compile CEL expression
	ast, issues := e.env.Compile(expression)
	if issues != nil && issues.Err() != nil {
		return "", fmt.Errorf("CEL compilation error: %w", issues.Err())
	}

	// Create program
	prg, err := e.env.Program(ast)
	if err != nil {
		return "", fmt.Errorf("CEL program creation error: %w", err)
	}

	// Evaluate with variables
	result, _, err := prg.Eval(map[string]interface{}{
		"data": data,
		"now":  time.Now().Unix(),
	})
	if err != nil {
		return "", fmt.Errorf("CEL evaluation error: %w", err)
	}

	// Convert result to string
	resultValue := convertCELValue(result)
	if str, ok := resultValue.(string); ok {
		return str, nil
	}

	return fmt.Sprintf("%v", resultValue), nil
}

// ProcessResponse is a convenience method that handles the full CEL processing pipeline
type ProcessRequest struct {
	Condition      string
	DataExpression string
	OutputFormat   string
	ResponseData   []byte
}

type ProcessResult struct {
	ConditionMet    bool
	ExtractedData   string
	FormattedOutput string
}

func (e *Evaluator) ProcessResponse(req ProcessRequest) (*ProcessResult, error) {
	result := &ProcessResult{}

	// 1. Evaluate condition
	conditionMet, err := e.EvaluateCondition(req.Condition, req.ResponseData)
	if err != nil {
		return nil, fmt.Errorf("condition evaluation failed: %w", err)
	}
	result.ConditionMet = conditionMet

	if !conditionMet {
		return result, nil // Don't proceed with data extraction if condition failed
	}

	// 2. Extract data
	extractedData, err := e.EvaluateDataExpression(req.DataExpression, req.ResponseData)
	if err != nil {
		return nil, fmt.Errorf("data extraction failed: %w", err)
	}
	result.ExtractedData = extractedData

	// 3. Format output
	formattedOutput, err := e.EvaluateOutputFormat(req.OutputFormat, extractedData, false)
	if err != nil {
		return nil, fmt.Errorf("output formatting failed: %w", err)
	}
	result.FormattedOutput = formattedOutput

	return result, nil
}

// timeLibrary provides time-related CEL functions
type timeLibrary struct{}

func (t *timeLibrary) CompileOptions() []cel.EnvOption {
	return []cel.EnvOption{
		cel.Function("timestamp",
			cel.Overload("timestamp_string",
				[]*cel.Type{}, cel.StringType,
				cel.FunctionBinding(func(args ...ref.Val) ref.Val {
					return types.String(time.Now().Format(time.RFC3339))
				}),
			),
		),
		cel.Function("unixtime",
			cel.Overload("unixtime_int",
				[]*cel.Type{}, cel.IntType,
				cel.FunctionBinding(func(args ...ref.Val) ref.Val {
					return types.Int(time.Now().Unix())
				}),
			),
		),
	}
}

func (t *timeLibrary) ProgramOptions() []cel.ProgramOption {
	return []cel.ProgramOption{}
}

// convertCELValue converts a CEL ref.Val to a native Go value
func convertCELValue(val ref.Val) interface{} {
	switch v := val.(type) {
	case types.String:
		return string(v)
	case types.Int:
		return int64(v)
	case types.Double:
		return float64(v)
	case types.Bool:
		return bool(v)
	case types.Null:
		return nil
	default:
		// For complex types (maps, lists), convert recursively
		nativeVal := val.Value()

		// Handle map types
		if mapVal, ok := nativeVal.(map[string]interface{}); ok {
			result := make(map[string]interface{})
			for k, v := range mapVal {
				if celVal, ok := v.(ref.Val); ok {
					result[k] = convertCELValue(celVal)
				} else {
					result[k] = v
				}
			}
			return result
		}

		// Handle slice types
		if sliceVal, ok := nativeVal.([]interface{}); ok {
			result := make([]interface{}, len(sliceVal))
			for i, v := range sliceVal {
				if celVal, ok := v.(ref.Val); ok {
					result[i] = convertCELValue(celVal)
				} else {
					result[i] = v
				}
			}
			return result
		}

		// Return native value as-is for other types
		return nativeVal
	}
}
