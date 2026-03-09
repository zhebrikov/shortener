package settings

import (
	"os"
	"testing"
)

func TestSettings(t *testing.T) {
	fname := `settings.json`
	settings := Settings{
		Port: 3000,
		Host: `localhost`,
	}
	if err := settings.Save(fname); err != nil {
		t.Error(err)
	}
	var result Settings
	if err := (&result).Load(fname); err != nil {
		t.Error(err)
	}
	if settings != result {
		t.Errorf(`%+v не равно %+v`, settings, result)
	}
	// удалим файл settings.json
	if err := os.Remove(fname); err != nil {
		t.Error(err)
	}
}
