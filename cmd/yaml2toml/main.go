package main

import (
	"fmt"
	"log"

	"github.com/pelletier/go-toml/v2"
	"gopkg.in/yaml.v2"
)

type Data struct {
	ID     int    `toml:"id" yaml:"id"`
	Name   string `toml:"name" yaml:"name"`
	Values []byte `toml:"values" yaml:"values"`
}

const yamlData = `
id: 101
name: Gopher
values:
- 11
- 22
- 33
`

// ConvertYAMLToTOML unmarshals YAML into Data and marshals it to TOML.
func ConvertYAMLToTOML(yamlInput []byte) ([]byte, error) {
	var data Data
	if err := yaml.Unmarshal(yamlInput, &data); err != nil {
		return nil, err
	}
	return toml.Marshal(data)
}

func main() {
	tomlData, err := ConvertYAMLToTOML([]byte(yamlData))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(tomlData))
}
