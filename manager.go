package kag3

import (
	"bytes"
	"io/fs"

	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"golang.org/x/text/language"
)

type Senario []interface{}

type Manager struct {
	Config *Config
	FSes map[string]fs.FS
	parser *KS
	Senario Senario
	Labels map[string]LabelInfo
	FontFace *text.GoTextFace
	Macros map[string]*Macro
	// CurrentStorage is the filename last passed to LoadScript, i.e. what
	// Senario/Labels currently reflect.
	CurrentStorage string
}

func (m *Manager) Init(fses map[string]fs.FS) {
	m.FSes = fses
	m.parser = &KS{}
	m.Config = &Config{}
	m.Config.LoadDefault()
	m.Config.Load(m.FSes["resources"], "config.toml")
	m.Macros = make(map[string]*Macro)
	m.loadDefaultFont()
}

func (m *Manager) LoadFirstScript() error {
	return m.LoadScript("first.ks")
}

func (m *Manager) LoadScript(file string) (err error) {
	var script []byte
	script, err = fs.ReadFile(m.FSes["senarios"], file)
	if err != nil {
		return
	}

	m.Senario, m.Labels, err = m.parser.ParseScenario(string(script))
	if err != nil {
		return
	}
	m.Senario = extractMacros(m.Senario, m.Macros)
	m.Labels = reindexLabels(m.Senario)
	m.CurrentStorage = file
	return nil
}

func (m *Manager) loadDefaultFont() {
	b, err := fs.ReadFile(Fonts, "NotoSansJP-Regular.ttf")
	if err != nil {
		panic(err)
	}
	s, err := text.NewGoTextFaceSource(bytes.NewReader(b))
	if err != nil {
		panic(err)
	}
	m.FontFace = &text.GoTextFace{
		Source: s,
		Size: float64(m.Config.DefaultFontSize),
		Language: language.Japanese,
	}
}
