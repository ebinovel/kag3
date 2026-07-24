//go:build windows

package driver

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/png"
)

// Screenshot captures the attached window's client area as WinAppDriver
// sees it (base64 PNG over GET /session/{id}/screenshot).
func (s *Session) Screenshot() (image.Image, error) {
	var resp struct {
		Value string `json:"value"`
	}
	if err := s.get(fmt.Sprintf("/session/%s/screenshot", s.ID), &resp); err != nil {
		return nil, fmt.Errorf("screenshot: %w", err)
	}
	data, err := base64.StdEncoding.DecodeString(resp.Value)
	if err != nil {
		return nil, fmt.Errorf("decoding screenshot base64: %w", err)
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("decoding screenshot PNG: %w", err)
	}
	return img, nil
}
