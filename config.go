package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"gopkg.in/yaml.v3"
)

type Server struct {
	User       string `yaml:"user"`
	Host       string `yaml:"host"`
	PrivateKey string `yaml:"private_key"`
	Port       string `yaml:"port"`
}

type SyncItem struct {
	Server                   string `yaml:"server"`
	Log                      string `yaml:"log"`
	Source                   string `yaml:"source"`
	Dest                     string `yaml:"dest"`
	DeleteExtraneousFromDest bool   `yaml:"delete_extraneous_from_dest"`
	Verbose                  uint   `yaml:"verbose"`
	DryRun                   bool   `yaml:"dry_run"`
	Exclude                  []string
}

type CompressAndCopyItem struct {
		Server   string `yaml:"server"`
		Filename string `yaml:"filename"`
		Log      string `yaml:"log"`
		Source   string `yaml:"source"`
		Dest     string `yaml:"dest"`
		Verbose  uint   `yaml:"verbose"`
		DryRun   bool   `yaml:"dry_run"`
		Exclude  []string
	}

type Config struct {
	Verbose uint `yaml:"verbose"`

	Servers map[string]Server `yaml:"servers"`
	CompressAndCopy []CompressAndCopyItem `yaml:"compress_and_copy"`
	Sync []SyncItem `yaml:"sync"`
}

func getConfig() *Config {
	cfgPath, err := ParseFlags()
	if err != nil {
		log.Fatal(err)
	}
	cfg, err := NewConfig(cfgPath)
	if err != nil {
		log.Fatal(err)
	}
	return cfg
}

// ParseFlags will create and parse the CLI flags
// and return the path to be used elsewhere
func ParseFlags() (string, error) {
	// String that contains the configured configuration path
	var configPath string

	// Set up a CLI flag called "-config" to allow users
	// to supply the configuration file
	flag.StringVar(&configPath, "config", "./config.yml", "path to config file")

	// Actually parse the flags
	flag.Parse()

	// Validate the path first
	if err := ValidateConfigPath(configPath); err != nil {
		return "", err
	}

	// Return the configuration path
	return configPath, nil
}

// NewConfig returns a new decoded Config struct
func NewConfig(configPath string) (*Config, error) {
	// Create config structure
	config := &Config{}

	// Open config file
	file, err := os.Open(configPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// Init new YAML decode
	d := yaml.NewDecoder(file)

	// Start YAML decoding from file
	if err := d.Decode(&config); err != nil {
		return nil, err
	}

	return config, nil
}

// ValidateConfigPath just makes sure, that the path provided is a file,
// that can be read
func ValidateConfigPath(path string) error {
	s, err := os.Stat(path)
	if err != nil {
		return err
	}
	if s.IsDir() {
		return fmt.Errorf("'%v' is a directory, not a normal file", path)
	}
	return nil
}
