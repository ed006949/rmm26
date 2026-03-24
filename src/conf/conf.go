package conf

import (
	"encoding/json"
	"os"

	"github.com/rs/zerolog/log"
)

type Config struct {
	Description string `json:"description"`
	DB          string `json:"db"`
	UUID        string `json:"uuid"`
}

func LoadConfig(path string) (Config, error) {
	var (
		c    Config
		file *os.File
		err  error
	)

	file, err = os.Open(path)
	switch {
	case err != nil:
		log.Error().Err(err).Str("path", path).Msg("Failed to open config file")
		return c, err
	}
	defer file.Close()

	err = json.NewDecoder(file).Decode(&c)
	switch {
	case err != nil:
		log.Error().Err(err).Msg("Failed to decode config file")
		return c, err
	}

	return c, nil
}
