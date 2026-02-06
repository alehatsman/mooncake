package utils

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCreateBackup tests the backup creation functionality
func TestCreateBackup(t *testing.T) {
	// Create temp directory for tests
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	testContent := "test content for backup"

	// Create source file
	if err := os.WriteFile(testFile, []byte(testContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Test backup creation
	backupPath, err := CreateBackup(testFile)
	if err != nil {
		t.Fatalf("CreateBackup failed: %v", err)
	}

	// Verify backup file exists
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		t.Errorf("Backup file was not created: %s", backupPath)
	}

	// Verify backup content matches original
	backupContent, err := os.ReadFile(backupPath)
	if err != nil {
		t.Fatalf("Failed to read backup file: %v", err)
	}
	if string(backupContent) != testContent {
		t.Errorf("Backup content mismatch: got %q, want %q", string(backupContent), testContent)
	}

	// Verify backup path format (should contain timestamp)
	if !strings.HasSuffix(backupPath, ".bak") {
		t.Errorf("Backup path should end with .bak: %s", backupPath)
	}
	if !strings.Contains(backupPath, testFile) {
		t.Errorf("Backup path should contain original filename: %s", backupPath)
	}

	// Clean up
	os.Remove(backupPath)
}

func TestCreateBackup_NonExistentFile(t *testing.T) {
	tmpDir := t.TempDir()
	nonExistentFile := filepath.Join(tmpDir, "does-not-exist.txt")

	_, err := CreateBackup(nonExistentFile)
	if err == nil {
		t.Error("CreateBackup should fail for non-existent file")
	}
	if !strings.Contains(err.Error(), "source file does not exist") {
		t.Errorf("Expected 'source file does not exist' error, got: %v", err)
	}
}

func TestCreateBackup_PermissionPreservation(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "perm-test.txt")

	// Create file with specific permissions
	if err := os.WriteFile(testFile, []byte("test"), 0600); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create backup
	backupPath, err := CreateBackup(testFile)
	if err != nil {
		t.Fatalf("CreateBackup failed: %v", err)
	}
	defer os.Remove(backupPath)

	// Check backup permissions
	srcInfo, _ := os.Stat(testFile)
	backupInfo, _ := os.Stat(backupPath)

	if srcInfo.Mode() != backupInfo.Mode() {
		t.Errorf("Backup permissions mismatch: got %v, want %v", backupInfo.Mode(), srcInfo.Mode())
	}
}

func TestCreateBackup_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "empty.txt")

	// Create empty file
	if err := os.WriteFile(testFile, []byte(""), 0644); err != nil {
		t.Fatalf("Failed to create empty file: %v", err)
	}

	// Create backup
	backupPath, err := CreateBackup(testFile)
	if err != nil {
		t.Fatalf("CreateBackup failed for empty file: %v", err)
	}
	defer os.Remove(backupPath)

	// Verify backup is also empty
	backupContent, _ := os.ReadFile(backupPath)
	if len(backupContent) != 0 {
		t.Errorf("Expected empty backup file, got %d bytes", len(backupContent))
	}
}

// TestCalculateSHA256 tests SHA256 checksum calculation
func TestCalculateSHA256(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "sha256-test.txt")
	testContent := "test content for sha256"

	// Create test file
	if err := os.WriteFile(testFile, []byte(testContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Calculate checksum
	checksum, err := CalculateSHA256(testFile)
	if err != nil {
		t.Fatalf("CalculateSHA256 failed: %v", err)
	}

	// Verify checksum length (SHA256 is 64 hex characters)
	if len(checksum) != 64 {
		t.Errorf("SHA256 checksum should be 64 characters, got %d", len(checksum))
	}

	// Verify checksum is consistent
	checksum2, err := CalculateSHA256(testFile)
	if err != nil {
		t.Fatalf("Second CalculateSHA256 failed: %v", err)
	}
	if checksum != checksum2 {
		t.Error("SHA256 checksums should be identical for same file")
	}

	// Verify checksum is lowercase hex
	for _, c := range checksum {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("Checksum contains invalid character: %c", c)
		}
	}
}

func TestCalculateSHA256_NonExistentFile(t *testing.T) {
	tmpDir := t.TempDir()
	nonExistentFile := filepath.Join(tmpDir, "does-not-exist.txt")

	_, err := CalculateSHA256(nonExistentFile)
	if err == nil {
		t.Error("CalculateSHA256 should fail for non-existent file")
	}
}

func TestCalculateSHA256_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "empty.txt")

	// Create empty file
	if err := os.WriteFile(testFile, []byte(""), 0644); err != nil {
		t.Fatalf("Failed to create empty file: %v", err)
	}

	// Calculate checksum
	checksum, err := CalculateSHA256(testFile)
	if err != nil {
		t.Fatalf("CalculateSHA256 failed for empty file: %v", err)
	}

	// SHA256 of empty file is a known value
	expectedEmpty := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	if checksum != expectedEmpty {
		t.Errorf("SHA256 of empty file incorrect: got %s, want %s", checksum, expectedEmpty)
	}
}

// TestCalculateMD5 tests MD5 checksum calculation
func TestCalculateMD5(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "md5-test.txt")
	testContent := "test content for md5"

	// Create test file
	if err := os.WriteFile(testFile, []byte(testContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Calculate checksum
	checksum, err := CalculateMD5(testFile)
	if err != nil {
		t.Fatalf("CalculateMD5 failed: %v", err)
	}

	// Verify checksum length (MD5 is 32 hex characters)
	if len(checksum) != 32 {
		t.Errorf("MD5 checksum should be 32 characters, got %d", len(checksum))
	}

	// Verify checksum is consistent
	checksum2, err := CalculateMD5(testFile)
	if err != nil {
		t.Fatalf("Second CalculateMD5 failed: %v", err)
	}
	if checksum != checksum2 {
		t.Error("MD5 checksums should be identical for same file")
	}

	// Verify checksum is lowercase hex
	for _, c := range checksum {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("Checksum contains invalid character: %c", c)
		}
	}
}

func TestCalculateMD5_NonExistentFile(t *testing.T) {
	tmpDir := t.TempDir()
	nonExistentFile := filepath.Join(tmpDir, "does-not-exist.txt")

	_, err := CalculateMD5(nonExistentFile)
	if err == nil {
		t.Error("CalculateMD5 should fail for non-existent file")
	}
}

func TestCalculateMD5_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "empty.txt")

	// Create empty file
	if err := os.WriteFile(testFile, []byte(""), 0644); err != nil {
		t.Fatalf("Failed to create empty file: %v", err)
	}

	// Calculate checksum
	checksum, err := CalculateMD5(testFile)
	if err != nil {
		t.Fatalf("CalculateMD5 failed for empty file: %v", err)
	}

	// MD5 of empty file is a known value
	expectedEmpty := "d41d8cd98f00b204e9800998ecf8427e"
	if checksum != expectedEmpty {
		t.Errorf("MD5 of empty file incorrect: got %s, want %s", checksum, expectedEmpty)
	}
}

// TestVerifyChecksum tests checksum verification
func TestVerifyChecksum_SHA256Match(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "verify-test.txt")
	testContent := "test content"

	// Create test file
	if err := os.WriteFile(testFile, []byte(testContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Get actual checksum
	expected, err := CalculateSHA256(testFile)
	if err != nil {
		t.Fatalf("Failed to calculate SHA256: %v", err)
	}

	// Verify checksum matches
	match, err := VerifyChecksum(testFile, expected)
	if err != nil {
		t.Fatalf("VerifyChecksum failed: %v", err)
	}
	if !match {
		t.Error("VerifyChecksum should return true for matching SHA256")
	}
}

func TestVerifyChecksum_SHA256Mismatch(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "verify-test.txt")

	// Create test file
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Use wrong checksum (all zeros)
	wrongChecksum := "0000000000000000000000000000000000000000000000000000000000000000"

	// Verify checksum does not match
	match, err := VerifyChecksum(testFile, wrongChecksum)
	if err != nil {
		t.Fatalf("VerifyChecksum failed: %v", err)
	}
	if match {
		t.Error("VerifyChecksum should return false for non-matching SHA256")
	}
}

func TestVerifyChecksum_MD5Match(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "verify-test.txt")
	testContent := "test content"

	// Create test file
	if err := os.WriteFile(testFile, []byte(testContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Get actual checksum
	expected, err := CalculateMD5(testFile)
	if err != nil {
		t.Fatalf("Failed to calculate MD5: %v", err)
	}

	// Verify checksum matches
	match, err := VerifyChecksum(testFile, expected)
	if err != nil {
		t.Fatalf("VerifyChecksum failed: %v", err)
	}
	if !match {
		t.Error("VerifyChecksum should return true for matching MD5")
	}
}

func TestVerifyChecksum_MD5Mismatch(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "verify-test.txt")

	// Create test file
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Use wrong checksum (all zeros)
	wrongChecksum := "00000000000000000000000000000000"

	// Verify checksum does not match
	match, err := VerifyChecksum(testFile, wrongChecksum)
	if err != nil {
		t.Fatalf("VerifyChecksum failed: %v", err)
	}
	if match {
		t.Error("VerifyChecksum should return false for non-matching MD5")
	}
}

func TestVerifyChecksum_InvalidLength(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "verify-test.txt")

	// Create test file
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Use checksum with invalid length
	invalidChecksum := "invalid"

	// Verify should return error
	_, err := VerifyChecksum(testFile, invalidChecksum)
	if err == nil {
		t.Error("VerifyChecksum should fail for invalid checksum length")
	}
	if !strings.Contains(err.Error(), "unsupported checksum format") {
		t.Errorf("Expected 'unsupported checksum format' error, got: %v", err)
	}
}

func TestVerifyChecksum_NonExistentFile(t *testing.T) {
	tmpDir := t.TempDir()
	nonExistentFile := filepath.Join(tmpDir, "does-not-exist.txt")

	// Valid SHA256 checksum
	checksum := "0000000000000000000000000000000000000000000000000000000000000000"

	_, err := VerifyChecksum(nonExistentFile, checksum)
	if err == nil {
		t.Error("VerifyChecksum should fail for non-existent file")
	}
}

// TestMergeVariables tests variable map merging
func TestMergeVariables_EmptyMaps(t *testing.T) {
	base := make(map[string]interface{})
	override := make(map[string]interface{})

	result := MergeVariables(base, override)

	if len(result) != 0 {
		t.Errorf("Expected empty result, got %d entries", len(result))
	}
}

func TestMergeVariables_OverridePrecedence(t *testing.T) {
	base := map[string]interface{}{
		"key1": "base_value",
		"key2": "base_value",
	}
	override := map[string]interface{}{
		"key2": "override_value",
		"key3": "override_value",
	}

	result := MergeVariables(base, override)

	// Check key1 from base
	if result["key1"] != "base_value" {
		t.Errorf("key1 should have base value, got %v", result["key1"])
	}

	// Check key2 was overridden
	if result["key2"] != "override_value" {
		t.Errorf("key2 should have override value, got %v", result["key2"])
	}

	// Check key3 from override
	if result["key3"] != "override_value" {
		t.Errorf("key3 should have override value, got %v", result["key3"])
	}

	// Check total keys
	if len(result) != 3 {
		t.Errorf("Expected 3 keys, got %d", len(result))
	}
}

func TestMergeVariables_NilBase(t *testing.T) {
	override := map[string]interface{}{
		"key1": "value1",
	}

	result := MergeVariables(nil, override)

	if len(result) != 1 {
		t.Errorf("Expected 1 entry, got %d", len(result))
	}
	if result["key1"] != "value1" {
		t.Errorf("key1 should have override value, got %v", result["key1"])
	}
}

func TestMergeVariables_NilOverride(t *testing.T) {
	base := map[string]interface{}{
		"key1": "value1",
	}

	result := MergeVariables(base, nil)

	if len(result) != 1 {
		t.Errorf("Expected 1 entry, got %d", len(result))
	}
	if result["key1"] != "value1" {
		t.Errorf("key1 should have base value, got %v", result["key1"])
	}
}

func TestMergeVariables_BothNil(t *testing.T) {
	result := MergeVariables(nil, nil)

	if len(result) != 0 {
		t.Errorf("Expected empty result, got %d entries", len(result))
	}
}

func TestMergeVariables_DifferentTypes(t *testing.T) {
	base := map[string]interface{}{
		"string": "value",
		"int":    42,
		"bool":   true,
		"slice":  []string{"a", "b"},
		"map":    map[string]string{"nested": "value"},
	}
	override := map[string]interface{}{
		"int":    100,
		"float":  3.14,
		"string": "overridden",
	}

	result := MergeVariables(base, override)

	// Check overridden values
	if result["string"] != "overridden" {
		t.Errorf("string should be overridden, got %v", result["string"])
	}
	if result["int"] != 100 {
		t.Errorf("int should be overridden to 100, got %v", result["int"])
	}

	// Check preserved values
	if result["bool"] != true {
		t.Errorf("bool should be preserved, got %v", result["bool"])
	}

	// Check new values
	if result["float"] != 3.14 {
		t.Errorf("float should be added, got %v", result["float"])
	}

	// Check total keys
	if len(result) != 6 {
		t.Errorf("Expected 6 keys, got %d", len(result))
	}
}

func TestMergeVariables_DoesNotModifyOriginals(t *testing.T) {
	base := map[string]interface{}{
		"key1": "value1",
	}
	override := map[string]interface{}{
		"key2": "value2",
	}

	result := MergeVariables(base, override)

	// Modify result
	result["key3"] = "value3"

	// Check originals are not modified
	if len(base) != 1 {
		t.Error("MergeVariables should not modify base map")
	}
	if len(override) != 1 {
		t.Error("MergeVariables should not modify override map")
	}
	if _, exists := base["key3"]; exists {
		t.Error("New key should not appear in base map")
	}
	if _, exists := override["key3"]; exists {
		t.Error("New key should not appear in override map")
	}
}
