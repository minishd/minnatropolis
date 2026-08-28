package filters

import (
	"encoding/json"
	"fmt"
	"iter"
	"log"
	"maps"
	"os"
	"slices"
	"strconv"
	"strings"
)

// Assets the game ships so clients can't reference things
// don't exist.
type Filters struct {
	// Generated from index..
	sprites map[string]struct{}
	systems map[string]struct{}
	maps    map[int32]struct{}

	// Manually specified..
	pictureNames    []string
	picturePrefixes []string
	battleAnimIDs   []int32
}

// Check if a set contains a key.
func has[T comparable](set map[T]struct{}, name T) (ok bool) {
	_, ok = set[name]
	return
}

func (f *Filters) HasSprite(name string) bool { return has(f.sprites, name) }
func (f *Filters) HasSystem(name string) bool { return has(f.systems, name) }

func (f *Filters) HasPicture(name string) bool {
	// Check if it's specified directly in names list first
	if slices.Contains(f.pictureNames, name) {
		// It is
		return true
	}

	// It wasn't, so let's check if it matches any prefixes..
	for _, prefix := range f.picturePrefixes {
		if strings.HasPrefix(name, prefix) {
			// It matches
			return true
		}
	}

	// No match
	return false
}
func (f *Filters) HasBattleAnimID(id int32) bool { return slices.Contains(f.battleAnimIDs, id) }

func (f *Filters) GetMaps() iter.Seq[int32]     { return maps.Keys(f.maps) }
func (f *Filters) GetPictureNames() []string    { return f.pictureNames }
func (f *Filters) GetPicturePrefixes() []string { return f.picturePrefixes }
func (f *Filters) GetBattleAnimIDs() []int32    { return f.battleAnimIDs }

// Parts of index.json root we care about
type assetIndex struct {
	Cache json.RawMessage `json:"cache"`
}

// Dirs of index.json cache we care about
type cacheDirs struct {
	CharSet map[string]any `json:"charset"`
	System  map[string]any `json:"system"`
}

// Strip a set down to just its keys
func strip(set map[string]any) map[string]struct{} {
	out := make(map[string]struct{})
	for name, _ := range set {
		out[name] = struct{}{}
	}
	return out
}

// Parse out map IDs from a list of file
// and folder names
func getMaps(set map[string]any) map[int32]struct{} {
	out := make(map[int32]struct{})
	for name, _ := range set {
		// Try to cut "map" prefix from name (e.g. "map0001.lmu")
		idAndExt, found := strings.CutPrefix(name, "map")
		if !found {
			continue
		}

		// Split off extension
		idStr, ext, found := strings.Cut(idAndExt, ".")
		if !found {
			continue
		}
		if ext != "lmu" && ext != "emu" {
			// not a map?
			continue
		}

		// Int-ify the ID
		id_, err := strconv.Atoi(idStr)
		if err != nil {
			// messed up ID
			continue
		}
		id := int32(id_)

		// Add to map
		out[id] = struct{}{}
	}
	return out
}

// Reads index.json produced by gencache.
func Load(path string, battleAnimIDs []int32, pictureNames []string, picturePrefixes []string) (*Filters, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read asset index: %w", err)
	}

	var idx assetIndex
	if err := json.Unmarshal(data, &idx); err != nil {
		return nil, fmt.Errorf("parsing asset index: %w", err)
	}

	// Get flat list of file/dir names
	// so we can pick out all of the map files
	var all map[string]any
	if err := json.Unmarshal(idx.Cache, &all); err != nil {
		return nil, fmt.Errorf("parsing cache filenames: %w", err)
	}

	// Get more structured list of file names
	// in asset directories
	var dirs cacheDirs
	if err := json.Unmarshal(idx.Cache, &dirs); err != nil {
		return nil, fmt.Errorf("parsing cache dirs: %w", err)
	}

	sprites := strip(dirs.CharSet)
	systems := strip(dirs.System)

	maps := getMaps(all)

	log.Printf(
		"asset index: %d sprites, %d systems, %d maps",
		len(sprites), len(systems), len(maps),
	)
	log.Printf(
		"filter config: %d battle anims, %d pictures, %d picture prefixes",
		len(battleAnimIDs), len(pictureNames), len(picturePrefixes),
	)
	return &Filters{
		sprites: sprites,
		systems: systems,
		maps:    maps,

		pictureNames:    pictureNames,
		picturePrefixes: picturePrefixes,
		battleAnimIDs:   battleAnimIDs,
	}, nil
}
