
package torbundle

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEnsure(t *testing.T) {
	// Cria diretório temporário para tor-data
	t.Dir()
	defer os.RemoveAll(t.Dir())

	// Garante que o Tor está instalado (no ambiente real, seria testar com binario)
	assert.NoError(t, Ensure(context.Background(), t.Dir()))
}