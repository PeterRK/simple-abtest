package utils

import (
	"encoding/json"
	"os"

	"gopkg.in/yaml.v2"
)

// LoadJsonFile reads a JSON file from filepath and decodes it into obj.
// obj must be a pointer to the target value.
func LoadJsonFile(filepath string, obj any) error {
	file, err := os.Open(filepath)
	if err != nil {
		return err
	}
	defer file.Close()
	return json.NewDecoder(file).Decode(obj)
}

// LoadYamlFile reads a YAML file from filepath and decodes it into obj.
// obj must be a pointer to the target value.
func LoadYamlFile(filepath string, obj any) error {
	file, err := os.Open(filepath)
	if err != nil {
		return err
	}
	defer file.Close()
	return yaml.NewDecoder(file).Decode(obj)
}
