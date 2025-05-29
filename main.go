package main

import (
	"flag"
	"log"
	"math/rand"
	"sync"
	"time"

	"github.com/gdamore/tcell/v2"
)

const (
	rows = 50
	cols = 50
)

var invertColors bool

type Cell struct {
	x, y        int
	alive       bool
	species     int // 0 = dead, 1 = green, 2 = red, 3 = blue
	next        bool
	nextSpecies int
	mu          sync.Mutex
	grid        *[][]*Cell
	gridMu      *sync.RWMutex
	neighbour8  [][2]int
}

func (c *Cell) countAliveNeighbors() (green, red, blue int) {
	c.gridMu.RLock()
	defer c.gridMu.RUnlock()

	for _, offset := range c.neighbour8 {
		nx, ny := c.x+offset[0], c.y+offset[1]
		if nx >= 0 && nx < rows && ny >= 0 && ny < cols {
			neighbor := (*c.grid)[nx][ny]
			neighbor.mu.Lock()
			if neighbor.alive {
				switch neighbor.species {
				case 1:
					green++
				case 2:
					red++
				case 3:
					blue++
				}
			}
			neighbor.mu.Unlock()
		}
	}
	return
}

func (c *Cell) computeNextState() {
	green, red, blue := c.countAliveNeighbors()
	total := green + red + blue

	c.mu.Lock()
	defer c.mu.Unlock()

	switch {
	case c.alive && c.species == 1 && (green == 2 || green == 3):
		c.next = true
		c.nextSpecies = 1
	case c.alive && c.species == 2 && (red == 2 || red == 3):
		c.next = true
		c.nextSpecies = 2
	case c.alive && c.species == 3 && (blue == 2 || blue == 3):
		c.next = true
		c.nextSpecies = 3
	case !c.alive && total == 3:
		c.next = true
		// pick dominant or random on tie
		counts := map[int]int{1: green, 2: red, 3: blue}

		maxCount := 0
		for _, count := range counts {
			if count > maxCount {
				maxCount = count
			}
		}

		// If tie, choose randomly
		var candidates []int
		for s, count := range counts {
			if count == maxCount {
				candidates = append(candidates, s)
			}
		}
		c.nextSpecies = candidates[rand.Intn(len(candidates))]
	default:
		c.next = false
		c.nextSpecies = 0
	}
}

func (c *Cell) applyNextState() {
	c.mu.Lock()
	c.alive = c.next
	c.species = c.nextSpecies
	c.mu.Unlock()
}

func (c *Cell) reactionTime() time.Duration {
	switch c.species {
	case 1:
		return 101 * time.Millisecond // Green
	case 2:
		return 102 * time.Millisecond // Red
	case 3:
		return 102 * time.Millisecond // Blue
	default:
		return 102 * time.Millisecond // Dead
	}
}

func (c *Cell) run(wg *sync.WaitGroup) {
	defer wg.Done()

	for {
		time.Sleep(c.reactionTime())
		c.computeNextState()
		c.applyNextState()
	}
}

var grid [][]*Cell
var gridMu sync.RWMutex

func initGrid() {
	grid = make([][]*Cell, rows)
	for i := range grid {
		grid[i] = make([]*Cell, cols)
		for j := range grid[i] {
			var species int
			alive := rand.Float32() < 0.3
			if alive {
				species = 1 + rand.Intn(3) // Random: 1, 2, or 3
			}
			grid[i][j] = &Cell{
				x:       i,
				y:       j,
				alive:   alive,
				species: species,
				grid:    &grid,
				gridMu:  &gridMu,
				neighbour8: [][2]int{
					{-1, -1}, {-1, 0}, {-1, 1},
					{0, -1}, {0, 1},
					{1, -1}, {1, 0}, {1, 1},
				},
			}
		}
	}
}

func displayGrid(screen tcell.Screen) {
	gridMu.RLock()
	defer gridMu.RUnlock()

	for i := range grid {
		for j := range grid[i] {
			cell := grid[i][j]
			cell.mu.Lock()
			alive := cell.alive
			species := cell.species
			cell.mu.Unlock()

			var fg, bg tcell.Color
			if alive {
				switch species {
				case 1:
					fg, bg = tcell.ColorBlack, tcell.ColorGreen
				case 2:
					fg, bg = tcell.ColorBlack, tcell.ColorRed
				case 3:
					fg, bg = tcell.ColorBlack, tcell.ColorBlue
				default:
					fg, bg = tcell.ColorBlack, tcell.ColorWhite
				}
			} else {
				fg, bg = tcell.ColorGreen, tcell.ColorBlack
			}

			if invertColors {
				fg, bg = bg, fg
			}

			style := tcell.StyleDefault.Foreground(fg).Background(bg)
			screen.SetContent(j*2, i, ' ', nil, style)
			screen.SetContent(j*2+1, i, ' ', nil, style)
		}
	}
	screen.Show()
}

func main() {
	flag.BoolVar(&invertColors, "invert", false, "invert foreground/background colors")
	flag.Parse()

	rand.Seed(time.Now().UnixNano())
	initGrid()

	screen, err := tcell.NewScreen()
	if err != nil {
		log.Fatalf("creating screen: %v", err)
	}
	if err = screen.Init(); err != nil {
		log.Fatalf("initializing screen: %v", err)
	}
	defer screen.Fini()

	screen.Clear()

	var wg sync.WaitGroup
	wg.Add(rows * cols)
	for i := range grid {
		for j := range grid[i] {
			go grid[i][j].run(&wg)
		}
	}

	go func() {
		for {
			displayGrid(screen)
			time.Sleep(50 * time.Millisecond)
		}
	}()

	for {
		ev := screen.PollEvent()
		if keyEv, ok := ev.(*tcell.EventKey); ok {
			if keyEv.Key() == tcell.KeyEscape || keyEv.Rune() == 'q' {
				return
			}
		}
	}
}
