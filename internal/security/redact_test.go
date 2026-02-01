package security

import (
	"strings"
	"sync"
	"testing"
)

func TestRedactor_AddSensitive(t *testing.T) {
	redactor := NewRedactor()

	redactor.AddSensitive("password123")
	redactor.AddSensitive("secret456")

	if len(redactor.sensitiveValues) != 2 {
		t.Errorf("Expected 2 sensitive values, got %d", len(redactor.sensitiveValues))
	}
}

func TestRedactor_AddSensitive_IgnoresEmpty(t *testing.T) {
	redactor := NewRedactor()

	redactor.AddSensitive("")
	redactor.AddSensitive("   ")
	redactor.AddSensitive("valid")

	// Only "valid" should be added (empty string is ignored, but whitespace-only is not)
	if len(redactor.sensitiveValues) != 2 {
		t.Errorf("Expected 2 sensitive values, got %d", len(redactor.sensitiveValues))
	}
}

func TestRedactor_Redact_SingleValue(t *testing.T) {
	redactor := NewRedactor()
	redactor.AddSensitive("password123")

	input := "The password is password123 for this user"
	expected := "The password is [REDACTED] for this user"

	result := redactor.Redact(input)
	if result != expected {
		t.Errorf("Expected '%s', got '%s'", expected, result)
	}
}

func TestRedactor_Redact_MultipleOccurrences(t *testing.T) {
	redactor := NewRedactor()
	redactor.AddSensitive("secret")

	input := "The secret is secret and the secret stays secret"
	result := redactor.Redact(input)

	// All occurrences should be redacted
	if strings.Contains(result, "secret") {
		t.Errorf("Result still contains 'secret': %s", result)
	}

	// Should have 4 occurrences of [REDACTED]
	count := strings.Count(result, "[REDACTED]")
	if count != 4 {
		t.Errorf("Expected 4 redactions, got %d", count)
	}
}

func TestRedactor_Redact_MultipleSensitiveValues(t *testing.T) {
	redactor := NewRedactor()
	redactor.AddSensitive("password123")
	redactor.AddSensitive("apikey456")

	input := "password: password123, key: apikey456"
	result := redactor.Redact(input)

	if strings.Contains(result, "password123") {
		t.Error("Result still contains 'password123'")
	}
	if strings.Contains(result, "apikey456") {
		t.Error("Result still contains 'apikey456'")
	}

	expected := "password: [REDACTED], key: [REDACTED]"
	if result != expected {
		t.Errorf("Expected '%s', got '%s'", expected, result)
	}
}

func TestRedactor_Redact_EmptyString(t *testing.T) {
	redactor := NewRedactor()
	redactor.AddSensitive("password")

	result := redactor.Redact("")
	if result != "" {
		t.Errorf("Expected empty string, got '%s'", result)
	}
}

func TestRedactor_Redact_NoSensitiveValues(t *testing.T) {
	redactor := NewRedactor()

	input := "This is a normal string"
	result := redactor.Redact(input)

	if result != input {
		t.Errorf("Expected unchanged string, got '%s'", result)
	}
}

func TestRedactor_Redact_SubstringHandling(t *testing.T) {
	redactor := NewRedactor()
	// Add longer string first (should be sorted by length)
	redactor.AddSensitive("password")
	redactor.AddSensitive("pass")

	input := "The password is password"
	result := redactor.Redact(input)

	// Should redact "password" first, then "pass" won't match anything
	expected := "The [REDACTED] is [REDACTED]"
	if result != expected {
		t.Errorf("Expected '%s', got '%s'", expected, result)
	}
}

func TestRedactor_ThreadSafety(t *testing.T) {
	redactor := NewRedactor()

	var wg sync.WaitGroup
	numGoroutines := 100

	// Concurrent AddSensitive calls
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			redactor.AddSensitive("secret" + string(rune(n)))
		}(i)
	}

	// Concurrent Redact calls
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = redactor.Redact("This is a test string with secrets")
		}()
	}

	wg.Wait()
	// If we get here without deadlock or panic, thread safety works
}

func TestRedactor_Sorting(t *testing.T) {
	redactor := NewRedactor()

	// Add in random order
	redactor.AddSensitive("ab")
	redactor.AddSensitive("abcdef")
	redactor.AddSensitive("abcd")

	// Verify they're sorted by length (longest first)
	if len(redactor.sensitiveValues) != 3 {
		t.Fatalf("Expected 3 values, got %d", len(redactor.sensitiveValues))
	}

	if redactor.sensitiveValues[0] != "abcdef" {
		t.Errorf("Expected longest string first, got '%s'", redactor.sensitiveValues[0])
	}
	if redactor.sensitiveValues[1] != "abcd" {
		t.Errorf("Expected second longest string, got '%s'", redactor.sensitiveValues[1])
	}
	if redactor.sensitiveValues[2] != "ab" {
		t.Errorf("Expected shortest string last, got '%s'", redactor.sensitiveValues[2])
	}
}

func BenchmarkRedactor_Redact(b *testing.B) {
	redactor := NewRedactor()
	redactor.AddSensitive("password123")
	redactor.AddSensitive("secret456")
	redactor.AddSensitive("apikey789")

	input := strings.Repeat("Some text with password123 and secret456 and apikey789. ", 10)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = redactor.Redact(input)
	}
}
