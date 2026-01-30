package config

import (
	"bufio"
	"os"

	"gopkg.in/yaml.v3"
)

func ReadConfig(path string) ([]Step, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	config := make([]Step, 0)

	decoder := yaml.NewDecoder(f)

	err = decoder.Decode(&config)
	if err != nil {
		return nil, err
	}

	return config, nil
}

func ReadVariables(path string) (map[string]interface{}, error) {
	if path == "" {
		return make(map[string]interface{}), nil
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := bufio.NewReader(file)

	variables := make(map[string]interface{})

	decoder := yaml.NewDecoder(reader)
	err = decoder.Decode(&variables)
	if err != nil {
		return nil, err
	}

	return variables, nil
}
