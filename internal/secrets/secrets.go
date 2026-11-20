package secrets

// Secret storage: values live in the OS keyring (Keychain / Secret Service /
// Windows Credential Manager), never in the git vault. Key scheme:
//   service = "rapi"
//   user    = "<collectionID>/<envName>/<varName>"

import (
	"fmt"
	"strings"

	"github.com/zalando/go-keyring"
)

const service = "rapi"

// Key builds the keyring user key for a variable.
func Key(collectionID, envName, name string) string {
	return collectionID + "/" + envName + "/" + name
}

// Set stores a secret value.
func Set(collectionID, envName, name, value string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("variable name is empty")
	}
	return keyring.Set(service, Key(collectionID, envName, name), value)
}

// Get reads a secret value.
func Get(collectionID, envName, name string) (string, error) {
	return keyring.Get(service, Key(collectionID, envName, name))
}

// Has reports whether a secret exists in the keyring.
func Has(collectionID, envName, name string) bool {
	_, err := keyring.Get(service, Key(collectionID, envName, name))
	return err == nil
}

// Delete removes a secret; missing entries are not an error.
func Delete(collectionID, envName, name string) error {
	err := keyring.Delete(service, Key(collectionID, envName, name))
	if err != nil && strings.Contains(strings.ToLower(err.Error()), "not found") {
		return nil
	}
	return err
}
