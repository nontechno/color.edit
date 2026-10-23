// Package spec defines the application-independent color.edit preview format.
package spec

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

const Version = 1

type Specification struct {
	Version  int               `json:"version"`
	Name     string            `json:"name"`
	Canvas   Canvas            `json:"canvas"`
	Colors   []ColorDefinition `json:"colors"`
	Editor   EditorStyle       `json:"editor"`
	Elements []Element         `json:"elements"`
}

type Canvas struct {
	Width      int    `json:"width"`
	Height     int    `json:"height"`
	Resize     string `json:"resize"`
	Background string `json:"background"`
}

type ColorDefinition struct {
	Path         string   `json:"path"`
	Label        string   `json:"label"`
	Description  string   `json:"description,omitempty"`
	FallbackPath string   `json:"fallback_path,omitempty"`
	Uses         []string `json:"uses,omitempty"`
}

// EditorStyle maps the generic editor's own chrome to assignment paths.
type EditorStyle struct {
	Background         string `json:"background"`
	Text               string `json:"text"`
	Border             string `json:"border"`
	Title              string `json:"title"`
	SelectedBackground string `json:"selected_background"`
	SelectedText       string `json:"selected_text"`
	SectionBackground  string `json:"section_background,omitempty"`
	SectionText        string `json:"section_text,omitempty"`
	StatusBackground   string `json:"status_background"`
	StatusText         string `json:"status_text"`
	StatusHelp         string `json:"status_help"`
	DialogBackground   string `json:"dialog_background"`
	DialogText         string `json:"dialog_text"`
	DialogBorder       string `json:"dialog_border"`
	DialogTitle        string `json:"dialog_title"`
	ButtonBackground   string `json:"button_background"`
	ButtonText         string `json:"button_text"`
	ButtonSelectedBG   string `json:"button_selected_background"`
	ButtonSelectedText string `json:"button_selected_text"`
}

// Element is positioned in logical Canvas cells. The renderer scales its
// rectangle to the current terminal size, making the preview resizable.
// Supported kinds are fill, frame, and text.
type Element struct {
	ID         string `json:"id"`
	Kind       string `json:"kind"`
	X          int    `json:"x"`
	Y          int    `json:"y"`
	Width      int    `json:"width"`
	Height     int    `json:"height"`
	Text       string `json:"text,omitempty"`
	Character  string `json:"character,omitempty"`
	Align      string `json:"align,omitempty"`
	Fill       bool   `json:"fill,omitempty"`
	Foreground string `json:"foreground,omitempty"`
	Background string `json:"background,omitempty"`
}

func Load(path string) (Specification, error) {
	file, err := os.Open(path)
	if err != nil {
		return Specification{}, fmt.Errorf("open preview specification: %w", err)
	}
	defer file.Close()
	decoder := json.NewDecoder(io.LimitReader(file, 4<<20))
	decoder.DisallowUnknownFields()
	var specification Specification
	if err := decoder.Decode(&specification); err != nil {
		return Specification{}, fmt.Errorf("decode preview specification: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			err = errors.New("multiple JSON values")
		}
		return Specification{}, fmt.Errorf("decode preview specification: %w", err)
	}
	if err := specification.Validate(); err != nil {
		return Specification{}, fmt.Errorf("validate preview specification: %w", err)
	}
	return specification, nil
}

func (s Specification) Validate() error {
	if s.Version != Version {
		return fmt.Errorf("unsupported version %d (expected %d)", s.Version, Version)
	}
	if strings.TrimSpace(s.Name) == "" {
		return errors.New("name must not be empty")
	}
	if s.Canvas.Width < 20 || s.Canvas.Width > 500 || s.Canvas.Height < 8 || s.Canvas.Height > 200 {
		return fmt.Errorf("canvas dimensions %dx%d are outside 20..500 by 8..200", s.Canvas.Width, s.Canvas.Height)
	}
	if s.Canvas.Resize != "scale" && s.Canvas.Resize != "clip" {
		return fmt.Errorf("canvas.resize %q must be scale or clip", s.Canvas.Resize)
	}
	definitions := make(map[string]ColorDefinition, len(s.Colors))
	for index, color := range s.Colors {
		if !validPath(color.Path) {
			return fmt.Errorf("colors[%d].path %q is invalid", index, color.Path)
		}
		if strings.TrimSpace(color.Label) == "" {
			return fmt.Errorf("colors[%d].label must not be empty", index)
		}
		if _, duplicate := definitions[color.Path]; duplicate {
			return fmt.Errorf("duplicate color path %q", color.Path)
		}
		definitions[color.Path] = color
	}
	if len(definitions) == 0 {
		return errors.New("at least one color must be defined")
	}
	for _, color := range s.Colors {
		if color.FallbackPath != "" {
			if _, exists := definitions[color.FallbackPath]; !exists {
				return fmt.Errorf("color %q refers to unknown fallback %q", color.Path, color.FallbackPath)
			}
			if color.FallbackPath == color.Path {
				return fmt.Errorf("color %q falls back to itself", color.Path)
			}
		}
	}
	checkedFallbacks := make(map[string]bool, len(definitions))
	var checkFallback func(string, map[string]bool) error
	checkFallback = func(path string, visiting map[string]bool) error {
		if checkedFallbacks[path] {
			return nil
		}
		if visiting[path] {
			return fmt.Errorf("fallback cycle includes %q", path)
		}
		visiting[path] = true
		if fallback := definitions[path].FallbackPath; fallback != "" {
			if err := checkFallback(fallback, visiting); err != nil {
				return err
			}
		}
		delete(visiting, path)
		checkedFallbacks[path] = true
		return nil
	}
	for path := range definitions {
		if err := checkFallback(path, make(map[string]bool)); err != nil {
			return err
		}
	}
	if s.Canvas.Background != "" {
		if _, exists := definitions[s.Canvas.Background]; !exists {
			return fmt.Errorf("canvas.background refers to unknown color %q", s.Canvas.Background)
		}
	}
	used := make(map[string]bool, len(definitions))
	ids := make(map[string]bool, len(s.Elements))
	for index, element := range s.Elements {
		if strings.TrimSpace(element.ID) == "" || ids[element.ID] {
			return fmt.Errorf("elements[%d].id %q is empty or duplicated", index, element.ID)
		}
		ids[element.ID] = true
		if element.Kind != "fill" && element.Kind != "frame" && element.Kind != "text" {
			return fmt.Errorf("element %q has unsupported kind %q", element.ID, element.Kind)
		}
		if element.X < 0 || element.Y < 0 || element.Width < 1 || element.Height < 1 ||
			element.X+element.Width > s.Canvas.Width || element.Y+element.Height > s.Canvas.Height {
			return fmt.Errorf("element %q rectangle is outside the canvas", element.ID)
		}
		if element.Align != "" && element.Align != "left" && element.Align != "center" && element.Align != "right" {
			return fmt.Errorf("element %q has invalid alignment %q", element.ID, element.Align)
		}
		for property, path := range map[string]string{"foreground": element.Foreground, "background": element.Background} {
			if path == "" {
				continue
			}
			if _, exists := definitions[path]; !exists {
				return fmt.Errorf("element %q %s refers to unknown color %q", element.ID, property, path)
			}
			used[path] = true
		}
	}
	for property, path := range editorPaths(s.Editor) {
		if path == "" {
			continue
		}
		if _, exists := definitions[path]; !exists {
			return fmt.Errorf("editor.%s refers to unknown color %q", property, path)
		}
		// Editor chrome is visible alongside the application preview. A color
		// demonstrated by the color list, status, or dialogs does not need an
		// artificial duplicate inside the application's preview canvas.
		used[path] = true
	}
	for _, color := range s.Colors {
		if !used[color.Path] {
			return fmt.Errorf("color %q is not visibly used by any preview element or editor binding", color.Path)
		}
		for _, elementID := range color.Uses {
			if !ids[elementID] {
				return fmt.Errorf("color %q lists unknown preview use %q", color.Path, elementID)
			}
			bound := false
			for _, element := range s.Elements {
				if element.ID == elementID && (element.Foreground == color.Path || element.Background == color.Path) {
					bound = true
					break
				}
			}
			if !bound {
				return fmt.Errorf("color %q lists preview use %q which does not bind that color", color.Path, elementID)
			}
		}
	}
	return nil
}

func editorPaths(style EditorStyle) map[string]string {
	return map[string]string{
		"background": style.Background, "text": style.Text, "border": style.Border, "title": style.Title,
		"selected_background": style.SelectedBackground, "selected_text": style.SelectedText,
		"section_background": style.SectionBackground, "section_text": style.SectionText,
		"status_background": style.StatusBackground, "status_text": style.StatusText, "status_help": style.StatusHelp,
		"dialog_background": style.DialogBackground, "dialog_text": style.DialogText, "dialog_border": style.DialogBorder,
		"dialog_title": style.DialogTitle, "button_background": style.ButtonBackground, "button_text": style.ButtonText,
		"button_selected_background": style.ButtonSelectedBG, "button_selected_text": style.ButtonSelectedText,
	}
}

func validPath(path string) bool {
	parts := strings.Split(path, ".")
	if len(parts) == 0 {
		return false
	}
	for _, part := range parts {
		if strings.TrimSpace(part) == "" || part != strings.TrimSpace(part) {
			return false
		}
		for _, character := range part {
			if (character < 'a' || character > 'z') && (character < 'A' || character > 'Z') &&
				(character < '0' || character > '9') && character != '_' && character != '-' {
				return false
			}
		}
	}
	return true
}
