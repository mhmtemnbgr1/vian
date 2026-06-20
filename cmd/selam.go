package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var selamCmd = &cobra.Command{
	Use:   "selam",
	Short: "Ekrana selam yazar",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Vian'dan selamlar!")
	},
}

func init() {
	rootCmd.AddCommand(selamCmd)
}