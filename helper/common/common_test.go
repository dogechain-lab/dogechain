package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_UnicodeSubstr(t *testing.T) {
	textSimples := []string{
		"",
		"hello world",
		"你好世界",
		"こんにちは世界",
		"Привіт Світ",
	}

	// start >= 0, size <= len(string)
	{
		results := []string{
			"",
			"he",
			"你好",
			"こん",
			"Пр",
		}

		for i, text := range textSimples {
			result := Substr(text, 0, 2)

			assert.Equal(t, result, results[i])
		}
	}

	// start < 0, size <= len(string)
	{
		results := []string{
			"",
			"he",
			"你好",
			"こん",
			"Пр",
		}

		for i, text := range textSimples {
			result := Substr(text, -1, 2)

			assert.Equal(t, result, results[i])
		}
	}

	// start >= 0, size > len(string)
	{
		results := textSimples

		for i, text := range textSimples {
			result := Substr(text, 0, 255)

			assert.Equal(t, result, results[i])
		}
	}

	// start < 0, size > len(string)
	{
		results := textSimples

		for i, text := range textSimples {
			result := Substr(text, -1, 255)

			assert.Equal(t, result, results[i])
		}
	}
}
