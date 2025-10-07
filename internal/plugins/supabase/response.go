package supabase

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
)

// parseSupabaseResponse parses HTTP response into SupabaseResponse
func parseSupabaseResponse(resp *http.Response) (*SupabaseResponse, error) {
	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Create response object
	response := &SupabaseResponse{
		Metadata: &ResponseMetadata{
			StatusCode: resp.StatusCode,
			Headers:    make(map[string]string),
		},
	}

	// Copy relevant headers
	for key, values := range resp.Header {
		if len(values) > 0 {
			response.Metadata.Headers[key] = values[0]
		}
	}

	// Handle errors
	if resp.StatusCode >= 400 {
		var supabaseErr SupabaseError
		if err := json.Unmarshal(body, &supabaseErr); err == nil {
			response.Error = &supabaseErr
		} else {
			response.Error = &SupabaseError{
				Message: string(body),
			}
		}
		return response, nil
	}

	// Parse successful response
	if len(body) > 0 {
		var data interface{}
		if err := json.Unmarshal(body, &data); err != nil {
			return nil, fmt.Errorf("failed to parse response JSON: %w", err)
		}
		response.Data = data
	}

	return response, nil
}

// parseContentRange parses Content-Range header for count information
func parseContentRange(contentRange string) int {
	// Format: "0-24/573" or "*/573"
	parts := strings.Split(contentRange, "/")
	if len(parts) != 2 {
		return -1
	}

	count, err := strconv.Atoi(parts[1])
	if err != nil {
		return -1
	}

	return count
}

// buildFilterParam builds a filter parameter for Supabase PostgREST API
func buildFilterParam(filter FilterConfig) string {
	switch filter.Operator {
	case OpEq:
		return fmt.Sprintf("eq.%v", filter.Value)
	case OpNeq:
		return fmt.Sprintf("neq.%v", filter.Value)
	case OpGt:
		return fmt.Sprintf("gt.%v", filter.Value)
	case OpGte:
		return fmt.Sprintf("gte.%v", filter.Value)
	case OpLt:
		return fmt.Sprintf("lt.%v", filter.Value)
	case OpLte:
		return fmt.Sprintf("lte.%v", filter.Value)
	case OpLike:
		return fmt.Sprintf("like.%v", filter.Value)
	case OpILike:
		return fmt.Sprintf("ilike.%v", filter.Value)
	case OpIs:
		return fmt.Sprintf("is.%v", filter.Value)
	case OpIn:
		// Handle array values for IN operator
		if arr, ok := filter.Value.([]interface{}); ok {
			values := make([]string, len(arr))
			for i, v := range arr {
				values[i] = fmt.Sprintf("%v", v)
			}
			return fmt.Sprintf("in.(%s)", strings.Join(values, ","))
		}
		return fmt.Sprintf("in.(%v)", filter.Value)
	default:
		return fmt.Sprintf("%s.%v", filter.Operator, filter.Value)
	}
}
