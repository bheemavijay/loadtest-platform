package generator

import (
	"encoding/json"
	"fmt"
	// Using math/rand for high-performance non-crypto random generation.
	"math/rand/v2"
	"strconv"
	"strings"
	"time"
)

func ReplacePlaceholders(input string) string {
	if !strings.Contains(input, "{{") {
		return input
	}

	var builder strings.Builder
	builder.Grow(len(input) + 32)

	for index := 0; index < len(input); {
		start := strings.Index(input[index:], "{{")
		if start < 0 {
			builder.WriteString(input[index:])
			break
		}

		start += index
		builder.WriteString(input[index:start])

		end := strings.Index(input[start+2:], "}}")
		if end < 0 {
			builder.WriteString(input[start:])
			break
		}

		end += start + 2
		token := input[start+2 : end]
		builder.WriteString(resolvePlaceholder(token))
		index = end + 2
	}

	return builder.String()
}

func GenerateDynamicBody(input string) ([]byte, error) {
	if strings.TrimSpace(input) == "" {
		return []byte(input), nil
	}

	decoder := json.NewDecoder(strings.NewReader(input))
	decoder.UseNumber()

	var payload any
	if err := decoder.Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode dynamic body: %w", err)
	}

	generated, err := generateValue(payload, nil)
	if err != nil {
		return nil, err
	}

	body, err := json.Marshal(generated)
	if err != nil {
		return nil, fmt.Errorf("encode dynamic body: %w", err)
	}

	return body, nil
}

func generateValue(value any, path []string) (any, error) {
	switch typed := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(typed))
		for key, child := range typed {
			nextPath := append(path, key)

			switch {
			case key == "visitorId":
				result[key] = randomUUID()
			case key == "visitId":
				result[key] = randomUUID()
			case key == "timestamp":
				result[key] = time.Now().UnixMilli()
			case key == "ip" && len(path) > 0 && path[len(path)-1] == "deviceParam":
				result[key] = randomIPv4()
			default:
				generatedChild, err := generateValue(child, nextPath)
				if err != nil {
					return nil, err
				}
				result[key] = generatedChild
			}
		}
		return result, nil
	case []any:
		result := make([]any, len(typed))
		for index, child := range typed {
			generatedChild, err := generateValue(child, path)
			if err != nil {
				return nil, err
			}
			result[index] = generatedChild
		}
		return result, nil
	case string:
		return ReplacePlaceholders(typed), nil
	default:
		return typed, nil
	}
}

func resolvePlaceholder(token string) string {
	switch strings.TrimSpace(token) {
	case "uuid":
		return randomUUID()
	case "timestamp":
		return strconv.FormatInt(time.Now().UnixMilli(), 10)
	case "random_int":
		return strconv.FormatUint(uint64(randomUint32()), 10)
	case "random_ip":
		return randomIPv4()
	default:
		return "{{" + token + "}}"
	}
}

func randomUUID() string {
	high := rand.Uint64()
	low := rand.Uint64()

	timeLow := uint32(high >> 32)
	timeMid := uint16((high >> 16) & 0xffff)
	timeHiAndVersion := uint16(high&0x0fff) | 0x4000
	clockSeq := uint16((low >> 48) & 0x3fff)
	clockSeq = clockSeq | 0x8000
	node := low & 0x0000ffffffffffff

	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", timeLow, timeMid, timeHiAndVersion, clockSeq, node)
}

func randomIPv4() string {
	return fmt.Sprintf("192.168.%d.%d", rand.UintN(256), rand.UintN(256))
}

func randomUint32() uint32 {
	return rand.Uint32()
}
