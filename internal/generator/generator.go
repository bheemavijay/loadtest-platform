package generator

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
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
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return fallbackUUID()
	}

	bytes[6] = (bytes[6] & 0x0f) | 0x40
	bytes[8] = (bytes[8] & 0x3f) | 0x80

	return fmt.Sprintf(
		"%s-%s-%s-%s-%s",
		hex.EncodeToString(bytes[0:4]),
		hex.EncodeToString(bytes[4:6]),
		hex.EncodeToString(bytes[6:8]),
		hex.EncodeToString(bytes[8:10]),
		hex.EncodeToString(bytes[10:16]),
	)
}

func randomIPv4() string {
	var bytes [2]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		now := time.Now().UnixNano()
		return fmt.Sprintf("192.168.%d.%d", now%255, (now/255)%255)
	}

	return fmt.Sprintf("192.168.%d.%d", bytes[0], bytes[1])
}

func randomUint32() uint32 {
	var bytes [4]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return uint32(time.Now().UnixNano())
	}

	return uint32(bytes[0])<<24 | uint32(bytes[1])<<16 | uint32(bytes[2])<<8 | uint32(bytes[3])
}

func fallbackUUID() string {
	now := time.Now().UnixNano()
	hexNow := fmt.Sprintf("%032x", now)
	return fmt.Sprintf("%s-%s-%s-%s-%s", hexNow[0:8], hexNow[8:12], "4000", "8000", hexNow[16:28])
}
