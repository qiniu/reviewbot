package config

import (
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConfig(t *testing.T) {
	defaultTrue := true
	defaultFalse := false
	case1 := `
qbox:
  staticcheck:
    enable: false
  luacheck:
    enable: true
    workDir: "nginx/Lua"
    command: luacheck
`

	exp1 := map[string]map[string]Linter{
		"qbox": {
			"staticcheck": {Enable: &defaultFalse},
			"luacheck":    {Enable: &defaultTrue, WorkDir: "nginx/Lua", Command: "luacheck"},
		},
	}

	case2 := `
qbox:
  luacheck:
    enable: false
    workDir: "nginx/Lua"
    command: luacheck
`
	exp2 := map[string]map[string]Linter{
		"qbox": {
			"luacheck": {Enable: &defaultFalse, WorkDir: "nginx/Lua", Command: "luacheck"},
		},
	}

	case3 := ``
	exp3 := map[string]map[string]Linter{
		"qbox": {
			"staticcheck": {Enable: &defaultTrue},
			"govet":       {Enable: &defaultTrue},
			"luacheck":    {Enable: &defaultTrue},
		},
	}

	cs := map[string]map[string]map[string]Linter{
		case1: exp1,
		case2: exp2,
		case3: exp3,
	}

	tempDir := t.TempDir()
	defer os.RemoveAll(tempDir)
	for k, v := range cs {
		f := path.Join("./", "configtest_ut.yaml")
		err := os.WriteFile(f, []byte(k), 0o666)
		assert.NoError(t, err)

		res, er := NewConfig(f)
		assert.NoError(t, er)
		assert.EqualValues(t, v, res)
	}
}
