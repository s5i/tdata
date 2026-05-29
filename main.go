package main

import (
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	mapfile "github.com/s5i/tdata/map"
	"github.com/s5i/tdata/npc"
	"github.com/spf13/cobra"
)

func main() {
	var dir string
	var outputFile string

	output := func() (io.Writer, func(), error) {
		if outputFile == "" {
			return os.Stdout, func() {}, nil
		}

		if err := os.MkdirAll(filepath.Dir(outputFile), 0755); err != nil {
			return nil, nil, err
		}

		f, err := os.Create(outputFile)
		if err != nil {
			return nil, nil, err
		}

		return f, func() { f.Close() }, nil
	}

	rootCmd := &cobra.Command{
		Use:   "tdata",
		Short: "Tibia Retail Data Processor",
	}

	npcCmd := &cobra.Command{
		Use: "npc",
	}

	npcResponsesCmd := &cobra.Command{
		Use:  "responses --dir=... [--out=f]",
		Args: cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			out, outClose, err := output()
			if err != nil {
				return err
			}
			defer outClose()

			resp, err := npc.ProcessDir(dir)
			if err != nil {
				return err
			}

			for _, npc := range resp {
				for _, r := range npc.Responses {
					fmt.Fprintf(out, "%s: %s\n", npc.NPC, r)
				}
			}

			return nil
		},
	}

	mapCmd := &cobra.Command{
		Use: "map",
	}

	mapTilesCmd := &cobra.Command{
		Use:  "strings --dir=... [--out=f]",
		Args: cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			out, outClose, err := output()
			if err != nil {
				return err
			}
			defer outClose()

			sectors, err := mapfile.ProcessDir(dir)
			if err != nil {
				return err
			}

			positions := map[string][]mapfile.Pos{}
			var stringsF func(p mapfile.Pos, i mapfile.Item)
			stringsF = func(p mapfile.Pos, i mapfile.Item) {
				if i.String != "" {
					k := fmt.Sprintf("[%d] %s", i.ID, strings.ReplaceAll(i.String, "\n", " "))
					positions[k] = append(positions[k], p)
				}
				for _, c := range i.Content {
					stringsF(p, c)
				}
			}

			for _, sec := range sectors {
				for _, tile := range sec.Tiles {
					for _, i := range tile.Items {
						stringsF(tile.Pos, i)
					}
				}
			}

			for _, str := range slices.Sorted(maps.Keys(positions)) {
				fmt.Fprintf(out, "%s\n", str)
				for _, pos := range positions[str] {
					fmt.Fprintf(out, "- https://tibiantis.info/library/map#%d,%d,%d,8\n", pos.X, pos.Y, pos.Z)
				}
			}

			return nil
		},
	}

	rootCmd.PersistentFlags().StringVar(&dir, "dir", "", "Path to the relevant directory.")
	rootCmd.PersistentFlags().StringVar(&outputFile, "out", "", "Output file path; stdout when empty.")
	rootCmd.AddCommand(npcCmd, mapCmd)
	npcCmd.AddCommand(npcResponsesCmd)
	mapCmd.AddCommand(mapTilesCmd)

	if rootCmd.Execute() != nil {
		os.Exit(1)
	}
}

func formatItems(items []mapfile.Item) string {
	if len(items) == 0 {
		return "{}"
	}
	var parts []string
	for _, item := range items {
		parts = append(parts, formatItem(item))
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

func formatItem(item mapfile.Item) string {
	s := fmt.Sprintf("%d", item.ID)

	type intAttr struct {
		name string
		val  int
	}
	for _, a := range []intAttr{
		{"KeyholeNumber", item.KeyholeNumber},
		{"KeyNumber", item.KeyNumber},
		{"ChestQuestNumber", item.ChestQuestNumber},
		{"DoorQuestNumber", item.DoorQuestNumber},
		{"DoorQuestValue", item.DoorQuestValue},
		{"Level", item.Level},
		{"Amount", item.Amount},
		{"Charges", item.Charges},
		{"RemainingUses", item.RemainingUses},
		{"PoolLiquidType", item.PoolLiquidType},
		{"ContainerLiquidType", item.ContainerLiquidType},
		{"RemainingExpireTime", item.RemainingExpireTime},
		{"SavedExpireTime", item.SavedExpireTime},
		{"AbsTeleportDest", item.AbsTeleportDest},
	} {
		if a.val != 0 {
			s += fmt.Sprintf(" %s=%d", a.name, a.val)
		}
	}

	if item.String != "" {
		s += fmt.Sprintf(" String=%q", item.String)
	}

	if len(item.Content) > 0 {
		s += " Content=" + formatItems(item.Content)
	}

	return s
}
