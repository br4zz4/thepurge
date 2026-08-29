package config

import (
	"strings"
	"testing"
)

func TestMarshalConfig(t *testing.T) {
	cfg := &Config{
		Storage: Storage{ThresholdType: "percent", ThresholdValue: 15, Volume: "/"},
	}
	data, err := Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	if !strings.Contains(s, "threshold_type: percent") {
		t.Errorf("marshaled config missing threshold_type; got:\n%s", s)
	}
	if !strings.Contains(s, "threshold_value: 15") {
		t.Errorf("marshaled config missing threshold_value; got:\n%s", s)
	}
}
