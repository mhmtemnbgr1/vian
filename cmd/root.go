package cmd

import (
	"fmt"
	"os"
	"vian/database"
	"vian/ui"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "vian",
	Short: "Vian Ana Uygulama",
	Run: func(cmd *cobra.Command, args []string) {
		database.InitDB() // Hata verse de uygulamanın çökmemesi için hatayı yutabiliriz veya loglayabiliriz.
		
		if err := ui.StartApp(); err != nil {
			fmt.Printf("Arayüz başlatılırken hata oluştu: %v\n", err)
			os.Exit(1)
		}
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}