package kag3

import (
	"embed"
	"io/fs"
	"log"
	"path"
)

//go:embed resources
var rawEmbed embed.FS

var (
	Embed    fs.FS
	Fonts    fs.FS
	Images   fs.FS
)

func init () {
	var err error
	Embed, err = fs.Sub(rawEmbed, "resources")
	if err != nil {
		log.Fatal(err)
	}
	Fonts, err = fs.Sub(Embed, path.Join("system", "fonts"))
	if err != nil {
		log.Fatal(err)
	}
	Images, err = fs.Sub(Embed, path.Join("system", "images"))
	if err != nil {
		log.Fatal(err)
	}
}
