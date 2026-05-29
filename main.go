package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

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

	rootCmd.PersistentFlags().StringVar(&dir, "dir", "", "Path to the relevant directory.")
	rootCmd.PersistentFlags().StringVar(&outputFile, "out", "", "Output file path; stdout when empty.")
	rootCmd.AddCommand(npcCmd)
	npcCmd.AddCommand(npcResponsesCmd)

	if rootCmd.Execute() != nil {
		os.Exit(1)
	}
}
