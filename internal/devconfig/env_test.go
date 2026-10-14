package devconfig

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLiteralConfigurationDoesNotExpandOrExecute(t *testing.T) {
	values, err := parse(strings.NewReader("\ufeff# local\r\nSECRET='$(echo secret)'\r\nURL=\"${SECRET}#literal\"\r\nEMPTY=\r\n"))
	if err != nil || values["SECRET"] != "$(echo secret)" || values["URL"] != "${SECRET}#literal" || values["EMPTY"] != "" {
		t.Fatalf("literal parsing failed: %v", err)
	}
}

func TestLoadPreservesExistingEnvironment(t *testing.T) {
	t.Setenv("CAR_CRUDS_DEVCONFIG_TEST", "")
	path := filepath.Join(t.TempDir(), "local.env")
	if err := os.WriteFile(path, []byte("CAR_CRUDS_DEVCONFIG_TEST=secret-from-file\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := Load(path); err != nil || os.Getenv("CAR_CRUDS_DEVCONFIG_TEST") != "" {
		t.Fatalf("explicit environment did not take precedence: %v", err)
	}
}

func TestInvalidConfigurationDoesNotExposeValues(t *testing.T) {
	for _, input := range []string{"not-a-key=private-token", "TOKEN='private-token", "TOKEN=private-token\nTOKEN=another-secret"} {
		_, err := parse(strings.NewReader(input))
		if err == nil || strings.Contains(err.Error(), "private-token") || strings.Contains(err.Error(), "another-secret") {
			t.Fatal("malformed configuration must report only its location and classification")
		}
	}
}
