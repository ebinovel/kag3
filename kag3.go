package kag3

type KS struct {
	isInScript bool
	ifCount int
}

type TextObject struct {
	Name string
	Line int
	Chara CharacterInfo
	Val string
}

type LabelObject struct {
	Name string
	Val string
	Info LabelInfo
}

type TagObject struct {
	Name string
	Line int
	Pm map[string]string
	Val string
	ifCount int
}

type LabelInfo struct {
	Line int
	index int
	Name string
	Val string
}

type CharacterInfo struct {
	Name string
	Face string
}
