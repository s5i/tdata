package npc

import (
	"bufio"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/transform"
)

type Data struct {
	NPC       string
	Responses []string
}

var nameRE = regexp.MustCompile(`^\s*Name = "([^"]+)"`)
var includeRE = regexp.MustCompile(`^\s*@"([^"]+)"`)
var numberRE = regexp.MustCompile(`\d+`)

func ProcessDir(path string) ([]*Data, error) {
	var ret []*Data
	if err := filepath.WalkDir(path, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		if strings.ToLower(filepath.Ext(path)) != ".npc" {
			return nil
		}

		data, err := parseFile(path)
		if err != nil {
			return err
		}

		ret = append(ret, data)
		return nil
	}); err != nil {
		return nil, err
	}

	slices.SortFunc(ret, func(a, b *Data) int {
		return strings.Compare(a.NPC, b.NPC)
	})

	return ret, nil
}

func parseFile(path string) (*Data, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	r := transform.NewReader(f, charmap.ISO8859_1.NewDecoder())
	ret := &Data{}

	respMap := map[string]bool{}
	scanner := bufio.NewScanner(r)
	needsName := filepath.Ext(path) == ".npc"

	for scanner.Scan() {
		line := scanner.Text()
		if needsName && nameRE.MatchString(line) {
			needsName = false
			ret.NPC = nameRE.FindStringSubmatch(line)[1]
			continue
		}

		if strings.Contains(line, "->") {
			line = strings.Split(line, "->")[1]
			responses := strings.Split(line, `"`)
			if len(responses) == 1 {
				continue
			}

			for i := 1; i < len(responses); i += 2 {
				r := responses[i]
				r = numberRE.ReplaceAllString(r, "_")
				if strings.Contains(r, "%") {
					r = strings.ReplaceAll(r, "%N", `_`)
					r = strings.ReplaceAll(r, "%P", `_`)
					r = strings.ReplaceAll(r, "%A", `_`)
					r = strings.ReplaceAll(r, "%T", `_:_`)
				}
				respMap[r] = true
			}
		}

		if strings.HasPrefix(line, "@") {
			data, err := parseFile(filepath.Join(filepath.Dir(path), includeRE.FindStringSubmatch(line)[1]))
			if err != nil {
				return nil, err
			}
			for _, r := range data.Responses {
				respMap[r] = true
			}
		}
	}

	ret.Responses = slices.Sorted(maps.Keys(respMap))

	return ret, nil
}
