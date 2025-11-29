package server

import (
	"path"
	"regexp"
	"strings"
)

// MatchResult contains the result of path matching including extracted parameters
type MatchResult struct {
	Matches    bool
	PathParams map[string]string
}

// matchPathPatternWithParams checks if the request path matches a given pattern
// and extracts any path parameters. Returns match result with parameters.
func matchPathPatternWithParams(pattern, requestPath string) MatchResult {
	result := MatchResult{
		Matches:    false,
		PathParams: make(map[string]string),
	}

	// Check if pattern is a regex with named groups (starts with ^ or contains regex metacharacters)
	if strings.HasPrefix(pattern, "^") || strings.HasPrefix(pattern, "(?") {
		return matchRegexWithParams(pattern, requestPath)
	}

	// Normalize paths
	cleanPattern := path.Clean(pattern)
	cleanPath := path.Clean(requestPath)

	// Exact match (no params)
	if cleanPattern == cleanPath {
		result.Matches = true
		return result
	}

	// Match all wildcard
	if cleanPattern == "/*" || cleanPattern == "*" {
		result.Matches = true
		return result
	}

	// Remove leading slash for consistent comparison
	patternNoSlash := strings.TrimPrefix(cleanPattern, "/")
	requestPathNoSlash := strings.TrimPrefix(cleanPath, "/")

	// Wildcard handling (e.g., /api/*)
	if strings.HasSuffix(patternNoSlash, "*") {
		patternPrefix := strings.TrimSuffix(patternNoSlash, "*")
		result.Matches = strings.HasPrefix(requestPathNoSlash, patternPrefix)
		return result
	}

	// Parametric path matching (e.g., /users/{id} or /users/:id)
	patternParts := strings.Split(patternNoSlash, "/")
	pathParts := strings.Split(requestPathNoSlash, "/")

	if len(patternParts) != len(pathParts) {
		return result
	}

	for i, part := range patternParts {
		// Handle {param} style
		if strings.HasPrefix(part, "{") && strings.HasSuffix(part, "}") {
			paramName := strings.TrimPrefix(strings.TrimSuffix(part, "}"), "{")
			result.PathParams[paramName] = pathParts[i]
			continue
		}
		// Handle :param style
		if strings.HasPrefix(part, ":") {
			paramName := strings.TrimPrefix(part, ":")
			result.PathParams[paramName] = pathParts[i]
			continue
		}
		// Literal match
		if part != pathParts[i] {
			return result
		}
	}

	result.Matches = true
	return result
}

// matchRegexWithParams checks if the request path matches a regex pattern
// and extracts named capture groups as path parameters
func matchRegexWithParams(pattern, requestPath string) MatchResult {
	result := MatchResult{
		Matches:    false,
		PathParams: make(map[string]string),
	}

	re, err := regexp.Compile(pattern)
	if err != nil {
		return result
	}

	match := re.FindStringSubmatch(requestPath)
	if match == nil {
		return result
	}

	result.Matches = true

	// Extract named capture groups
	names := re.SubexpNames()
	for i, name := range names {
		if i > 0 && name != "" && i < len(match) {
			result.PathParams[name] = match[i]
		}
	}

	return result
}

// matchPathPattern is the legacy function for backward compatibility
// Supports: exact match, wildcard (*), parametric ({param} or :param), and regex (^...$)
func matchPathPattern(pattern, requestPath string) bool {
	return matchPathPatternWithParams(pattern, requestPath).Matches
}
