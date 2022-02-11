// Copyright 2022 Siôn le Roux.  All rights reserved.
// Use of this source code is subject to an MIT-style
// licence which can be found in the LICENSE file.

package main

import (
	"errors"
	"fmt"
	"image"
	"image/color"
	"log"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text"
	"golang.org/x/image/font"
)

// Media settings based on the Nokia 3310 jam restrictions
var (
	// ColorTransparent is completely transparent, used for images that aren't
	// square shaped to show the underlying colour
	ColorTransparent color.Color = color.RGBA{67, 82, 61, 0}
	// ColorLight is the ON or 1 screen colour, similar to white
	ColorLight color.Color = color.RGBA{199, 240, 216, 255}
	// ColorDark is the OFF or 0 screen colour, similar to black
	ColorDark color.Color = color.RGBA{67, 82, 61, 255}
	// NokiaPalette is a 1-bit palette of greenish colours simulating Nokia 3310
	NokiaPalette color.Palette = color.Palette{ColorTransparent, ColorDark, ColorLight}
	// GameSize is the screen resolution of a Nokia 3310
	GameSize image.Point = image.Point{84, 48}
	// StartingMoney is the amount of money you start the game with
	StartingMoney int = 1000
)

func main() {
	windowScale := 10
	ebiten.SetWindowSize(GameSize.X*windowScale, GameSize.Y*windowScale)
	ebiten.SetWindowTitle("Nokia Defence")

	// Fonts
	font := loadFont("assets/fonts/tiny.ttf", 6)

	game := &Game{
		Loading: true,
		Size:    GameSize,
		Money:   StartingMoney,
		Font:    font,
	}

	go NewGame(game)

	if err := ebiten.RunGame(game); err != nil {
		log.Fatal(err)
	}
}

// Game represents the main game state
type Game struct {
	Loading  bool
	Size     image.Point
	Cursor   *Cursor
	Sprites  map[SpriteType]*SpriteSheet
	Towers   Towers
	Money    int
	MobFrame int
	Count    int
	Font     font.Face
}

// NewGame sets up a new game object with default states and game objects
func NewGame(g *Game) {

	// Music
	const sampleRate int = 44100 // assuming "normal" sample rate
	music := loadSoundFile("assets/music/construction.ogg", sampleRate)
	musicLoop := audio.NewInfiniteLoop(music, music.Length())
	musicPlayer, err := audio.NewPlayer(audio.NewContext(sampleRate), musicLoop)
	if err != nil {
		log.Fatalf("error making music player: %v\n", err)
	}
	musicPlayer.Play()

	// Sprites
	g.Sprites = make(map[SpriteType]*SpriteSheet, 12)
	g.Sprites[spriteTowerBasic] = loadSprite("basic-tower")
	g.Sprites[spriteTowerStrong] = loadSprite("strong-tower")
	g.Sprites[spriteBigMonsterHorizont] = loadSprite("big_monster_horizont")
	g.Cursor = NewCursor(image.Pt(GameSize.X/2, GameSize.Y/2))

	g.Loading = false
}

// Layout is hardcoded for now, may be made dynamic in future
func (g *Game) Layout(outsideWidth int, outsideHeight int) (screenWidth int, screenHeight int) {
	return g.Size.X, g.Size.Y
}

// Update calculates game logic
func (g *Game) Update() error {

	// Pressing Q any time quits immediately
	if ebiten.IsKeyPressed(ebiten.KeyQ) {
		return errors.New("game quit by player")
	}

	// Pressing F toggles full-screen
	if inpututil.IsKeyJustPressed(ebiten.KeyF) {
		if ebiten.IsFullscreen() {
			ebiten.SetFullscreen(false)
		} else {
			ebiten.SetFullscreen(true)
		}
	}

	// Skip updating while the game is loading
	if g.Loading {
		return nil
	}

	// Movement controls
	if ebiten.IsKeyPressed(ebiten.KeyS) {
		g.Cursor.Move(image.Pt(0, 1))
	}
	if ebiten.IsKeyPressed(ebiten.KeyW) {
		g.Cursor.Move(image.Pt(0, -1))
	}
	if ebiten.IsKeyPressed(ebiten.KeyA) {
		g.Cursor.Move(image.Pt(-1, 0))
	}
	if ebiten.IsKeyPressed(ebiten.KeyD) {
		g.Cursor.Move(image.Pt(1, 0))
	}

	for _, t := range g.Towers {
		t.Update(g)
	}

	if g.Count%10 == 0 {
		g.MobFrame = (g.MobFrame + 1) % len(g.Sprites[spriteBigMonsterHorizont].Sprite)
	}
	g.Count++

	// Tower placement controls
	if inpututil.IsKeyJustPressed(ebiten.KeyE) {
		t := NewBasicTower(g)
		moneydiff := g.Money - t.Cost
		log.Printf("Buying tower %d - %d = %d\n", g.Money, t.Cost, moneydiff)
		if moneydiff >= 0 {
			g.Towers = append(g.Towers, t)
			g.Money = moneydiff
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyZ) {
		t := NewStrongTower(g)
		moneydiff := g.Money - t.Cost
		log.Printf("Buying tower %d - %d = %d\n", g.Money, t.Cost, moneydiff)
		if moneydiff >= 0 {
			g.Towers = append(g.Towers, t)
			g.Money = moneydiff
		}
	}

	return nil
}

// Draw draws the game screen by one frame
func (g *Game) Draw(screen *ebiten.Image) {
	// Light background
	op := &ebiten.DrawImageOptions{}
	screen.Fill(ColorLight)

	if g.Loading {
		// Try using text with pixel font
		txt := "Loading..."
		txtf, _ := font.BoundString(g.Font, txt)
		txth := (txtf.Max.Y - txtf.Min.Y).Ceil() / 2
		txtw := (txtf.Max.X - txtf.Min.X).Ceil() / 2
		text.Draw(screen, txt, g.Font, g.Size.X/2-txtw, g.Size.Y/2-txth, ColorDark)
		return
	}

	hudSize := 6.0
	ebitenutil.DrawRect(screen, 0, 0, float64(g.Size.X), hudSize, ColorDark)
	moneytxt := fmt.Sprintf("D%d", g.Money)
	text.Draw(screen, moneytxt, g.Font, 1, 5, ColorLight)

	for _, t := range g.Towers {
		t.Draw(g, screen)
	}
	op.GeoM.Translate(float64(g.Cursor.Coords.X), float64(g.Cursor.Coords.Y))
	screen.DrawImage(g.Cursor.Image, op)
	// Try drawing a moving monster sprite
	op.GeoM.Reset()
	op.GeoM.Translate(float64(20+g.Count/10), 20)
	mob := g.Sprites[spriteBigMonsterHorizont]
	frame := mob.Sprite[g.MobFrame]
	screen.DrawImage(mob.Image.SubImage(image.Rect(
		frame.Position.X,
		frame.Position.Y,
		frame.Position.X+frame.Position.W,
		frame.Position.Y+frame.Position.H,
	)).(*ebiten.Image), op)
}

// Cursor is used to interact with game entities at the given coordinates
type Cursor struct {
	Coords image.Point
	Image  *ebiten.Image
}

// NewCursor creates a new cursor struct at the given coordinates
// It is shaped like a crosshair and is used to interact with the game
func NewCursor(coords image.Point) *Cursor {

	i := image.NewPaletted(
		image.Rect(0, 0, 3, 3),
		NokiaPalette,
	)
	i.Pix = []uint8{
		0, 1, 0,
		1, 0, 1,
		0, 1, 0,
	}

	return &Cursor{
		Coords: coords,
		Image:  ebiten.NewImageFromImage(i),
	}
}

// Move moves the player upwards
func (c *Cursor) Move(dest image.Point) {
	c.Coords = c.Coords.Add(dest)
}

// Entity is anything that can be interacted with in the game and drawn  to the
// screen, like Towers and Creeps
type Entity interface {
	Update(g *Game)
	Draw(g *Game, screen *ebiten.Image)
}
