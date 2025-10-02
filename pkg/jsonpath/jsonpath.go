package jsonpath
package jsonpath

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// ExtractValue extracts a value from JSON data using a simple path syntax
// Supports:
// - "field" - top level field
// - "field.subfield" - nested field
// - "field[0]" - array index
// - "field.array[1].subfield" - complex paths
func ExtractValue(jsonData []byte, path string) (string, error) {
	var data interface{}
	if err := json.Unmarshal(jsonData, &data); err != nil {
		return "", fmt.Errorf("failed to parse JSON: %w", err)
	}
	
	value, err := extractFromPath(data, path)
	if err != nil {
		return "", err
	}
	
	// Convert value to string
	return valueToString(value), nil
}

// ExtractMultipleValues extracts multiple values from JSON data
func ExtractMultipleValues(jsonData []byte, paths []string) ([]string, error) {
	var data interface{}
	if err := json.Unmarshal(jsonData, &data); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}
	
	var results []string
	for _, path := range paths {
		value, err := extractFromPath(data, path)
		if err != nil {
			return nil, fmt.Errorf("failed to extract path %s: %w", path, err)
		}
		results = append(results, valueToString(value))
	}
	
	return results, nil
}

// extractFromPath navigates through the data structure using the path
func extractFromPath(data interface{}, path string) (interface{}, error) {
	if path == "" {
		return data, nil
	}
	
	// Split path into segments
	segments := splitPath(path)
	current := data
	
	for _, segment := range segments {
		var err error
		current, err = navigateSegment(current, segment)
		if err != nil {
			return nil, err
		}
	}
	
	return current, nil
}

// splitPath splits a JSON path into segments, handling array indices
func splitPath(path string) []string {
	var segments []string
	var currentSegment strings.Builder
	inBracket := false
	
	for i, char := range path {
		switch char {
		case '.':
			if !inBracket && currentSegment.Len() > 0 {
				segments = append(segments, currentSegment.String())
				currentSegment.Reset()
			} else if !inBracket {
				// Skip leading dots
			} else {
				currentSegment.WriteRune(char)
			}
		case '[':
			if currentSegment.Len() > 0 {
				segments = append(segments, currentSegment.String())
				currentSegment.Reset()
			}
			inBracket = true
			currentSegment.WriteRune(char)
		case ']':
			inBracket = false
			currentSegment.WriteRune(char)
			if i < len(path)-1 && path[i+1] != '.' {
				// If there's more path after ], add this segment
				segments = append(segments, currentSegment.String())
				currentSegment.Reset()
			}
		default:
			currentSegment.WriteRune(char)
		}
	}
	
	if currentSegment.Len() > 0 {
		segments = append(segments, currentSegment.String())
	}
	
	return segments
}

// navigateSegment navigates one segment of the path
func navigateSegment(current interface{}, segment string) (interface{}, error) {
	// Handle array access like "[0]"
	if strings.HasPrefix(segment, "[") && strings.HasSuffix(segment, "]") {
		indexStr := segment[1 : len(segment)-1]
		index, err := strconv.Atoi(indexStr)
		if err != nil {
			return nil, fmt.Errorf("invalid array index: %s", indexStr)
		}
		
		switch arr := current.(type) {
		case []interface{}:
			if index < 0 || index >= len(arr) {
				return nil, fmt.Errorf("array index out of bounds: %d", index)
			}
			return arr[index], nil
		default:
			return nil, fmt.Errorf("cannot index non-array type")
		}
	}
	
	// Handle field access with potential array index like "field[0]"
	if strings.Contains(segment, "[") {
		// Split into field name and array index
		parts := strings.SplitN(segment, "[", 2)
		fieldName := parts[0]
		indexPart := "[" + parts[1] // Re-add the bracket
		
		// First navigate to the field
		fieldValue, err := navigateField(current, fieldName)
		if err != nil {
			return nil, err
		}
		
		// Then handle the array index
		return navigateSegment(fieldValue, indexPart)
	}
	
	// Simple field access
	return navigateField(current, segment)
}

// navigateField navigates to a named field in an object
func navigateField(current interface{}, fieldName string) (interface{}, error) {
	switch obj := current.(type) {
	case map[string]interface{}:
		value, exists := obj[fieldName]
		if !exists {
			return nil, fmt.Errorf("field not found: %s", fieldName)
		}
		return value, nil
	default:
		return nil, fmt.Errorf("cannot access field %s on non-object type", fieldName)
	}
}

// valueToString converts any value to its string representation
func valueToString(value interface{}) string {
	switch v := value.(type) {
	case string:
		return v
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case int:
		return strconv.Itoa(v)
	case bool:
		return strconv.FormatBool(v)
	case nil:
		return ""
	default:
		// For complex types, marshal back to JSON
		data, _ := json.Marshal(v)
		return string(data)
	}
}