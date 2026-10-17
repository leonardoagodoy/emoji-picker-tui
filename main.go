// emojis — seletor de emoji em TUI (Bubble Tea). "/" busca, Enter copia e sai.
// Busca em português e inglês (nomes e palavras-chave do CLDR).
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// paleta (style-guide: Vaulted)
var (
	cAccent  = lipgloss.Color("#ee5396")
	cText    = lipgloss.Color("#dde1e6")
	cStrong  = lipgloss.Color("#f2f4f8")
	cMuted   = lipgloss.Color("#8d8d8d")
	cBorder  = lipgloss.Color("#2e2e2e")
	cSurface = lipgloss.Color("#262626")
)

const (
	cellW = 8 // largura da célula em colunas
	cellH = 4 // altura da célula em linhas
)

// scaled: o kitty (protocolo de tamanho de texto, OSC 66) desenha o emoji maior,
// num bloco de 4x2 células; nos outros terminais ele sai no tamanho normal.
var scaled = os.Getenv("TERM") == "xterm-kitty" || os.Getenv("KITTY_WINDOW_ID") != ""

type tab struct {
	label string
	group string // "" = todos; "@recent" = recentes
}

type statusMsg struct{ text string }

type model struct {
	all    []emoji
	byChar map[string]emoji
	tabs   []tab
	tabIdx int
	shown  []emoji
	cur    int
	off    int // primeira linha visível do grid

	searching bool
	prevTab   int // aba de antes da busca, restaurada ao fechá-la

	w, h   int
	filter textinput.Model
	status string
}

func newModel() *model {
	f := textinput.New()
	f.Prompt = "/ "
	f.Placeholder = "buscar emoji…"
	f.PromptStyle = lipgloss.NewStyle().Foreground(cAccent)
	f.TextStyle = lipgloss.NewStyle().Foreground(cText)
	f.PlaceholderStyle = lipgloss.NewStyle().Foreground(cMuted)
	f.Cursor.Style = lipgloss.NewStyle().Foreground(cAccent)

	m := &model{all: load(), byChar: map[string]emoji{}, filter: f}
	for _, e := range m.all {
		m.byChar[e.char] = e
	}
	if len(loadRecent()) > 0 {
		m.tabs = append(m.tabs, tab{"recentes", "@recent"})
	}
	m.tabs = append(m.tabs, tab{"todos", ""})
	for _, g := range groups {
		m.tabs = append(m.tabs, tab{g.label, g.id})
	}
	m.refilter()
	return m
}

func (m *model) Init() tea.Cmd { return nil }

// ── dados ───────────────────────────────────────────────────────────────────

func (m *model) refilter() {
	words := strings.Fields(strings.ToLower(m.filter.Value()))
	t := m.tabs[m.tabIdx]
	var pool []emoji
	if t.group == "@recent" {
		for _, c := range loadRecent() {
			if e, ok := m.byChar[c]; ok {
				pool = append(pool, e)
			}
		}
	} else {
		pool = m.all
	}
	m.shown = m.shown[:0]
	for _, e := range pool {
		if t.group != "" && t.group != "@recent" && e.group != t.group {
			continue
		}
		ok := true
		for _, w := range words {
			if !strings.Contains(e.keys, w) {
				ok = false
				break
			}
		}
		if ok {
			m.shown = append(m.shown, e)
		}
	}
	m.cur, m.off = 0, 0
}

// cols deixa sempre ao menos 1 coluna livre à direita (ver gridRow).
func (m *model) cols() int { return max((m.w-1)/cellW, 1) }

// gridRows: linhas visíveis do grid (cabeçalho, abas, separador, detalhe, rodapé = 5).
func (m *model) gridRows() int { return max((m.h-5)/cellH, 1) }

func (m *model) move(dx, dy int) {
	if len(m.shown) == 0 {
		return
	}
	cols := m.cols()
	n := m.cur + dx + dy*cols
	if dy != 0 && (n < 0 || n >= len(m.shown)) { // última linha curta: vai pro último
		if dy > 0 && m.cur/cols < (len(m.shown)-1)/cols {
			n = len(m.shown) - 1
		} else {
			return
		}
	}
	m.cur = max(0, min(n, len(m.shown)-1))
	m.scroll()
}

func (m *model) scroll() {
	row, rows := m.cur/m.cols(), m.gridRows()
	if row < m.off {
		m.off = row
	}
	if row >= m.off+rows {
		m.off = row - rows + 1
	}
}

// allTab é o índice da aba "todos".
func (m *model) allTab() int {
	for i, t := range m.tabs {
		if t.group == "" {
			return i
		}
	}
	return 0
}

func (m *model) setTab(d int) {
	m.tabIdx = (m.tabIdx + d + len(m.tabs)) % len(m.tabs)
	m.refilter()
}

// ── update ──────────────────────────────────────────────────────────────────

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
		m.scroll()
		return m, nil
	case statusMsg:
		m.status = msg.text
		return m, nil
	case tea.KeyMsg:
		if cmd, done := m.updateKey(msg); done {
			return m, cmd
		}
	}
	if m.searching { // blink do cursor
		var cmd tea.Cmd
		m.filter, cmd = m.filter.Update(msg)
		return m, cmd
	}
	return m, nil
}

// updateKey trata as teclas; done=false deixa a tecla seguir para o campo de busca.
func (m *model) updateKey(msg tea.KeyMsg) (tea.Cmd, bool) {
	key := msg.String()

	// teclas que valem com ou sem a busca aberta
	switch key {
	case "ctrl+c":
		return tea.Quit, true
	case "enter":
		if m.cur < len(m.shown) {
			e := m.shown[m.cur]
			if err := copyText(e.char); err != nil {
				return func() tea.Msg { return statusMsg{"erro ao copiar: " + err.Error()} }, true
			}
			saveRecent(e.char)
			return tea.Quit, true
		}
		return nil, true
	case "left":
		m.move(-1, 0)
		return nil, true
	case "right":
		m.move(1, 0)
		return nil, true
	case "up":
		m.move(0, -1)
		return nil, true
	case "down":
		m.move(0, 1)
		return nil, true
	case "pgup":
		m.move(0, -m.gridRows())
		return nil, true
	case "pgdown":
		m.move(0, m.gridRows())
		return nil, true
	case "tab", "shift+tab":
		if !m.searching { // a busca sempre roda na aba "todos"
			if key == "tab" {
				m.setTab(1)
			} else {
				m.setTab(-1)
			}
		}
		return nil, true
	}

	if m.searching {
		if key == "esc" {
			m.searching = false
			m.filter.Blur()
			m.filter.SetValue("")
			m.tabIdx = m.prevTab
			m.refilter()
			return nil, true
		}
		prev := m.filter.Value()
		var cmd tea.Cmd
		m.filter, cmd = m.filter.Update(msg)
		if m.filter.Value() != prev {
			m.status = ""
			m.refilter()
		}
		return cmd, true
	}

	switch key {
	case "q", "esc":
		return tea.Quit, true
	case "/":
		m.searching = true
		m.prevTab = m.tabIdx
		m.tabIdx = m.allTab()
		m.refilter()
		return m.filter.Focus(), true
	case "h":
		m.move(-1, 0)
	case "l":
		m.move(1, 0)
	case "k":
		m.move(0, -1)
	case "j":
		m.move(0, 1)
	case "home", "g":
		m.cur = 0
		m.scroll()
	case "end", "G":
		m.cur = max(len(m.shown)-1, 0)
		m.scroll()
	}
	return nil, true
}

// ── view ────────────────────────────────────────────────────────────────────

func fg(c lipgloss.Color) lipgloss.Style { return lipgloss.NewStyle().Foreground(c) }

func (m *model) View() string {
	if m.w < 24 || m.h < 8 {
		return "terminal pequeno demais"
	}
	lines := make([]string, 0, m.h)
	if m.searching {
		lines = append(lines, m.filter.View())
	} else {
		lines = append(lines, fg(cMuted).Render(fmt.Sprintf("emojis  %d", len(m.shown))))
	}
	lines = append(lines, m.tabsLine())

	cols, rows := m.cols(), m.gridRows()
	for r := 0; r < rows; r++ {
		lines = append(lines, m.gridRow(m.off+r, cols)...)
	}

	for len(lines) < m.h-3 { // sobra de altura quando não divide por cellH
		lines = append(lines, "")
	}
	lines = append(lines, fg(cBorder).Render(strings.Repeat("─", m.w)))
	lines = append(lines, m.detail())
	lines = append(lines, m.footer())
	for i, l := range lines {
		lines[i] = ansi.Truncate(l, m.w, "")
	}
	return strings.Join(lines, "\n")
}

func (m *model) tabsLine() string {
	var parts []string
	for i, t := range m.tabs {
		if i == m.tabIdx {
			parts = append(parts, lipgloss.NewStyle().Foreground(cAccent).Bold(true).Render("▸"+t.label))
		} else {
			parts = append(parts, fg(cMuted).Render(" "+t.label))
		}
	}
	return strings.Join(parts, " ")
}

// gridRow desenha uma linha de emojis. Cada emoji vai em coluna absoluta (CSI G),
// assim larguras divergentes entre fonte e terminal não deslocam o resto da linha.
func (m *model) gridRow(row, cols int) []string {
	// Cada célula tem 4 linhas: respiro, emoji, (2ª linha do emoji escalado), respiro.
	// Os blocos são posicionados por coluna absoluta (CSI G) porque o renderer não
	// conta esses movimentos ao medir a largura da linha: por isso a linha do emoji
	// é apagada (CSI 2K) antes de desenhar, senão ficaria rastro do bloco selecionado.
	// A 3ª linha NÃO é apagada (apagaria a metade de baixo do emoji): só reescreve as
	// laterais, fora das colunas do emoji.
	const (
		erase = "\x1b[2K"
		selBg = "\x1b[48;2;238;83;150m"
		reset = "\x1b[0m"
	)
	ew, eh := 2, 1 // tamanho do emoji em células
	if scaled {
		ew, eh = 4, 2
	}
	pad := (cellW - ew) / 2 // sobra de cada lado
	var l [cellH]strings.Builder
	l[0].WriteString(erase)
	l[1].WriteString(erase)
	l[cellH-1].WriteString(erase)
	for c := 0; c < cols; c++ {
		i := row*cols + c
		if i >= len(m.shown) {
			break
		}
		x := c*cellW + 1
		ex := x + pad
		sel := i == m.cur
		if sel { // bloco cheio na cor do accent
			blk := selBg + strings.Repeat(" ", cellW) + reset
			for _, n := range []int{0, cellH - 1} {
				fmt.Fprintf(&l[n], "\x1b[%dG%s", x, blk)
			}
			fmt.Fprintf(&l[1], "\x1b[%dG%s", x, blk)
		}
		// linhas de baixo do emoji: só as laterais (ou tudo, se o emoji tem 1 linha)
		for n := 2; n < cellH-1; n++ {
			if n-1 < eh {
				if sel {
					side := selBg + strings.Repeat(" ", pad) + reset
					fmt.Fprintf(&l[n], "\x1b[%dG%s\x1b[%dG%s", x, side, ex+ew, side)
				} else {
					fmt.Fprintf(&l[n], "\x1b[%dG%s\x1b[%dG%s", x, strings.Repeat(" ", pad), ex+ew, strings.Repeat(" ", pad))
				}
			}
		}
		g := m.shown[i].char
		if scaled {
			g = "\x1b]66;s=2:n=3:d=4:h=2:v=2;" + g + "\x1b\\"
		}
		if sel {
			g = selBg + g + reset
		}
		fmt.Fprintf(&l[1], "\x1b[%dG%s", ex, g)
	}
	out := make([]string, cellH)
	for n := range l {
		// O renderer acha a linha curta (não conta os CSI G) e termina com CSI K, que
		// apagaria tudo à direita do cursor, inclusive a lateral do bloco selecionado.
		// Deixando o cursor na última coluna (sempre livre) o CSI K não pega nada.
		fmt.Fprintf(&l[n], "\x1b[%dG", m.w)
		out[n] = l[n].String()
	}
	return out
}

func (m *model) detail() string {
	if m.status != "" {
		return fg(cAccent).Render(m.status)
	}
	if m.cur >= len(m.shown) {
		return fg(cMuted).Render("nenhum emoji encontrado")
	}
	e := m.shown[m.cur]
	s := fg(cAccent).Render(e.char) + "  " + fg(cStrong).Render(e.pt)
	if e.en != e.pt {
		s += "  " + fg(cMuted).Render(e.en)
	}
	return s
}

func (m *model) footer() string {
	keys := [][2]string{{"enter", "copiar"}, {"←↑↓→", "mover"}, {"tab", "categoria"}, {"/", "buscar"}, {"q", "sair"}}
	if m.searching {
		keys[len(keys)-1] = [2]string{"esc", "fechar busca"}
	}
	parts := make([]string, len(keys))
	for i, k := range keys {
		parts[i] = fg(cAccent).Render(k[0]) + " " + fg(cMuted).Render(k[1])
	}
	return strings.Join(parts, "  ")
}

func main() {
	if len(os.Args) > 1 && (os.Args[1] == "-h" || os.Args[1] == "--help") {
		fmt.Println("uso: emojis   (/ para buscar; Enter copia o emoji e sai)")
		return
	}
	if _, err := tea.NewProgram(newModel(), tea.WithAltScreen()).Run(); err != nil {
		fmt.Fprintln(os.Stderr, "emojis:", err)
		os.Exit(1)
	}
}
