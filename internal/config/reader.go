package config

import (
	"bufio"
	"os"

	"github.com/alehatsman/mooncake/internal/logger"
	"gopkg.in/yaml.v3"
)

func ReadConfig(path string) ([]Step, error) {
	logger.Debugf("Reading configuration from file: %v", path)

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

	logger.Debugf("Read configuration with %v steps", len(config))

	return config, nil
}

func ReadVariables(path string) (map[string]interface{}, error) {
	if path == "" {
		return make(map[string]interface{}), nil
	}

	logger.Debugf("Reading variables from file: %v", path)

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

	logger.Debugf("Read variables: %v", variables)

	return variables, nil
}
