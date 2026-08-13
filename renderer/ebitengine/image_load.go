package ebitengine

import (
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io/fs"
	"path"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
)

// resolveFolderImage loads a [button]-style graphic against folder=,
// matching real Tyrano's convention that folder names a *subdirectory* of
// the images root (e.g. folder="bgimage" -> images/bgimage/…) rather than a
// separate top-level asset root. tyrano.ks's own bundled cg_image_button/
// replay_image_button macros also lean on a real-Tyrano-specific relative
// escape for their shared "no image" placeholder (folder="bgimage" plus a
// graphic of "../../tyrano/images/system/noimage.png") that plain
// fs.Sub(images, folder) can't resolve — Go's io/fs deliberately rejects any
// ".." path component. After path.Clean, if the joined path still tries to
// climb above the images root, this looks for a "system/" path component to
// redirect the remainder to r.fses["system/images"] (kag3's own equivalent
// of Tyrano's bundled engine-asset folder) — the one real asset this
// pattern needs, noimage.png, lives exactly there.
func resolveFolderImage(r *Renderer, folder, graphic string) (imgFS fs.FS, name string) {
	full := graphic
	if folder != "" && folder != "images" {
		full = path.Join(folder, graphic)
	}
	full = path.Clean(full)
	if full == ".." || strings.HasPrefix(full, "../") {
		if idx := strings.LastIndex(full, "system/"); idx >= 0 {
			return r.fses["system/images"], full[idx+len("system/"):]
		}
		full = strings.TrimPrefix(full, "../")
		for strings.HasPrefix(full, "../") {
			full = full[len("../"):]
		}
	}
	return r.fses["images"], full
}

// loadImage resolves folder/storage via resolveFolderImage and loads the
// result, folding the two-step "resolve fs.FS + path, then load" sequence
// that's repeated at every [button]/[image]/chara call site into one call.
// A storage carrying psdFaceStorageSentinel (tags_chara_psd.go) is not a
// real file path at all — it's a self-describing descriptor for a
// [chara_new_psd]-generated face, regenerated (or served from cache) by
// loadPSDFace instead of ever reaching resolveFolderImage/fs.FS.
func loadImage(r *Renderer, folder, storage string) (*ebiten.Image, error) {
	if strings.HasPrefix(storage, psdFaceStorageSentinel) {
		return loadPSDFace(r, storage)
	}
	imgFS, name := resolveFolderImage(r, folder, storage)
	img, _, err := ebitenutil.NewImageFromFileSystem(imgFS, name)
	if err != nil {
		return nil, err
	}
	return img, nil
}
