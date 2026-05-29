package mapfile

import (
	"bufio"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/transform"
)

type Sector struct {
	SectorX, SectorY, SectorZ int
	Tiles                     []Tile
}

type Tile struct {
	Pos            Pos
	Refresh        bool
	ProtectionZone bool
	Items          []Item
}

type Item struct {
	ID                  int
	String              string
	KeyholeNumber       int
	KeyNumber           int
	ChestQuestNumber    int
	DoorQuestNumber     int
	DoorQuestValue      int
	Level               int
	Amount              int
	Charges             int
	RemainingUses       int
	PoolLiquidType      int
	ContainerLiquidType int
	RemainingExpireTime int
	SavedExpireTime     int
	AbsTeleportDest     int
	Content             []Item
}

type Pos struct {
	X, Y, Z int
}

func (p Pos) String() string {
	return fmt.Sprintf("(%d, %d, %d)", p.X, p.Y, p.Z)
}

func ProcessDir(dir string) ([]Sector, error) {
	var sectors []Sector
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if strings.ToLower(filepath.Ext(path)) != ".sec" {
			return nil
		}
		sec, err := ParseFile(path)
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		sectors = append(sectors, *sec)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return sectors, nil
}

func ParseFile(path string) (*Sector, error) {
	sx, sy, sz, err := parseSectorFilename(filepath.Base(path))
	if err != nil {
		return nil, err
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	r := transform.NewReader(f, charmap.ISO8859_1.NewDecoder())

	sec := &Sector{SectorX: sx, SectorY: sy, SectorZ: sz}
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		tile, err := parseTileLine(line, sx, sy, sz)
		if err != nil {
			return nil, fmt.Errorf("line %q: %w", truncate(line, 120), err)
		}
		sec.Tiles = append(sec.Tiles, tile)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return sec, nil
}

func parseSectorFilename(name string) (int, int, int, error) {
	name = strings.TrimSuffix(name, filepath.Ext(name))
	parts := strings.Split(name, "-")
	if len(parts) != 3 {
		return 0, 0, 0, fmt.Errorf("unexpected filename format: %s", name)
	}
	x, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, 0, err
	}
	y, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, 0, err
	}
	z, err := strconv.Atoi(parts[2])
	if err != nil {
		return 0, 0, 0, err
	}
	return x, y, z, nil
}

func parseTileLine(line string, sx, sy, sz int) (Tile, error) {
	colonIdx := strings.Index(line, ":")
	if colonIdx < 0 {
		return Tile{}, fmt.Errorf("no colon found")
	}
	coords := line[:colonIdx]
	rest := strings.TrimSpace(line[colonIdx+1:])

	dashIdx := strings.Index(coords, "-")
	if dashIdx < 0 {
		return Tile{}, fmt.Errorf("no dash in coordinates %q", coords)
	}

	row, err := strconv.Atoi(strings.TrimSpace(coords[:dashIdx]))
	if err != nil {
		return Tile{}, fmt.Errorf("bad row in %q: %w", coords, err)
	}
	col, err := strconv.Atoi(strings.TrimSpace(coords[dashIdx+1:]))
	if err != nil {
		return Tile{}, fmt.Errorf("bad col in %q: %w", coords, err)
	}

	tile := Tile{
		Pos: Pos{
			X: sx*32 + col,
			Y: sy*32 + row,
			Z: sz,
		},
	}

	if err := parseTileBody(rest, &tile); err != nil {
		return Tile{}, err
	}

	return tile, nil
}

func parseTileBody(s string, tile *Tile) error {
	for {
		s = strings.TrimSpace(s)
		if s == "" {
			break
		}

		if strings.HasPrefix(s, "Content=") {
			s = s[len("Content="):]
			items, _, err := parseItemList(s)
			if err != nil {
				return fmt.Errorf("parsing Content: %w", err)
			}
			tile.Items = items
			break
		}

		if strings.HasPrefix(s, "Refresh") {
			tile.Refresh = true
			s = s[len("Refresh"):]
			s = strings.TrimLeft(s, ", ")
			continue
		}
		if strings.HasPrefix(s, "ProtectionZone") {
			tile.ProtectionZone = true
			s = s[len("ProtectionZone"):]
			s = strings.TrimLeft(s, ", ")
			continue
		}

		idx := strings.Index(s, ",")
		cidx := strings.Index(s, "Content=")
		if cidx >= 0 && (idx < 0 || cidx < idx) {
			s = s[cidx:]
			continue
		}
		if idx >= 0 {
			s = s[idx+1:]
			continue
		}
		break
	}
	return nil
}

func parseItemList(s string) ([]Item, string, error) {
	s = strings.TrimSpace(s)
	if len(s) == 0 || s[0] != '{' {
		return nil, s, fmt.Errorf("expected '{', got %q", truncate(s, 30))
	}
	s = s[1:] // skip '{'

	var items []Item

	for {
		s = strings.TrimSpace(s)
		if len(s) == 0 {
			return nil, "", fmt.Errorf("unexpected end of item list")
		}
		if s[0] == '}' {
			return items, s[1:], nil
		}

		item, rest, err := parseItem(s)
		if err != nil {
			return nil, "", err
		}
		items = append(items, item)
		s = strings.TrimSpace(rest)

		if len(s) > 0 && s[0] == ',' {
			s = s[1:]
		}
	}
}

func parseItem(s string) (Item, string, error) {
	s = strings.TrimSpace(s)

	idEnd := 0
	for idEnd < len(s) && s[idEnd] >= '0' && s[idEnd] <= '9' {
		idEnd++
	}
	if idEnd == 0 {
		return Item{}, s, fmt.Errorf("expected item ID, got %q", truncate(s, 30))
	}

	id, err := strconv.Atoi(s[:idEnd])
	if err != nil {
		return Item{}, s, err
	}
	item := Item{ID: id}
	s = s[idEnd:]

	for {
		s = strings.TrimSpace(s)
		if len(s) == 0 || s[0] == ',' || s[0] == '}' {
			break
		}

		var consumed bool
		s, consumed, err = tryParseAttr(s, &item)
		if err != nil {
			return Item{}, s, err
		}
		if !consumed {
			break
		}
	}

	return item, s, nil
}

func tryParseAttr(s string, item *Item) (string, bool, error) {
	s = strings.TrimSpace(s)

	if strings.HasPrefix(s, "String=") {
		val, rest, err := parseQuotedString(s[len("String="):])
		if err != nil {
			return s, false, err
		}
		item.String = val
		return rest, true, nil
	}

	if strings.HasPrefix(s, "Content=") {
		items, rest, err := parseItemList(s[len("Content="):])
		if err != nil {
			return s, false, err
		}
		item.Content = items
		return rest, true, nil
	}

	intAttrs := []struct {
		prefix string
		target *int
	}{
		{"KeyholeNumber=", &item.KeyholeNumber},
		{"KeyNumber=", &item.KeyNumber},
		{"ChestQuestNumber=", &item.ChestQuestNumber},
		{"DoorQuestNumber=", &item.DoorQuestNumber},
		{"DoorQuestValue=", &item.DoorQuestValue},
		{"Level=", &item.Level},
		{"Amount=", &item.Amount},
		{"Charges=", &item.Charges},
		{"RemainingUses=", &item.RemainingUses},
		{"PoolLiquidType=", &item.PoolLiquidType},
		{"ContainerLiquidType=", &item.ContainerLiquidType},
		{"RemainingExpireTime=", &item.RemainingExpireTime},
		{"SavedExpireTime=", &item.SavedExpireTime},
		{"AbsTeleportDestination=", &item.AbsTeleportDest},
	}
	for _, ia := range intAttrs {
		if strings.HasPrefix(s, ia.prefix) {
			rest := s[len(ia.prefix):]
			end := 0
			if end < len(rest) && rest[end] == '-' {
				end++
			}
			for end < len(rest) && rest[end] >= '0' && rest[end] <= '9' {
				end++
			}
			if end == 0 || (end == 1 && rest[0] == '-') {
				return s, false, fmt.Errorf("bad integer for %s in %q", ia.prefix, truncate(rest, 30))
			}
			v, err := strconv.Atoi(rest[:end])
			if err != nil {
				return s, false, err
			}
			*ia.target = v
			return rest[end:], true, nil
		}
	}

	return s, false, nil
}

func parseQuotedString(s string) (string, string, error) {
	if len(s) == 0 || s[0] != '"' {
		return "", s, fmt.Errorf("expected '\"', got %q", truncate(s, 20))
	}
	s = s[1:]

	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			switch s[i+1] {
			case '"':
				b.WriteByte('"')
				i++
			case 'n':
				b.WriteByte('\n')
				i++
			case '\\':
				b.WriteByte('\\')
				i++
			default:
				b.WriteByte('\\')
				b.WriteByte(s[i+1])
				i++
			}
			continue
		}
		if s[i] == '"' {
			return b.String(), s[i+1:], nil
		}
		b.WriteByte(s[i])
	}
	return "", "", fmt.Errorf("unterminated string")
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
