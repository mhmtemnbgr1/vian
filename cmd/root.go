package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "vian",
	Short: "Vian Ana Uygulama",
	// Burası sadece terminale 'vian' yazınca çalışır
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Şu an Vian'ın ana merkezindesin. Yardım için --help yazabilirsin.")
	},
}

func Execute() {
	rootCmd.Execute()
}
