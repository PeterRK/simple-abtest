package utils

import (
	"os"

	json "github.com/goccy/go-json"
	"gopkg.in/yaml.v2"
)

func LoadJsonFile(filepath string, obj any) error {
	file, err := os.Open(filepath)
	if err != nil {
		return err
	}
	return json.NewDecoder(file).Decode(obj)
}

func LoadYamlFile(filepath string, obj any) error {
	file, err := os.Open(filepath)
	if err != nil {
		return err
	}
	return yaml.NewDecoder(file).Decode(obj)
}
