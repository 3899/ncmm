// Copyright (c) 2026 @3899. All rights reserved.
// Use of this source code is governed by a MIT-style license that can be found in the LICENSE file.

package tag

type WAV struct {
}

func NewWAV(path string) (*WAV, error) {
	wav := WAV{}
	return &wav, nil
}

func (m *WAV) SetCover(buf []byte, mime string) error {
	return nil
}

func (m *WAV) SetCoverUrl(coverUrl string) error {
	return nil
}

func (m *WAV) SetTitle(title string) error {
	return nil
}

func (m *WAV) SetAlbum(album string) error {
	return nil
}

func (m *WAV) SetArtist(artists []string) error {
	return nil
}

func (m *WAV) SetComment(comment string) error {
	return nil
}

func (m *WAV) Save() error {
	return nil
}
