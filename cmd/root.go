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
	Short: "Vian: ağ içi şifreli sohbet ve dosya aktarımı",
	Run: func(cmd *cobra.Command, args []string) {
		if err := database.InitDB(ui.Cfg.DataDir); err != nil {
			// Geçmiş kaydı olmadan da çalışabilir.
			fmt.Fprintf(os.Stderr, "Uyarı: veritabanı açılamadı, geçmiş kaydedilmeyecek: %v\n", err)
		}

		if err := ui.StartApp(); err != nil {
			fmt.Printf("Arayüz başlatılırken hata oluştu: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	f := rootCmd.Flags()
	f.StringVar(&ui.Cfg.DataDir, "veri-dizini", ui.Cfg.DataDir, "veritabanı ve ayarların tutulduğu klasör")
	f.StringVar(&ui.Cfg.DownloadDir, "indirilenler", ui.Cfg.DownloadDir, "alınan dosyaların kaydedileceği klasör")
	f.StringVar(&ui.Cfg.UploadDir, "gonderilecekler", ui.Cfg.UploadDir, "dosya seçicinin açılacağı klasör")
	f.IntVar(&ui.Cfg.Port, "port", ui.Cfg.Port, "oda kurarken kullanılacak TCP portu (0 = rastgele)")
	f.IntVar(&ui.Cfg.DiscoveryPort, "kesif-portu", ui.Cfg.DiscoveryPort, "oda keşfi için UDP portu (herkeste aynı olmalı)")
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
