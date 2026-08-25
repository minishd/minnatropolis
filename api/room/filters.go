package room

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
)

// Assets the game ships so clients can't reference things
// don't exist.
type Filters struct {
	sprites map[string]struct{}
	systems map[string]struct{}
}

func (f *Filters) HasSprite(name string) bool { return has(f.sprites, name) }
func (f *Filters) HasSystem(name string) bool { return has(f.systems, name) }

// Entries of index.json we care about (only one atm.)
type assetIndex struct {
	Cache map[string]any `json:"cache"`
}

// Reads index.json produced by gencache.
func LoadFilters(path string) (*Filters, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read asset index: %w", err)
	}

	var idx assetIndex
	if err := json.Unmarshal(data, &idx); err != nil {
		return nil, fmt.Errorf("parsing asset index: %w", err)
	}

	sprites, err := namesFor(idx.Cache, "CharSet")
	if err != nil {
		return nil, err
	}
	systems, err := namesFor(idx.Cache, "System")
	if err != nil {
		return nil, err
	}

	log.Printf("asset index: %d sprites, %d systems", len(sprites), len(systems))
	return &Filters{sprites: sprites, systems: systems}, nil
}

// Collects the asset names from the directory's subtree.
func namesFor(cache map[string]any, dirName string) (map[string]struct{}, error) {
	for _, value := range cache {
		// The root-level game files sit alongside directories here
		entries, isDir := value.(map[string]any)
		if !isDir {
			continue
		}

		dn, _ := entries["_dirname"].(string)
		if !strings.EqualFold(dn, dirName) {
			continue
		}

		names := make(map[string]struct{}, len(entries))
		for name, entry := range entries {
			if name == "_dirname" {
				continue
			}
			if _, isFile := entry.(string); !isFile {
				continue
			}
			names[strings.ToLower(name)] = struct{}{}
		}
		return names, nil
	}

	return nil, fmt.Errorf("no %q directory in asset index", dirName)
}

func has(set map[string]struct{}, name string) bool {
	_, ok := set[strings.ToLower(name)]
	return ok
}
