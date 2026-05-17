package ui

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type model struct {
	cursor   int
	choices  []string
	selected string
	output   string
}

func initialModel() model {
	return model{
		choices: []string{"Sistem & Performans", "Ağ Ayarları", "Logları İncele", "Temizle", "Çıkış"},
	}
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit

		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}

		case "down", "j":
			if m.cursor < len(m.choices)-1 {
				m.cursor++
			}

		case "enter", " ":
			m.selected = m.choices[m.cursor]

			switch m.selected {
			case "Sistem & Performans":
				cmdStr := `$cpuLoad = (Get-CimInstance Win32_Processor | Measure-Object -Property LoadPercentage -Average).Average; $os = Get-CimInstance Win32_OperatingSystem; $ramUsage = [math]::Round(($os.TotalVisibleMemorySize - $os.FreePhysicalMemory) / $os.TotalVisibleMemorySize * 100, 1); $drive = Get-PSDrive C; $diskUsage = [math]::Round(($drive.Used / ($drive.Used + $drive.Free)) * 100, 1); $cpuTempK = (Get-CimInstance Win32_PerfFormattedData_Counters_ThermalZoneInformation | Select-Object -ExpandProperty Temperature | Select-Object -First 1); $cpuTemp = if ($cpuTempK) { [math]::Round($cpuTempK - 273.15, 1) } else { "N/A" }; $gpuTemp = try { (nvidia-smi --query-gpu=temperature.gpu --format=csv,noheader 2>$null) } catch { "N/A" }; if (-not $gpuTemp) { $gpuTemp = "N/A" }; Write-Output "$cpuLoad|$ramUsage|$diskUsage|$cpuTemp|$gpuTemp"`
				out, err := exec.Command("powershell", "-NoProfile", "-Command", cmdStr).Output()
				if err != nil {
					m.output = "Sistem ve performans bilgisi alınamadı."
				} else {
					parts := strings.Split(strings.TrimSpace(string(out)), "|")
					if len(parts) >= 5 {
						cpuLoadBar := drawBar("💻", "CPU Yükü", parts[0], "%", 100)
						ramBar := drawBar("🧠", "RAM Kulla.", parts[1], "%", 100)
						diskBar := drawBar("💾", "Disk (C:)", parts[2], "%", 100)
						cpuTempBar := drawBar("🌡️ ", "CPU Sıcak.", parts[3], "°C", 100)
						gpuTempBar := drawBar("🎮", "GPU Sıcak.", parts[4], "°C", 100)
						
						m.output = lipgloss.JoinVertical(lipgloss.Left,
							cpuLoadBar,
							"",
							ramBar,
							"",
							diskBar,
							"",
							cpuTempBar,
							"",
							gpuTempBar,
						)
					} else {
						m.output = "Veri okunamadı.\n" + string(out)
					}
				}

			case "Ağ Ayarları":
				cmdStr := `(Get-NetIPAddress -AddressFamily IPv4 | Where-Object { $_.InterfaceAlias -notmatch 'Loopback' } | Select-Object -First 1).IPAddress`
				out, err := exec.Command("powershell", "-NoProfile", "-Command", cmdStr).Output()
				if err != nil {
					m.output = "Ağ bilgisi alınamadı."
				} else {
					m.output = "Yerel IP Adresiniz: " + string(out)
				}

			case "Logları İncele":
				cmdStr := `Get-EventLog -LogName System -Newest 3 | ForEach-Object { "$($_.TimeGenerated.ToString('yyyy-MM-dd HH:mm:ss')) | $($_.EntryType) | $($_.Source)" }`
				out, err := exec.Command("powershell", "-NoProfile", "-Command", cmdStr).Output()
				if err != nil {
					m.output = "Loglar okunamadı (Yönetici yetkisi gerekebilir)."
				} else {
					m.output = string(out)
				}

			case "Temizle":
				m.output = ""

			case "Çıkış":
				return m, tea.Quit
			}
		}
	}
	return m, nil
}

func (m model) View() string {
	var titleStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.Color("#7D56F4")).
		Padding(0, 1).
		MarginBottom(1)

	var windowStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#874BFD")).
		Padding(1, 2).
		Width(65)

	var selectedStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#EE6FF8")).
		Bold(true)

	var separatorStyle = lipgloss.NewStyle().
		MarginTop(1).
		PaddingTop(1).
		Border(lipgloss.NormalBorder(), true, false, false, false).
		BorderForeground(lipgloss.Color("#6272A4"))

	s := titleStyle.Render(" VIAN KONTROL PANELİ ") + "\n\n"

	for i, choice := range m.choices {
		cursor := " "
		if m.cursor == i {
			cursor = ">"
			s += fmt.Sprintf("%s %s\n", cursor, selectedStyle.Render(choice))
		} else {
			s += fmt.Sprintf("%s %s\n", cursor, choice)
		}
	}

	if m.output != "" {
		s += separatorStyle.Render(m.output)
	}

	s += "\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("#626262")).Render("Çıkış için 'q' veya Ctrl+C")

	return windowStyle.Render(s) + "\n"
}

func StartApp() error {
	p := tea.NewProgram(initialModel())
	_, err := p.Run()
	return err
}

func drawBar(icon string, name string, valueStr string, unit string, maxVal float64) string {
	val, err := strconv.ParseFloat(strings.TrimSpace(valueStr), 64)
	if err != nil || valueStr == "N/A" {
		errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF5555"))
		return fmt.Sprintf("%s %-12s: %s", icon, name, errStyle.Render("Bulunamadı"))
	}

	barWidth := 20
	filled := int((val / maxVal) * float64(barWidth))
	if filled > barWidth {
		filled = barWidth
	} else if filled < 0 {
		filled = 0
	}
	empty := barWidth - filled

	barChar := "▰"
	emptyChar := "▱"

	var barColor lipgloss.Color
	percent := val / maxVal
	if percent < 0.5 {
		barColor = lipgloss.Color("#50FA7B") // Dracula Green
	} else if percent < 0.8 {
		barColor = lipgloss.Color("#FFB86C") // Dracula Orange
	} else {
		barColor = lipgloss.Color("#FF5555") // Dracula Red
	}

	barStyle := lipgloss.NewStyle().Foreground(barColor)

	filledStr := barStyle.Render(strings.Repeat(barChar, filled))
	emptyStr := lipgloss.NewStyle().Foreground(lipgloss.Color("#44475A")).Render(strings.Repeat(emptyChar, empty))

	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#F8F8F2")).Bold(true).Width(15)
	valStyle := lipgloss.NewStyle().Foreground(barColor).Bold(true).Width(8).Align(lipgloss.Right)

	return lipgloss.JoinHorizontal(lipgloss.Center,
		labelStyle.Render(fmt.Sprintf("%s %s", icon, name)),
		"  ",
		filledStr,
		emptyStr,
		"  ",
		valStyle.Render(fmt.Sprintf("%.1f%s", val, unit)),
	)
}
