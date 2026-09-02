// [chara_new_psd]: register a character whose "立ち絵"(standing image)
// faces are generated from a single layered PSD file plus a
// PSDToolFavorites (.pfv) preset list, using github.com/oov/psd (PSD layer
// decoding), github.com/raa0121/pfv (.pfv preset parsing) and
// github.com/raa0121/ppi (compositing a preset's selected layers into one
// flat image). No equivalent in real TyranoScript — a kag3-only extension
// for artists who distribute character art as a single multi-expression
// PSD (the common convention alongside PSDToolKit-style favorites files)
// instead of one PNG per expression.
package ebitengine

import (
	"bytes"
	"fmt"
	"io/fs"
	"strings"

	"github.com/ebinovel/kag3"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/oov/psd"
	"github.com/raa0121/pfv"
	"github.com/raa0121/ppi"
)

func init() {
	register("chara_new_psd", handleCharaNewPSD)
}

// psdFaceStorageSentinel/Sep build a self-describing synthetic "storage"
// string (psdPath, pfvPath, encoding, presetName) that loadImage recognizes
// and regenerates from scratch instead of treating as a real file path.
// This deliberately follows the same "Storage is enough to re-derive the
// image" contract kag3.Character.Storage's own doc comment describes for
// ordinary files: a save/load resumed in a fresh process (see
// reconcileViewCharas, save_apply.go) never re-runs [chara_new_psd] itself,
// so charas[name].Faces values (also captured verbatim by
// saveData.CharaFaces) must be able to stand on their own without it. The
// leading NUL byte can never appear in a real TyranoScript storage=
// attribute or any real filesystem path, so this can never collide with an
// actual image file; it round-trips fine through the JSON save format too
// (a plain control byte inside an otherwise-ordinary UTF-8 string).
const (
	psdFaceStorageSentinel = "\x00kag3:psdface"
	psdFaceStorageSep      = "\x1f"
)

func encodePSDFaceStorage(psdPath, pfvPath, encoding, presetName string) string {
	return psdFaceStorageSentinel + psdFaceStorageSep + psdPath + psdFaceStorageSep + pfvPath + psdFaceStorageSep + encoding + psdFaceStorageSep + presetName
}

func decodePSDFaceStorage(storage string) (psdPath, pfvPath, encoding, presetName string, ok bool) {
	prefix := psdFaceStorageSentinel + psdFaceStorageSep
	if !strings.HasPrefix(storage, prefix) {
		return "", "", "", "", false
	}
	parts := strings.SplitN(strings.TrimPrefix(storage, prefix), psdFaceStorageSep, 4)
	if len(parts) != 4 {
		return "", "", "", "", false
	}
	return parts[0], parts[1], parts[2], parts[3], true
}

// psdFaceGroup is every preset a single (psdPath, pfvPath, encoding)
// combination produces, generated once by ppi.CreateImage and cached —
// decoding a PSD and compositing every one of its presets is the expensive
// part (see loadPSDFaceGroup), and a character typically has many faces
// sharing the same source PSD/.pfv, each triggering its own loadImage call.
type psdFaceGroup struct {
	// Order preserves the .pfv file's own preset order (pfv.Pfv.Items,
	// append-only) — ppi.CreateImage's own return slice order is NOT
	// reliable for this, since it builds its result from a map internally.
	Order []string
	Faces map[string]*ebiten.Image
}

var psdFaceGroups = map[string]*psdFaceGroup{}

// loadPSDFaceGroup decodes psdPath/pfvPath (both resolved the same way an
// ordinary chara storage= path is, via resolveFolderImage) and composites
// every preset the .pfv file defines, caching the whole group under a key
// combining all three inputs so a character with many PSD-derived faces
// only pays the decode+composite cost once. Safe to call repeatedly,
// including from a fresh process after a save/load (see
// psdFaceStorageSentinel's doc comment), since it never depends on
// [chara_new_psd] having actually run.
func loadPSDFaceGroup(r *Renderer, psdPath, pfvPath, encoding string) (*psdFaceGroup, error) {
	cacheKey := psdPath + psdFaceStorageSep + pfvPath + psdFaceStorageSep + encoding
	if g, ok := psdFaceGroups[cacheKey]; ok {
		return g, nil
	}

	psdFS, psdName := resolveFolderImage(r, "", psdPath)
	psdBytes, err := fs.ReadFile(psdFS, psdName)
	if err != nil {
		return nil, fmt.Errorf("PSDファイル %s の読み込みに失敗しました: %w", psdPath, err)
	}
	psdImg, _, err := psd.Decode(bytes.NewReader(psdBytes), &psd.DecodeOptions{})
	if err != nil {
		return nil, fmt.Errorf("PSDファイル %s の解析に失敗しました: %w", psdPath, err)
	}

	pfvFS, pfvName := resolveFolderImage(r, "", pfvPath)
	pfvBytes, err := fs.ReadFile(pfvFS, pfvName)
	if err != nil {
		return nil, fmt.Errorf("PSDToolFavoritesファイル %s の読み込みに失敗しました: %w", pfvPath, err)
	}
	pfvConf, err := decodePfv(string(pfvBytes))
	if err != nil {
		return nil, fmt.Errorf("PSDToolFavoritesファイル %s の解析に失敗しました: %w", pfvPath, err)
	}

	images, err := ppi.CreateImage(psdImg, pfvConf, encoding)
	if err != nil {
		return nil, fmt.Errorf("PSD %s とPSDToolFavorites %s の合成に失敗しました: %w", psdPath, pfvPath, err)
	}

	order := make([]string, 0, len(pfvConf.Items))
	for _, item := range pfvConf.Items {
		order = append(order, item.Name)
	}
	faces := make(map[string]*ebiten.Image, len(images))
	for _, img := range images {
		// ppi.CreateImage silently returns a degenerate 0x0 image for any
		// preset whose favorite= entries matched no PSD layer at all
		// (rather than an error) — almost always either encoding= being
		// wrong for this PSD (see the field's own doc comment) or a
		// favorite= preset referencing a layer path this PSD doesn't have.
		// Surfacing that here, rather than letting a 0x0 *ebiten.Image
		// silently reach drawCharacters later, saves a very confusing
		// "character is just invisible" report.
		b := img.Image.Bounds()
		if b.Dx() == 0 || b.Dy() == 0 {
			fmt.Printf("chara_new_psd: プリセット %s の合成結果が空でした(favorite= の項目名がPSDのレイヤー名と一致しているか、encoding= の指定が正しいか確認してください)\n", img.Name)
			continue
		}
		faces[img.Name] = ebiten.NewImageFromImage(img.Image)
	}

	g := &psdFaceGroup{Order: order, Faces: faces}
	psdFaceGroups[cacheKey] = g
	return g, nil
}

// decodePfv wraps pfv.Decode, which panics (a bare panic(err), not a
// returned error) on a malformed "faview-mode/" line — a third-party
// parser's own failure mode having nothing to do with kag3's usual "storage
// doesn't exist" error convention shouldn't be allowed to take down the
// whole renderer the way an unguarded panic would — see runSpeechJob
// (tags_speech.go) for the same defensive shape against a different
// external call.
func decodePfv(s string) (conf *pfv.Pfv, err error) {
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("pfv.Decode panicked: %v", p)
		}
	}()
	return pfv.Decode(s), nil
}

// loadPSDFace backs loadImage's psdFaceStorageSentinel branch: decode
// storage back into its four parts, load (or reuse the cached)
// psdFaceGroup, and return the one preset asked for.
func loadPSDFace(r *Renderer, storage string) (*ebiten.Image, error) {
	psdPath, pfvPath, encoding, presetName, ok := decodePSDFaceStorage(storage)
	if !ok {
		return nil, fmt.Errorf("不正なPSD立ち絵storageです: %q", storage)
	}
	g, err := loadPSDFaceGroup(r, psdPath, pfvPath, encoding)
	if err != nil {
		return nil, err
	}
	img, ok := g.Faces[presetName]
	if !ok {
		return nil, fmt.Errorf("PSD %s / favorite %s にプリセット %q が見つかりません", psdPath, pfvPath, presetName)
	}
	return img, nil
}

// handleCharaNewPSD implements
// [chara_new_psd name= storage= favorite= face= jname= encoding=].
// storage= is a .psd file, favorite= is a PSDToolFavorites .pfv file
// listing named presets (each a set of layers to composite together) —
// every preset becomes a face registered under its own name in
// charas[name].Faces, exactly as if [chara_face] had been called once per
// preset, so the ordinary [chara_show face=]/[chara_mod face=] tags work
// completely unmodified against a PSD-driven character. face= picks which
// preset is showing immediately (defaults to the .pfv file's first preset,
// in file order, if omitted or not found). encoding= is the PSD layer name
// encoding ppi.CreateImage needs in order to match layer names against
// favorite='s preset definitions — "utf-8" (the default, meaning no
// conversion) covers modern Photoshop's Unicode layer names; older/legacy
// PSDs lacking them typically need "sjis" (see docs/COMPATIBILITY.md, and
// github.com/raa0121/ppi's own README example, which uses exactly that
// value).
func handleCharaNewPSD(ctx *tagCtx) error {
	r := ctx.r
	pm := ctx.tag.Pm
	name, ok := getString(pm, "name")
	if !ok || name == "" {
		return fmt.Errorf("[chara_new_psd] には name= が必要です")
	}
	storage, ok := getString(pm, "storage")
	if !ok || storage == "" {
		return fmt.Errorf("[chara_new_psd] には storage= (PSDファイル) が必要です")
	}
	favorite, ok := getString(pm, "favorite")
	if !ok || favorite == "" {
		return fmt.Errorf("[chara_new_psd] には favorite= (PSDToolFavoritesファイル) が必要です")
	}
	encoding, ok := getString(pm, "encoding")
	if !ok || encoding == "" {
		encoding = "utf-8"
	}

	g, err := loadPSDFaceGroup(r, storage, favorite, encoding)
	if err != nil {
		return err
	}
	if len(g.Order) == 0 {
		return fmt.Errorf("[chara_new_psd] %s / %s にはプリセットが1つもありません", storage, favorite)
	}

	initialFace, ok := getString(pm, "face")
	if !ok || initialFace == "" {
		initialFace = g.Order[0]
	}
	img, ok := g.Faces[initialFace]
	if !ok {
		return fmt.Errorf("[chara_new_psd] プリセット %q が見つかりません(favorite=のプリセットは %v)", initialFace, g.Order)
	}

	faces := make(map[string]string, len(g.Faces)+1)
	for presetName := range g.Faces {
		faces[presetName] = encodePSDFaceStorage(storage, favorite, encoding, presetName)
	}
	faces["default"] = encodePSDFaceStorage(storage, favorite, encoding, initialFace)

	charas[name] = &kag3.Character{
		Name:    name,
		JName:   pm["jname"],
		Image:   img,
		Faces:   faces,
		Storage: faces["default"],
	}
	return nil
}
