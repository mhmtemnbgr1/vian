package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Bu komutun ne iş yapacağını tanımlıyoruz
var selamCmd = &cobra.Command{
	Use:   "selam",
	Short: "Ekrana selam yazar",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Vian'dan selamlar!")
	},
}

func init() {
	// İşte bağlama noktası burası!
	rootCmd.AddCommand(selamCmd)
}
