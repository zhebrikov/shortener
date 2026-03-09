package settings

import (
	"encoding/json"
	"os"
)

type Settings struct {
	Port int    `json:"port"`
	Host string `json:"host"`
}

// Save сохраняет настройки в файле fname.
func (settings Settings) Save(fname string) error {
	// сериализуем структуру в JSON формат
	data, err := json.MarshalIndent(settings, "", "   ")
	if err != nil {
		return err
	}
	// сохраняем данные в файл
	return os.WriteFile(fname, data, 0666)
}

func (settings *Settings) Load(fname string) error {
	data, err := os.ReadFile(fname)
	if err != nil {
		return err
	}
	if err = json.Unmarshal(data, settings); err != nil {
		return err
	}
	return nil
}
