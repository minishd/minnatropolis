package main

import (
	"errors"
	"os"
	"slices"
	"unicode/utf8"

	"github.com/goccy/go-yaml"
	"github.com/minishd/minnatropolis/textfilter"
)

// another filter config idea
type textFilterPhrases struct {
	Fuzzy   []string
	Regexes []string
}

type textFilterCategory struct {
	Standins map[string][]string
	Strings  []string
	Words    []string
	// add regexes here after
	// or make it so both strings and words can use regex
}

var (
	ErrBadSizeReal   = errors.New("incorrectly sized 'real' letter")
	ErrBadSizeFake   = errors.New("incorrectly sized 'fake' letter")
	ErrDuplicateFake = errors.New("duplicate 'fake' letter")

	ErrNoDBConnect = errors.New("no db connect URI was specified")
	ErrNoIndexPath = errors.New("index path wasn't specified")
)

type configFile struct {
	DBConnect string
	ListenOn  string

	GuardPSK string
	Filter   struct {
		IndexPath         string
		SingleplayerRooms []int32
		PictureNames      []string
		PicturePrefixes   []string
		BattleAnimIDs     []int32
	}
	Unconscious bool

	TextFilter struct {
		Chat      *textFilterCategory
		Usernames *textFilterCategory
	}
}

func initTextFilter(cat *textFilterCategory) (*textfilter.TextFilter, error) {
	// First, change the standin [string]s into [rune]s
	standins := make(map[rune][]rune)
	for real, fakes := range cat.Standins {
		realR, size := utf8.DecodeRuneInString(real)
		if size != len(real) {
			return nil, ErrBadSizeReal
		}

		for _, fake := range fakes {
			fakeR, size := utf8.DecodeRuneInString(fake)
			if size != len(fake) {
				return nil, ErrBadSizeFake
			}

			curFakes := standins[realR]
			if slices.Contains(curFakes, fakeR) {
				return nil, ErrDuplicateFake
			}

			standins[realR] = append(standins[realR], fakeR)
		}
	}

	// Maybe, check for uppercase letters
	// or duplicate words?

	// Then, construct the [textfilter.TextFilter]
	return textfilter.New(standins, cat.Strings, cat.Words), nil
}

func loadConfig(path string) (*configFile, error) {
	// Read and parse data
	configData, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg configFile
	if err := yaml.Unmarshal(configData, &cfg); err != nil {
		return nil, err
	}

	// Check values
	if cfg.DBConnect == "" {
		return nil, ErrNoDBConnect
	}
	if cfg.Filter.IndexPath == "" {
		return nil, ErrNoIndexPath
	}

	// Seems OK, return it
	return &cfg, nil
}
