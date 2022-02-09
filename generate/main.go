package main

import (
	"fmt"
	"gopkg.in/yaml.v2"
	"os"
)

type Unit struct {
	Name     string   `yaml:"name"`
	Symbol   string   `yaml:"symbol"`
	fromBase string   `yaml:"fromBase"`
	toBase   string   `yaml:"toBase"`
	Matches  []string `yaml:"matches"`
}

type Definition struct {
	Type     string   `yaml:"type"`
	BaseUnit string   `yaml:"baseUnit"`
	Matches  []string `yaml:"matches"`
	Units    []Unit   `yaml:"units"`
}

type UnitsYaml struct {
	Version     string       `yaml:"version"`
	Definitions []Definition `yaml:"definitions"`
}

func main() {
	cwd, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	f, err := os.ReadFile(cwd + "/units.yaml")
	if err != nil {
		panic(err)
	}

	data := UnitsYaml{}
	err = yaml.Unmarshal(f, &data)
	if err != nil {
		panic(err)
	}

	fmt.Printf("--- t dump:\n%s\n\n", data)
}
