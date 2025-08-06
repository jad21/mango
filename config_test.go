package main

import "testing"

func TestReadOptionFile(t *testing.T) {
	config_file := "./fixtures/options/.mango"
	_, err := ReadConfig(config_file)
	if err != nil {
		t.Fatalf("Could not read config file: %s", err)
	}
}
