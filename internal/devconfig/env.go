// Package devconfig loads local development configuration for command-line
// tools. Application services must obtain configuration from their environment.
package devconfig

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
)

var keyPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// Load reads literal KEY=value assignments without shell expansion. Existing
// environment variables, including explicitly empty values, take precedence.
// A missing file permits tools to run with configuration supplied by CI.
// Errors never include values or file contents.
func Load(path string) error {
	if path == "" {
		return nil
	}
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return errors.New("no se pudo abrir la configuración local")
	}
	defer file.Close()
	values, err := parse(file)
	if err != nil {
		return err
	}
	for key, value := range values {
		if _, present := os.LookupEnv(key); !present {
			if err := os.Setenv(key, value); err != nil {
				return errors.New("no se pudo cargar la configuración local")
			}
		}
	}
	return nil
}

func parse(reader io.Reader) (map[string]string, error) {
	values := make(map[string]string)
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 4096), 1<<20)
	line := 0
	for scanner.Scan() {
		line++
		text := strings.TrimSpace(scanner.Text())
		if line == 1 {
			text = strings.TrimPrefix(text, "\ufeff")
		}
		if text == "" || strings.HasPrefix(text, "#") {
			continue
		}
		key, value, present := strings.Cut(text, "=")
		key, value = strings.TrimSpace(key), strings.TrimSpace(value)
		if !present || !keyPattern.MatchString(key) {
			return nil, fmt.Errorf("configuración local inválida en línea %d", line)
		}
		if _, duplicate := values[key]; duplicate {
			return nil, fmt.Errorf("asignación duplicada en línea %d", line)
		}
		if strings.HasPrefix(value, "\"") || strings.HasPrefix(value, "'") {
			quote := value[0]
			if len(value) < 2 || value[len(value)-1] != quote {
				return nil, fmt.Errorf("comillas incompletas en línea %d", line)
			}
			value = value[1 : len(value)-1]
		}
		values[key] = value
	}
	if scanner.Err() != nil {
		return nil, errors.New("no se pudo leer la configuración local")
	}
	return values, nil
}
