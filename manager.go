package kag3

import (
	"bytes"
	"io/fs"

	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"golang.org/x/text/language"
)

type Senario []interface{}

type Manager struct {
	Config   *Config
	FSes     map[string]fs.FS
	parser   *KS
	Senario  Senario
	Labels   map[string]LabelInfo
	FontFace *text.GoTextFace
	// VerticalFontFace shares FontFace's Source but has the OpenType "vert"
	// feature enabled, so glyphs substitute to their vertical-writing forms
	// (punctuation moves to the upper-right of its cell, long vowel marks
	// rotate, etc.) — the same effect Windows' "@"-prefixed vertical font
	// names have, without needing a separate font file. See
	// renderer/ebitengine/renderer.go's Vertical text drawing branch.
	VerticalFontFace *text.GoTextFace
	Macros           map[string]*Macro
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
		Source:   s,
		Size:     float64(m.Config.DefaultFontSize),
		Language: language.Japanese,
	}
	m.VerticalFontFace = &text.GoTextFace{
		Source:   s,
		Size:     float64(m.Config.DefaultFontSize),
		Language: language.Japanese,
	}
	m.VerticalFontFace.SetFeature(text.MustParseTag("vert"), 1)
}
