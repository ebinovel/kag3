package main

import (
	"embed"
	"io/fs"
	"log"
)

//go:embed resources
var rawEmbed embed.FS

var (
	Embed    fs.FS
	Bgms     fs.FS
	Fonts    fs.FS
	Images   fs.FS
	Senarios fs.FS
	Ses      fs.FS
)

func resourcesInit() {
	var err error
	Embed, err = fs.Sub(rawEmbed, "resources")
	if err != nil {
		log.Fatal(err)
	}
	Bgms, err = fs.Sub(Embed, "bgms")
	if err != nil {
		log.Fatal(err)
	}
	Fonts, err = fs.Sub(Embed, "fonts")
	if err != nil {
		log.Fatal(err)
	}
	Images, err = fs.Sub(Embed, "images")
	if err != nil {
		log.Fatal(err)
	}
	Senarios, err = fs.Sub(Embed, "senarios")
	if err != nil {
		log.Fatal(err)
	}
	Ses, err = fs.Sub(Embed, "ses")
	if err != nil {
		log.Fatal(err)
	}
}
