package main

import (
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"
	"gopkg.in/yaml.v2"
)

func TestConvertYAMLToTOML(t *testing.T) {
	tomlData, err := ConvertYAMLToTOML([]byte(yamlData))
	if err != nil {
		t.Fatalf("ConvertYAMLToTOML: %v", err)
	}

	// Проверяем, что результат — валидный TOML и восстанавливаем те же данные
	var decoded Data
	if err := toml.Unmarshal(tomlData, &decoded); err != nil {
		t.Fatalf("toml.Unmarshal: %v", err)
	}

	var expected Data
	if err := yaml.Unmarshal([]byte(yamlData), &expected); err != nil {
		t.Fatalf("yaml.Unmarshal: %v", err)
	}

	if decoded.ID != expected.ID {
		t.Errorf("ID: got %d, want %d", decoded.ID, expected.ID)
	}
	if decoded.Name != expected.Name {
		t.Errorf("Name: got %q, want %q", decoded.Name, expected.Name)
	}
	if len(decoded.Values) != len(expected.Values) {
		t.Errorf("Values length: got %d, want %d", len(decoded.Values), len(expected.Values))
	} else {
		for i := range decoded.Values {
			if decoded.Values[i] != expected.Values[i] {
				t.Errorf("Values[%d]: got %d, want %d", i, decoded.Values[i], expected.Values[i])
			}
		}
	}
}

func TestConvertYAMLToTOML_OutputContainsExpectedFields(t *testing.T) {
	tomlData, err := ConvertYAMLToTOML([]byte(yamlData))
	if err != nil {
		t.Fatalf("ConvertYAMLToTOML: %v", err)
	}
	s := string(tomlData)

	wantSubstrs := []string{"id = 101", "Gopher", "values ="}
	for _, sub := range wantSubstrs {
		if !strings.Contains(s, sub) {
			t.Errorf("output should contain %q, got:\n%s", sub, s)
		}
	}
}

func TestConvertYAMLToTOML_InvalidYAML(t *testing.T) {
	_, err := ConvertYAMLToTOML([]byte("invalid: yaml: ["))
	if err == nil {
		t.Error("expected error for invalid YAML, got nil")
	}
}

func TestConvertYAMLToTOML_EmptyInput(t *testing.T) {
	tomlData, err := ConvertYAMLToTOML([]byte(""))
	if err != nil {
		t.Fatalf("empty YAML should produce zero value: %v", err)
	}
	var decoded Data
	if err := toml.Unmarshal(tomlData, &decoded); err != nil {
		t.Fatalf("toml.Unmarshal: %v", err)
	}
	if decoded.ID != 0 || decoded.Name != "" || len(decoded.Values) != 0 {
		t.Errorf("expected zero value, got ID=%d Name=%q Values=%v", decoded.ID, decoded.Name, decoded.Values)
	}
}
