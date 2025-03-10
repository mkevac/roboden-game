package menus

import (
	"fmt"
	"image"
	"runtime"
	"runtime/pprof"
	"strconv"
	"strings"

	"github.com/ebitenui/ebitenui/widget"
	"github.com/hajimehoshi/ebiten/v2"
	resource "github.com/quasilyte/ebitengine-resource"
	"github.com/quasilyte/ge"
	"github.com/quasilyte/ge/xslices"
	"github.com/quasilyte/gsignal"
	"github.com/quasilyte/roboden-game/assets"
	"github.com/quasilyte/roboden-game/controls"
	"github.com/quasilyte/roboden-game/descriptions"
	"github.com/quasilyte/roboden-game/gamedata"
	"github.com/quasilyte/roboden-game/gameui"
	"github.com/quasilyte/roboden-game/gameui/eui"
	"github.com/quasilyte/roboden-game/scenes/staging"
	"github.com/quasilyte/roboden-game/serverapi"
	"github.com/quasilyte/roboden-game/session"
)

const dronesPerRow = 8

type LobbyMenuController struct {
	state *session.State

	config gamedata.LevelConfig
	mode   gamedata.Mode

	navTree       *gameui.NavTree
	rightNavBlock *gameui.NavBlock

	droneButtons         []droneButton
	turretButtons        []droneButton
	coreButtons          []coreButton
	pointsAllocatedLabel *widget.Text
	difficultyLabel      *widget.Text

	seedInput *widget.TextInput

	goButton         *widget.Button
	schemaButton     *widget.Button
	randSchemaButton *widget.Button
	backButton       *widget.Button

	colonyTab     *widget.TabBookTab
	worldTab      *widget.TabBookTab
	difficultyTab *widget.TabBookTab
	extraTab      *widget.TabBookTab
	selectedTab   *widget.TabBookTab
	autoFocusTab  *widget.TabBookTab
	tabs          []*widget.TabBookTab
	tabWidget     *widget.TabBook
	tabNavs       []*gameui.NavBlock

	navController *navController

	helpPanel  *widget.Container
	helpLabel  *widget.Text
	helpRecipe *eui.RecipeView

	recipeIcons map[gamedata.RecipeSubject]*ebiten.Image

	keyboard *eui.Keyboard

	ui *eui.SceneObject

	scene *ge.Scene
}

type coreButton struct {
	widget *eui.ItemButton
	core   *gamedata.ColonyCoreStats
}

type droneButton struct {
	widget    *eui.ItemButton
	drone     *gamedata.AgentStats
	recipe    gamedata.AgentMergeRecipe
	available bool
}

func NewLobbyMenuController(state *session.State, mode gamedata.Mode) *LobbyMenuController {
	return &LobbyMenuController{
		state: state,
		mode:  mode,
	}
}

func (c *LobbyMenuController) Init(scene *ge.Scene) {
	c.scene = scene

	c.config = *c.getConfigForMode()

	if c.state.Persistent.Settings.MusicVolumeLevel != 0 {
		scene.Audio().ContinueMusic(assets.AudioMusicTrack3)
	}

	c.prepareRecipeIcons()
	c.initUI()

	if c.state.CPUProfileWriter != nil {
		pprof.StopCPUProfile()
		if err := c.state.CPUProfileWriter.Close(); err != nil {
			panic(err)
		}
	}
	if c.state.MemProfileWriter != nil {
		pprof.WriteHeapProfile(c.state.MemProfileWriter)
		if err := c.state.MemProfileWriter.Close(); err != nil {
			panic(err)
		}
	}
}

func (c *LobbyMenuController) onTabSelected(tab *widget.TabBookTab) {
	c.navController.Unfocus()

	c.selectedTab = tab

	tabIndex := xslices.Index(c.tabs, c.selectedTab)
	for i := range c.tabNavs {
		disabled := i != tabIndex
		c.tabNavs[i].Disabled = disabled
		if i == tabIndex && c.tabs[i] == c.autoFocusTab {
			c.navController.FocusBlock(c.tabNavs[i])
		}
	}

	c.autoFocusTab = nil
}

func (c *LobbyMenuController) changeTab(i int, autoFocus bool) {
	tabIndex := xslices.Index(c.tabs, c.selectedTab) + i
	if tabIndex < 0 {
		tabIndex = len(c.tabs) - 1
	}
	if tabIndex >= len(c.tabs) {
		tabIndex = 0
	}

	c.tabWidget.SetTab(c.tabs[tabIndex])
	c.autoFocusTab = c.tabs[tabIndex]
}

func (c *LobbyMenuController) Update(delta float64) {
	c.state.MenuInput.Update()
	if c.state.MenuInput.ActionIsJustPressed(controls.ActionMenuTabLeft) {
		c.changeTab(-1, true)
		return
	}
	if c.state.MenuInput.ActionIsJustPressed(controls.ActionMenuTabRight) {
		c.changeTab(+1, true)
		return
	}
	if c.state.MenuInput.ActionIsJustPressed(controls.ActionMenuBack) {
		c.back()
		return
	}
}

func (c *LobbyMenuController) prepareRecipeIcons() {
	c.recipeIcons = gameui.GenerateRecipePreviews(c.scene, false, c.state.Persistent.Settings.LargeDiodes)
}

func (c *LobbyMenuController) initUI() {
	c.navTree = gameui.NewNavTree()
	c.rightNavBlock = c.navTree.NewBlock()

	eui.AddBackground(c.state.BackgroundImage, c.scene)
	uiResources := c.state.Resources.UI

	root := widget.NewContainer(
		widget.ContainerOpts.Layout(widget.NewAnchorLayout()),
	)

	rootGrid := widget.NewContainer(
		widget.ContainerOpts.WidgetOpts(
			widget.WidgetOpts.LayoutData(widget.AnchorLayoutData{
				HorizontalPosition: widget.AnchorLayoutPositionCenter,
				VerticalPosition:   widget.AnchorLayoutPositionCenter,
			}),
		),
		widget.ContainerOpts.Layout(widget.NewGridLayout(
			widget.GridLayoutOpts.Columns(2),
			widget.GridLayoutOpts.Stretch([]bool{false, false}, nil),
			widget.GridLayoutOpts.Spacing(4, 4))))

	root.AddChild(rootGrid)

	leftRowsContainer := widget.NewContainer(
		widget.ContainerOpts.WidgetOpts(widget.WidgetOpts.LayoutData(widget.GridLayoutData{
			VerticalPosition: widget.GridLayoutPositionStart,
		})),
		widget.ContainerOpts.Layout(widget.NewAnchorLayout()))
	leftRows := widget.NewContainer(
		widget.ContainerOpts.WidgetOpts(
			widget.WidgetOpts.LayoutData(widget.AnchorLayoutData{
				StretchHorizontal: true,
				StretchVertical:   true,
			}),
			widget.WidgetOpts.MinSize(572, (1080/2)-47),
		),
		widget.ContainerOpts.Layout(widget.NewGridLayout(
			widget.GridLayoutOpts.Columns(1),
			widget.GridLayoutOpts.Stretch([]bool{true}, nil),
			widget.GridLayoutOpts.Spacing(4, 4),
		)),
	)
	leftRowsContainer.AddChild(leftRows)
	rootGrid.AddChild(leftRowsContainer)

	rightRows := eui.NewRowLayoutContainer(4, []bool{false, true, false})
	rootGrid.AddChild(rightRows)

	c.tabWidget = c.createTabs(uiResources)
	leftRows.AddChild(c.tabWidget)

	rightRows.AddChild(c.createSeedPanel(uiResources))
	rightRows.AddChild(c.createHelpPanel(uiResources))
	rightRows.AddChild(c.createButtonsPanel(uiResources))

	c.bindNavTree()
	c.navController = setupUI(c.scene, root, c.state.MenuInput, c.navTree)
	if c.colonyTab.Disabled {
		c.selectedTab = c.worldTab
	} else {
		c.selectedTab = c.colonyTab
	}
	c.onTabSelected(c.selectedTab)

	c.updateDifficultyScore(c.calcDifficultyScore())

	if c.config.RawGameMode == "reverse" {
		c.randSchemaButton.GetWidget().Disabled = c.config.PlayersMode != serverapi.PmodeTwoPlayers
	}
}

func (c *LobbyMenuController) bindNavTree() {
	colonyBlock := c.tabNavs[0]
	worldBlock := c.tabNavs[1]
	difficultyBlock := c.tabNavs[2]
	extraBlock := c.tabNavs[3]

	splitBlock := gameui.NewMultiNavBlock(c.tabNavs...)

	c.rightNavBlock.Edges[gameui.NavLeft] = splitBlock
	worldBlock.Edges[gameui.NavRight] = c.rightNavBlock
	worldBlock.Edges[gameui.NavDown] = c.rightNavBlock
	difficultyBlock.Edges[gameui.NavRight] = c.rightNavBlock
	difficultyBlock.Edges[gameui.NavDown] = c.rightNavBlock
	extraBlock.Edges[gameui.NavRight] = c.rightNavBlock
	extraBlock.Edges[gameui.NavDown] = c.rightNavBlock
	colonyBlock.Edges[gameui.NavRight] = c.rightNavBlock

	{
		goButtonElem := c.rightNavBlock.NewElem(c.goButton)
		backButtonElem := c.rightNavBlock.NewElem(c.backButton)

		schemaButtonElem := c.rightNavBlock.NewElem(c.schemaButton)
		randSchemaButtonElem := c.rightNavBlock.NewElem(c.randSchemaButton)

		goButtonElem.Edges[gameui.NavRight] = backButtonElem
		goButtonElem.Edges[gameui.NavDown] = schemaButtonElem
		schemaButtonElem.Edges[gameui.NavRight] = randSchemaButtonElem
		schemaButtonElem.Edges[gameui.NavUp] = goButtonElem
		backButtonElem.Edges[gameui.NavDown] = randSchemaButtonElem
		backButtonElem.Edges[gameui.NavLeft] = goButtonElem
		randSchemaButtonElem.Edges[gameui.NavLeft] = schemaButtonElem
		randSchemaButtonElem.Edges[gameui.NavUp] = backButtonElem
	}

	{

		var coreElements []*gameui.NavElem
		for _, b := range c.coreButtons {
			e := colonyBlock.NewElem(b.widget.Button)
			coreElements = append(coreElements, e)
		}
		bindNavListNoWrap(coreElements, gameui.NavLeft, gameui.NavRight)

		var turretElements []*gameui.NavElem
		for _, b := range c.turretButtons {
			e := colonyBlock.NewElem(b.widget.Button)
			turretElements = append(turretElements, e)
		}
		bindNavListNoWrap(turretElements, gameui.NavLeft, gameui.NavRight)

		var droneElements []*gameui.NavElem
		for _, b := range c.droneButtons {
			e := colonyBlock.NewElem(b.widget.Button)
			droneElements = append(droneElements, e)
		}

		for i, coreElem := range coreElements {
			if i > len(turretElements) {
				break
			}
			coreElem.Edges[gameui.NavDown] = turretElements[i]
		}
		for i, turretElem := range turretElements {
			targetI := i
			if targetI >= len(coreElements) {
				targetI = len(coreElements) - 1
			}
			turretElem.Edges[gameui.NavUp] = coreElements[targetI]
			turretElem.Edges[gameui.NavDown] = droneElements[i]
		}

		for i, droneElem := range droneElements[:dronesPerRow] {
			targetI := i
			if targetI >= len(turretElements) {
				targetI = len(turretElements) - 1
			}
			droneElem.Edges[gameui.NavUp] = turretElements[targetI]
		}
		const numRows = 3
		for row := 0; row < numRows; row++ {
			for col := 0; col < dronesPerRow; col++ {
				i := (row * dronesPerRow) + col
				elem := droneElements[i]
				if row < numRows-1 {
					elem.Edges[gameui.NavDown] = droneElements[i+dronesPerRow]
				}
				if row > 0 {
					elem.Edges[gameui.NavUp] = droneElements[i-dronesPerRow]
				}
				if col < dronesPerRow-1 {
					elem.Edges[gameui.NavRight] = droneElements[i+1]
				}
				if col > 0 {
					elem.Edges[gameui.NavLeft] = droneElements[i-1]
				}
			}
		}
	}
}

func (c *LobbyMenuController) getConfigForMode() *gamedata.LevelConfig {
	return c.state.GetConfigForMode(c.mode)
}

func (c *LobbyMenuController) saveConfig() {
	*c.getConfigForMode() = c.config.Clone()
}

func (c *LobbyMenuController) createButtonsPanel(uiResources *eui.Resources) *widget.Container {
	panel := eui.NewPanel(uiResources, 0, 0)

	d := c.scene.Dict()

	tinyFont := assets.BitmapFont1

	c.difficultyLabel = eui.NewCenteredLabel("Difficulty: 1000%", tinyFont)
	panel.AddChild(c.difficultyLabel)

	buttonsGrid := widget.NewContainer(
		widget.ContainerOpts.WidgetOpts(widget.WidgetOpts.LayoutData(widget.RowLayoutData{
			Stretch: true,
		})),
		widget.ContainerOpts.Layout(widget.NewGridLayout(
			widget.GridLayoutOpts.Columns(2),
			widget.GridLayoutOpts.Stretch([]bool{true, true}, nil),
			widget.GridLayoutOpts.Spacing(4, 4))))

	panel.AddChild(buttonsGrid)

	c.goButton = eui.NewButton(uiResources, c.scene, d.Get("menu.lobby.go"), func() {
		c.saveConfig()

		if c.config.PlayersMode == serverapi.PmodeSinglePlayer && c.mode == gamedata.ModeReverse {
			pstats := c.state.Persistent.PlayerStats
			c.config.CoreDesign = gamedata.PickColonyDesign(pstats.CoresUnlocked, c.scene.Rand())
			c.config.TurretDesign = gamedata.PickTurretDesign(c.config.CoreDesign, pstats.TurretsUnlocked, c.scene.Rand())
			c.config.Tier2Recipes = gamedata.CreateDroneBuild(c.scene.Rand())
		}

		c.config.GameMode = c.mode
		c.config.DronePointsAllocated = c.calcAllocatedPoints()
		if c.seedInput.GetText() != "" {
			seed, err := strconv.ParseInt(c.seedInput.GetText(), 10, 64)
			if err != nil {
				panic(err)
			}
			c.config.Seed = seed
		} else {
			c.config.Seed = c.randomSeed()
		}

		c.config.Finalize()
		c.scene.Context().ChangeScene(staging.NewController(c.state, c.config.Clone(), NewLobbyMenuController(c.state, c.mode)))
	})
	buttonsGrid.AddChild(c.goButton)

	c.backButton = eui.NewButton(uiResources, c.scene, d.Get("menu.back"), func() {
		c.back()
	})
	buttonsGrid.AddChild(c.backButton)

	c.schemaButton = eui.NewButtonWithConfig(uiResources, eui.ButtonConfig{
		Scene: c.scene,
		Text:  d.Get("menu.lobby.edit"),
		OnPressed: func() {
			c.saveConfig()
			c.scene.Context().ChangeScene(NewSchemaMenuController(c.state, c.mode))
		},
		OnHover: func() {
			c.setHelpText(d.Get("menu.lobby.schema_edit"))
		},
	})
	buttonsGrid.AddChild(c.schemaButton)

	c.randSchemaButton = eui.NewButtonWithConfig(uiResources, eui.ButtonConfig{
		Scene: c.scene,
		Text:  d.Get("menu.lobby.rand"),
		OnPressed: func() {
			pstats := c.state.Persistent.PlayerStats
			drones := gamedata.RandDroneBuild(c.scene.Rand(), pstats.DronesUnlocked)
			droneMap := make(map[*gamedata.AgentStats]struct{}, len(drones))
			for _, d := range drones {
				droneMap[gamedata.FindRecipeByName(d).Result] = struct{}{}
			}
			for _, b := range c.droneButtons {
				_, enabled := droneMap[b.drone]
				b.widget.SetToggled(enabled)
			}
			c.onDroneToggled()
		},
		OnHover: func() {
			c.setHelpText(d.Get("menu.lobby.schema_rand"))
		},
	})
	buttonsGrid.AddChild(c.randSchemaButton)

	return panel
}

func (c *LobbyMenuController) createTabs(uiResources *eui.Resources) *widget.TabBook {
	tabs := []*widget.TabBookTab{}

	colonyTab := c.createColonyTab(uiResources)
	tabs = append(tabs, colonyTab)
	worldTab := c.createWorldTab(uiResources)
	tabs = append(tabs, worldTab)
	difficultyTab := c.createDifficultyTab(uiResources)
	tabs = append(tabs, difficultyTab)
	extraTab := c.createExtraTab(uiResources)
	tabs = append(tabs, extraTab)

	c.tabs = tabs

	if c.config.RawGameMode == "reverse" {
		c.maybeDisableColonyTab(c.config.PlayersMode != serverapi.PmodeTwoPlayers)
	}

	t := widget.NewTabBook(
		widget.TabBookOpts.TabSelectedHandler(func(args *widget.TabBookTabSelectedEventArgs) {
			c.onTabSelected(args.Tab)
		}),
		widget.TabBookOpts.Tabs(tabs...),
		widget.TabBookOpts.TabButtonImage(uiResources.TabButton.Image),
		widget.TabBookOpts.TabButtonText(uiResources.TabButton.FontFace, uiResources.TabButton.TextColors),
		widget.TabBookOpts.TabButtonOpts(
			widget.ButtonOpts.TextPadding(uiResources.Button.Padding),
			widget.ButtonOpts.WidgetOpts(
				widget.WidgetOpts.LayoutData(widget.RowLayoutData{
					Stretch: true,
				}),
			),
		),
		widget.TabBookOpts.TabButtonSpacing(10),
		widget.TabBookOpts.Spacing(12))

	return t
}

func (c *LobbyMenuController) maybeDisableColonyTab(disable bool) {
	if c.config.RawGameMode != "reverse" {
		return
	}
	c.colonyTab.Disabled = disable
}

func (c *LobbyMenuController) createExtraTab(uiResources *eui.Resources) *widget.TabBookTab {
	d := c.scene.Dict()

	navBlock := c.navTree.NewBlock()
	c.tabNavs = append(c.tabNavs, navBlock)

	var verticalButtons []*gameui.NavElem
	var horizontalButtons []*gameui.NavElem

	tab := widget.NewTabBookTab(d.Get("menu.lobby.tab.extra"),
		widget.ContainerOpts.Layout(widget.NewGridLayout(
			widget.GridLayoutOpts.Columns(1),
			widget.GridLayoutOpts.Stretch([]bool{true}, nil),
			widget.GridLayoutOpts.Spacing(4, 4),
		)),
	)

	{
		disabled := []int{}
		if c.config.RawGameMode == "reverse" {
			disabled = append(disabled, 1, 2, 4) // These combinations are not supported for this mode
		}
		if c.state.Device.IsMobile() {
			disabled = append(disabled, 3) // Two players are not available on mobiles
		}
		key := "menu.lobby.players"
		if c.config.RawGameMode == "reverse" {
			key += ".reverse"
		}
		b := c.newOptionButtonWithDisabled(&c.config.PlayersMode, key, disabled, []string{
			d.Get("menu.lobby.player_mode.single_player"),
			d.Get("menu.lobby.player_mode.single_bot"),
			d.Get("menu.lobby.player_mode.player_and_bot"),
			d.Get("menu.lobby.player_mode.two_players"),
			d.Get("menu.lobby.player_mode.two_bots"),
		})
		tab.AddChild(b)
		verticalButtons = append(verticalButtons, navBlock.NewElem(b))
		b.PressedEvent.AddHandler(func(args interface{}) {
			// This handler is called before the config value is changed.
			disable := c.config.PlayersMode == serverapi.PmodeTwoPlayers
			c.maybeDisableColonyTab(disable)
			c.randSchemaButton.GetWidget().Disabled = disable
		})
	}

	if c.config.RawGameMode != "reverse" {
		disabled := []int{}
		if c.config.RawGameMode == "arena" || c.config.RawGameMode == "inf_arena" {
			disabled = []int{1}
		}
		b := c.newOptionButtonWithDisabled(&c.config.InterfaceMode, "menu.lobby.ui_mode", disabled, []string{
			d.Get("menu.lobby.ui_minimal"),
			d.Get("menu.lobby.ui_radar"),
			d.Get("menu.lobby.ui_full"),
		})
		tab.AddChild(b)
		verticalButtons = append(verticalButtons, navBlock.NewElem(b))
	}

	{
		b := c.newOptionButton(&c.config.Teleporters, "menu.lobby.num_teleporters", []string{
			"0",
			"1",
			"2",
		})
		tab.AddChild(b)
		verticalButtons = append(verticalButtons, navBlock.NewElem(b))
	}

	{
		b := c.newOptionButton(&c.config.GameSpeed, "menu.lobby.game_speed", []string{
			"x1.0",
			"x1.2",
			"x1.5",
			"x2.0",
		})
		tab.AddChild(b)
		verticalButtons = append(verticalButtons, navBlock.NewElem(b))
	}

	panel := eui.NewPanel(uiResources, 0, 0)

	grid := widget.NewContainer(
		widget.ContainerOpts.Layout(widget.NewAnchorLayout()),
		widget.ContainerOpts.WidgetOpts(widget.WidgetOpts.LayoutData(widget.AnchorLayoutData{
			StretchHorizontal: true,
			StretchVertical:   true,
		})),
		widget.ContainerOpts.Layout(widget.NewGridLayout(
			widget.GridLayoutOpts.Columns(dronesPerRow),
			widget.GridLayoutOpts.Spacing(4, 4))))

	var toggleButtons []*eui.ItemButton

	toggleButtons = append(toggleButtons, c.newToggleItemButton(&c.config.Relicts, "relicts", assets.ImageRepulseTower))
	if c.config.RawGameMode != "reverse" {
		toggleButtons = append(toggleButtons, c.newToggleItemButton(&c.config.FogOfWar, "fog_of_war", assets.ImageItemFogOfWar))
	}

	for _, b := range toggleButtons {
		grid.AddChild(b.Widget)
		horizontalButtons = append(horizontalButtons, navBlock.NewElem(b.Button))
	}

	bindNavListNoWrap(verticalButtons, gameui.NavUp, gameui.NavDown)
	bindNavListNoWrap(horizontalButtons, gameui.NavLeft, gameui.NavRight)
	for _, e := range horizontalButtons {
		e.Edges[gameui.NavUp] = verticalButtons[len(verticalButtons)-1]
	}
	verticalButtons[len(verticalButtons)-1].Edges[gameui.NavDown] = horizontalButtons[0]

	panel.AddChild(grid)

	tab.AddChild(panel)

	c.extraTab = tab

	return tab
}

func (c *LobbyMenuController) createDifficultyTab(uiResources *eui.Resources) *widget.TabBookTab {
	d := c.scene.Dict()

	navBlock := c.navTree.NewBlock()
	c.tabNavs = append(c.tabNavs, navBlock)

	var verticalButtons []*gameui.NavElem
	var horizontalButtons []*gameui.NavElem

	tab := widget.NewTabBookTab(d.Get("menu.lobby.tab.difficulty"),
		widget.ContainerOpts.Layout(widget.NewGridLayout(
			widget.GridLayoutOpts.Columns(1),
			widget.GridLayoutOpts.Stretch([]bool{true}, nil),
			widget.GridLayoutOpts.Spacing(4, 4),
		)),
	)

	if c.mode == gamedata.ModeClassic || c.mode == gamedata.ModeBlitz {
		disabled := []int{}
		if c.mode == gamedata.ModeBlitz {
			disabled = []int{0, 1, 5}
		}
		b := c.newOptionButtonWithDisabled(&c.config.NumCreepBases, "menu.lobby.num_creep_bases", disabled, []string{
			"0",
			"1",
			"2",
			"3",
			"4",
			"5",
		})
		tab.AddChild(b)
		verticalButtons = append(verticalButtons, navBlock.NewElem(b))
	}

	if c.mode != gamedata.ModeBlitz {
		b := c.newOptionButton(&c.config.InitialCreeps, "menu.lobby.initial_creeps", []string{
			d.Get("menu.option.none"),
			d.Get("menu.option.some"),
			d.Get("menu.option.lots"),
		})
		tab.AddChild(b)
		verticalButtons = append(verticalButtons, navBlock.NewElem(b))
	}

	{
		disabled := []int{}
		if c.config.RawGameMode != "reverse" {
			disabled = append(disabled, 0, 1)
		}
		b := c.newOptionButtonWithDisabled(&c.config.CreepDifficulty, "menu.lobby.creeps_difficulty", disabled, []string{
			"25%",
			"50%",
			"75%",
			"100%",
			"125%",
			"150%",
			"175%",
			"200%",
			"225%",
			"250%",
			"275%",
			"300%",
			"325%",
			"350%",
		})
		tab.AddChild(b)
		verticalButtons = append(verticalButtons, navBlock.NewElem(b))
	}

	if c.mode == gamedata.ModeReverse {
		b := c.newOptionButton(&c.config.DronesPower, "menu.lobby.drones_power", []string{
			"80%",
			"100%",
			"120%",
			"140%",
			"160%",
			"180%",
			"200%",
			"220%",
		})
		tab.AddChild(b)
		verticalButtons = append(verticalButtons, navBlock.NewElem(b))
	}

	if c.mode == gamedata.ModeReverse {
		techProgressSelect := c.newOptionButton(&c.config.TechProgressRate, "menu.lobby.tech_progress_rate", []string{
			"40%",
			"50%",
			"60%",
			"70%",
			"80%",
			"90%",
			"100%",
			"110%",
			"120%",
		})
		tab.AddChild(techProgressSelect)
		verticalButtons = append(verticalButtons, navBlock.NewElem(techProgressSelect))

		superCreepRateSelect := c.newOptionButton(&c.config.ReverseSuperCreepRate, "menu.lobby.reverse_super_creep_rate", []string{
			"x0.1",
			"x0.4",
			"x0.7",
			"x1.0",
			"x1.3",
		})
		tab.AddChild(superCreepRateSelect)
		verticalButtons = append(verticalButtons, navBlock.NewElem(superCreepRateSelect))
	}

	if c.mode == gamedata.ModeClassic || c.mode == gamedata.ModeReverse {
		bossDifficultySelect := c.newOptionButton(&c.config.BossDifficulty, "menu.lobby.boss_difficulty", []string{
			d.Get("menu.power.weak"),
			d.Get("menu.power.normal"),
			d.Get("menu.power.tough"),
			d.Get("menu.power.very_tough"),
		})
		tab.AddChild(bossDifficultySelect)
		verticalButtons = append(verticalButtons, navBlock.NewElem(bossDifficultySelect))
	}

	if c.mode == gamedata.ModeBlitz {
		creepProductionRateSelect := c.newOptionButton(&c.config.CreepProductionRate, "menu.lobby.creep_production_rate", []string{
			"100%",
			"120%",
			"140%",
			"160%",
			"180%",
			"200%",
			"220%",
			"240%",
			"260%",
			"280%",
			"300%",
		})
		tab.AddChild(creepProductionRateSelect)
		verticalButtons = append(verticalButtons, navBlock.NewElem(creepProductionRateSelect))
	}

	if c.mode == gamedata.ModeClassic {
		creepSpawnRateSelect := c.newOptionButton(&c.config.CreepSpawnRate, "menu.lobby.creep_spawn_rate", []string{
			"75%",
			"100%",
			"125%",
			"150%",
			"175%",
			"200%",
		})
		tab.AddChild(creepSpawnRateSelect)
		verticalButtons = append(verticalButtons, navBlock.NewElem(creepSpawnRateSelect))
	}

	if c.mode == gamedata.ModeArena || c.mode == gamedata.ModeInfArena {
		b := c.newOptionButton(&c.config.ArenaProgression, "menu.lobby.arena_progression", []string{
			"75%",
			"100%",
			"125%",
			"150%",
			"175%",
			"200%",
			"225%",
			"250%",
		})
		tab.AddChild(b)
		verticalButtons = append(verticalButtons, navBlock.NewElem(b))
	}

	panel := eui.NewPanel(uiResources, 0, 0)

	grid := widget.NewContainer(
		widget.ContainerOpts.Layout(widget.NewAnchorLayout()),
		widget.ContainerOpts.WidgetOpts(widget.WidgetOpts.LayoutData(widget.AnchorLayoutData{
			StretchHorizontal: true,
			StretchVertical:   true,
		})),
		widget.ContainerOpts.Layout(widget.NewGridLayout(
			widget.GridLayoutOpts.Columns(8),
			widget.GridLayoutOpts.Spacing(4, 4))))

	var toggleButtons []*eui.ItemButton

	if c.mode != gamedata.ModeBlitz {
		toggleButtons = append(toggleButtons, c.newToggleItemButton(&c.config.StartingResources, "starting_resources", assets.ImageItemStartingResources))
	}
	if c.mode == gamedata.ModeReverse {
		toggleButtons = append(toggleButtons, c.newToggleItemButton(&c.config.AtomicBomb, "atom_weapon", assets.ImageItemAtomWeapon))
	}
	if c.mode == gamedata.ModeReverse {
		toggleButtons = append(toggleButtons, c.newToggleItemButton(&c.config.EliteFleet, "elite_fleet", assets.ImageItemEliteFleet))
	}
	if c.mode == gamedata.ModeClassic || c.mode == gamedata.ModeBlitz {
		toggleButtons = append(toggleButtons, c.newToggleItemButton(&c.config.SuperCreeps, "super_creeps", assets.ImageItemSuperCreeps))
	}
	toggleButtons = append(toggleButtons, c.newToggleItemButton(&c.config.CreepFortress, "creep_fortress", assets.ImageItemFortress))
	toggleButtons = append(toggleButtons, c.newToggleItemButton(&c.config.IonMortars, "ion_mortars", assets.ImageIonMortarCreep))
	if c.mode == gamedata.ModeClassic {
		toggleButtons = append(toggleButtons, c.newToggleItemButton(&c.config.CoordinatorCreeps, "coordinator_creeps", assets.ImageCreepCenturion))
	}
	switch c.mode {
	case gamedata.ModeClassic, gamedata.ModeArena, gamedata.ModeInfArena, gamedata.ModeBlitz:
		toggleButtons = append(toggleButtons, c.newToggleItemButton(&c.config.GrenadierCreeps, "grenadier_creeps", assets.ImageCreepGrenadier))
	}

	for _, b := range toggleButtons {
		grid.AddChild(b.Widget)
		horizontalButtons = append(horizontalButtons, navBlock.NewElem(b.Button))
	}

	bindNavListNoWrap(verticalButtons, gameui.NavUp, gameui.NavDown)
	bindNavListNoWrap(horizontalButtons, gameui.NavLeft, gameui.NavRight)
	for _, e := range horizontalButtons {
		e.Edges[gameui.NavUp] = verticalButtons[len(verticalButtons)-1]
	}
	verticalButtons[len(verticalButtons)-1].Edges[gameui.NavDown] = horizontalButtons[0]

	panel.AddChild(grid)

	tab.AddChild(panel)

	c.difficultyTab = tab

	return tab
}

func (c *LobbyMenuController) optionDescriptionText(key string) string {
	d := c.scene.Dict()
	return fmt.Sprintf("%s\n\n%s", d.Get(key), d.Get(key, "description"))
}

func (c *LobbyMenuController) newToggleItemButton(value *bool, key string, icon resource.ImageID) *eui.ItemButton {
	var b *eui.ItemButton

	info := gamedata.LobbyOptionMap[key]
	playerStats := c.state.Persistent.PlayerStats
	unlocked := playerStats.TotalScore >= info.ScoreCost

	var img *ebiten.Image
	if unlocked {
		img = createSubImage(c.scene.LoadImage(icon))
	} else {
		img = c.scene.LoadImage(assets.ImageLock).Data
	}
	b = eui.NewItemButton(c.state.Resources.UI, img, nil, "", 0, func() {
		*value = !*value
		b.Toggle()
		c.updateDifficultyScore(c.calcDifficultyScore())
	})
	b.SetDisabled(!unlocked)

	d := c.scene.Dict()
	b.Button.CursorEnteredEvent.AddHandler(func(args interface{}) {
		var s string
		if unlocked {
			s = c.optionDescriptionText("menu.lobby." + key)
		} else {
			s = fmt.Sprintf("%s\n\n%s: %d/%d", d.Get("menu.option.locked"), d.Get("drone.score_required"), playerStats.TotalScore, info.ScoreCost)
		}
		c.setHelpText(s)
	})

	if *value {
		b.Toggle()
	}

	return b
}

func (c *LobbyMenuController) newOptionButtonWithDisabled(value *int, key string, disabled []int, valueNames []string) *widget.Button {
	b := eui.NewSelectButton(eui.SelectButtonConfig{
		PlaySound:      true,
		Resources:      c.state.Resources.UI,
		Input:          c.state.MenuInput,
		Value:          value,
		DisabledValues: disabled,
		Label:          c.scene.Dict().Get(key),
		ValueNames:     valueNames,
		OnPressed: func() {
			c.updateDifficultyScore(c.calcDifficultyScore())
		},
		OnHover: func() {
			c.setHelpText(c.optionDescriptionText(key))
		},
	})
	c.scene.AddObject(b)
	return b.Widget
}

func (c *LobbyMenuController) newOptionButton(value *int, key string, valueNames []string) widget.PreferredSizeLocateableWidget {
	return c.newOptionButtonWithDisabled(value, key, nil, valueNames)
}

func (c *LobbyMenuController) createWorldTab(uiResources *eui.Resources) *widget.TabBookTab {
	d := c.scene.Dict()

	navBlock := c.navTree.NewBlock()
	c.tabNavs = append(c.tabNavs, navBlock)

	var verticalButtons []*gameui.NavElem
	var horizontalButtons []*gameui.NavElem

	tab := widget.NewTabBookTab(d.Get("menu.lobby.tab.world"),
		widget.ContainerOpts.Layout(widget.NewGridLayout(
			widget.GridLayoutOpts.Columns(1),
			widget.GridLayoutOpts.Stretch([]bool{true}, nil),
			widget.GridLayoutOpts.Spacing(4, 4),
		)),
	)

	{
		b := c.newOptionButton(&c.config.Resources, "menu.lobby.world_resources", []string{
			d.Get("menu.option.very_low"),
			d.Get("menu.option.low"),
			d.Get("menu.option.normal"),
			d.Get("menu.option.rich"),
			d.Get("menu.option.very_rich"),
		})
		tab.AddChild(b)
		verticalButtons = append(verticalButtons, navBlock.NewElem(b))
	}

	{
		disabled := []int{}
		if c.config.RawGameMode == "reverse" || c.config.RawGameMode == "blitz" {
			disabled = append(disabled, 0)
		}
		b := c.newOptionButtonWithDisabled(&c.config.WorldSize, "menu.lobby.world_size", disabled, []string{
			d.Get("menu.option.very_small"),
			d.Get("menu.option.small"),
			d.Get("menu.option.normal"),
			d.Get("menu.option.big"),
		})
		tab.AddChild(b)
		verticalButtons = append(verticalButtons, navBlock.NewElem(b))
	}

	{
		b := c.newOptionButton(&c.config.OilRegenRate, "menu.lobby.oil_regen_rate", []string{
			"0%",
			"50%",
			"100%",
			"150%",
		})
		tab.AddChild(b)
		verticalButtons = append(verticalButtons, navBlock.NewElem(b))
	}

	{
		b := c.newOptionButton(&c.config.Terrain, "menu.lobby.land", []string{
			d.Get("menu.lobby.land_flat"),
			d.Get("menu.lobby.land_normal"),
			d.Get("menu.lobby.land_mountains"),
		})
		tab.AddChild(b)
		verticalButtons = append(verticalButtons, navBlock.NewElem(b))
	}

	{
		b := c.newOptionButton(&c.config.WorldShape, "menu.lobby.world_shape", []string{
			d.Get("menu.lobby.world_shape.square"),
			d.Get("menu.lobby.world_shape.horizontal"),
			d.Get("menu.lobby.world_shape.vertical"),
		})
		tab.AddChild(b)
		verticalButtons = append(verticalButtons, navBlock.NewElem(b))
	}

	{
		b := c.newOptionButton(&c.config.Environment, "menu.lobby.environment", []string{
			d.Get("menu.lobby.forest"),
			d.Get("menu.lobby.inferno"),
			d.Get("menu.lobby.moon"),
			d.Get("menu.lobby.snow"),
		})
		tab.AddChild(b)
		verticalButtons = append(verticalButtons, navBlock.NewElem(b))
	}

	panel := eui.NewPanel(uiResources, 0, 0)

	grid := widget.NewContainer(
		widget.ContainerOpts.Layout(widget.NewAnchorLayout()),
		widget.ContainerOpts.WidgetOpts(widget.WidgetOpts.LayoutData(widget.AnchorLayoutData{
			StretchHorizontal: true,
			StretchVertical:   true,
		})),
		widget.ContainerOpts.Layout(widget.NewGridLayout(
			widget.GridLayoutOpts.Columns(8),
			widget.GridLayoutOpts.Spacing(4, 4))))

	var toggleButtons []*eui.ItemButton

	toggleButtons = append(toggleButtons, c.newToggleItemButton(&c.config.GoldEnabled, "gold_enabled", assets.ImageEssenceGoldSource))
	toggleButtons = append(toggleButtons, c.newToggleItemButton(&c.config.WeatherEnabled, "weather_enabled", assets.ImageItemWeather))

	for _, b := range toggleButtons {
		grid.AddChild(b.Widget)
		horizontalButtons = append(horizontalButtons, navBlock.NewElem(b.Button))
	}

	bindNavListNoWrap(verticalButtons, gameui.NavUp, gameui.NavDown)
	bindNavListNoWrap(horizontalButtons, gameui.NavLeft, gameui.NavRight)
	for _, e := range horizontalButtons {
		e.Edges[gameui.NavUp] = verticalButtons[len(verticalButtons)-1]
	}
	verticalButtons[len(verticalButtons)-1].Edges[gameui.NavDown] = horizontalButtons[0]

	panel.AddChild(grid)

	tab.AddChild(panel)

	c.worldTab = tab

	return tab
}

func (c *LobbyMenuController) createColonyTab(uiResources *eui.Resources) *widget.TabBookTab {
	d := c.scene.Dict()

	navBlock := c.navTree.NewBlock()
	c.tabNavs = append(c.tabNavs, navBlock)

	tab := widget.NewTabBookTab(d.Get("menu.lobby.tab.colony"),
		widget.ContainerOpts.Layout(widget.NewGridLayout(
			widget.GridLayoutOpts.Columns(1),
			widget.GridLayoutOpts.Stretch([]bool{true}, nil),
			widget.GridLayoutOpts.Spacing(4, 4),
		)),
	)

	tinyFont := assets.BitmapFont1

	tab.AddChild(c.createBasesPanel(uiResources))
	tab.AddChild(c.createTurretsPanel(uiResources))

	label := widget.NewText(
		widget.TextOpts.Text("Points Allocated: 99/99", tinyFont, uiResources.Button.TextColors.Idle),
		widget.TextOpts.Position(widget.TextPositionCenter, widget.TextPositionCenter),
	)
	c.pointsAllocatedLabel = label
	tab.AddChild(label)

	tab.AddChild(c.createDronesPanel(uiResources))

	c.updateAllocatedPoints(c.calcAllocatedPoints())

	c.colonyTab = tab

	return tab
}

func (c *LobbyMenuController) calcDifficultyScore() int {
	return gamedata.CalcDifficultyScore(c.config.ReplayLevelConfig, c.calcAllocatedPoints())
}

func (c *LobbyMenuController) updateAllocatedPoints(allocated int) {
	c.pointsAllocatedLabel.Label = fmt.Sprintf("%s: %d/%d", c.scene.Dict().Get("menu.lobby.points_allocated"), allocated, gamedata.ClassicModePoints)
}

func (c *LobbyMenuController) updateDifficultyScore(score int) {
	d := c.scene.Dict()
	var tag string
	switch {
	case score < 40:
		tag = d.Get("menu.option.very_easy")
	case score < 80:
		tag = d.Get("menu.option.easy")
	case score < 120:
		tag = d.Get("menu.option.normal")
	case score < 160:
		tag = d.Get("menu.option.hard")
	case score < 220:
		tag = d.Get("menu.option.very_hard")
	case score < 350:
		tag = d.Get("menu.option.impossible")
	case score < 450:
		tag = d.Get("menu.difficulty_score_despair")
	default:
		tag = d.Get("menu.difficulty_score_ultimate_despair")
	}
	c.difficultyLabel.Label = fmt.Sprintf("%s: %d%% (%s)", c.scene.Dict().Get("menu.lobby.tab.difficulty"), score, tag)
}

func (c *LobbyMenuController) calcAllocatedPoints() int {
	total := 0
	for _, b := range c.droneButtons {
		if !b.widget.IsToggled() {
			continue
		}
		total += b.drone.PointCost
	}
	return total
}

func (c *LobbyMenuController) createHelpPanel(uiResources *eui.Resources) *widget.Container {
	panel := eui.NewTextPanel(uiResources, 0, 0)
	c.helpPanel = panel

	tinyFont := assets.BitmapFont1

	label := eui.NewLabel("", tinyFont)
	label.MaxWidth = 305
	c.helpLabel = label
	panel.AddChild(label)

	c.helpRecipe = eui.NewRecipeView(uiResources)
	panel.AddChild(c.helpRecipe.Container)

	return panel
}

func (c *LobbyMenuController) randomSeed() int64 {
	for {
		seed := c.scene.Rand().PositiveInt64()
		if gamedata.GetSeedKind(seed, c.config.ReplayLevelConfig) == gamedata.SeedNormal {
			return seed
		}
	}
}

func (c *LobbyMenuController) createSeedPanel(uiResources *eui.Resources) *widget.Container {
	worldSettingsPanel := eui.NewPanel(uiResources, 340, 0)

	tinyFont := assets.BitmapFont1

	d := c.scene.Dict()

	{
		grid := widget.NewContainer(
			widget.ContainerOpts.WidgetOpts(widget.WidgetOpts.LayoutData(widget.RowLayoutData{
				Stretch: true,
			})),
			widget.ContainerOpts.Layout(widget.NewGridLayout(
				widget.GridLayoutOpts.Columns(2),
				widget.GridLayoutOpts.Stretch([]bool{true, false}, nil),
				widget.GridLayoutOpts.Spacing(4, 4),
			)),
		)

		const maxSeedLen = 18
		textinput := eui.NewTextInput(uiResources, eui.TextInputConfig{SteamDeck: c.state.Device.IsSteamDeck()},
			widget.TextInputOpts.WidgetOpts(
				widget.WidgetOpts.CursorEnterHandler(func(args *widget.WidgetCursorEnterEventArgs) {
					c.setHelpText(c.optionDescriptionText("menu.lobby.game_seed"))
				}),
			),
			widget.TextInputOpts.Validation(func(newInputText string) (bool, *string) {
				if len(newInputText) > maxSeedLen {
					return false, nil
				}
				onlyDigits := true
				for _, ch := range newInputText {
					if ch >= '0' && ch <= '9' {
						continue
					}
					onlyDigits = false
					break
				}
				return onlyDigits, nil
			}))
		randSeed := strconv.FormatInt(c.randomSeed(), 10)
		if len(randSeed) >= maxSeedLen {
			randSeed = randSeed[:maxSeedLen]
		}
		textinput.SetText(randSeed)
		grid.AddChild(textinput)
		c.seedInput = textinput
		label := widget.NewLabel(
			widget.LabelOpts.TextOpts(
				widget.TextOpts.Position(widget.TextPositionCenter, widget.TextPositionCenter),
			),
			widget.LabelOpts.Text(d.Get("menu.lobby.game_seed"), tinyFont, &widget.LabelColor{
				Idle:     uiResources.Button.TextColors.Idle,
				Disabled: uiResources.Button.TextColors.Disabled,
			}),
		)
		grid.AddChild(label)

		if runtime.GOOS == "android" {
			c.seedInput.GetWidget().FocusEvent.AddHandler(func(args any) {
				e := args.(*widget.WidgetFocusEventArgs)
				if e.Focused {
					if c.keyboard == nil {
						c.openKeyboard()
					}
				}
			})
		}

		worldSettingsPanel.AddChild(grid)
	}

	return worldSettingsPanel
}

func (c *LobbyMenuController) createBasesPanel(uiResources *eui.Resources) *widget.Container {
	panel := eui.NewPanel(uiResources, 0, 0)

	grid := widget.NewContainer(
		widget.ContainerOpts.Layout(widget.NewAnchorLayout()),
		widget.ContainerOpts.WidgetOpts(widget.WidgetOpts.LayoutData(widget.AnchorLayoutData{
			StretchHorizontal: true,
			StretchVertical:   true,
		})),
		widget.ContainerOpts.Layout(widget.NewGridLayout(
			widget.GridLayoutOpts.Columns(6),
			widget.GridLayoutOpts.Spacing(4, 4))))

	for i := range gamedata.CoreStatsList {
		core := gamedata.CoreStatsList[i]
		var b *eui.ItemButton
		available := xslices.Contains(c.state.Persistent.PlayerStats.CoresUnlocked, core.Name)
		var img *ebiten.Image
		if available {
			img = c.scene.LoadImage(core.Image).Data
		} else {
			img = c.scene.LoadImage(assets.ImageLock).Data
		}
		b = eui.NewBigItemButton(uiResources, img, func() {
			if c.config.CoreDesign != core.Name {
				b.Toggle()
				c.onCoreToggled(core)
			}
		})
		b.SetDisabled(!available)
		grid.AddChild(b.Widget)
		b.Button.CursorEnteredEvent.AddHandler(func(args interface{}) {
			var s string
			if available {
				s = descriptions.CoreText(c.scene.Dict(), core)
			} else {
				s = descriptions.LockedCoreText(c.scene.Dict(), &c.state.Persistent.PlayerStats, core)
			}
			c.setHelpText(s)
		})
		if c.config.CoreDesign == core.Name {
			b.Toggle()
		}
		c.coreButtons = append(c.coreButtons, coreButton{
			widget: b,
			core:   core,
		})
	}

	panel.AddChild(grid)

	return panel
}

func (c *LobbyMenuController) setHelpText(s string) {
	c.helpLabel.Label = s
	c.helpRecipe.SetImages(nil, nil)
	c.helpPanel.RequestRelayout()
}

func (c *LobbyMenuController) createTurretsPanel(uiResources *eui.Resources) *widget.Container {
	panel := eui.NewPanel(uiResources, 0, 0)

	grid := widget.NewContainer(
		widget.ContainerOpts.Layout(widget.NewAnchorLayout()),
		widget.ContainerOpts.WidgetOpts(widget.WidgetOpts.LayoutData(widget.AnchorLayoutData{
			StretchHorizontal: true,
			StretchVertical:   true,
		})),
		widget.ContainerOpts.Layout(widget.NewGridLayout(
			widget.GridLayoutOpts.Columns(8),
			widget.GridLayoutOpts.Spacing(4, 4))))

	for i := range gamedata.TurretStatsList {
		turret := gamedata.TurretStatsList[i]
		available := xslices.Contains(c.state.Persistent.PlayerStats.TurretsUnlocked, turret.Kind.String())
		var img *ebiten.Image
		if available {
			imageID := turret.Image
			if turret.PreviewImage != assets.ImageNone {
				imageID = turret.PreviewImage
			}
			img = c.scene.LoadImage(imageID).Data
		} else {
			img = c.scene.LoadImage(assets.ImageLock).Data
		}
		var b *eui.ItemButton
		b = eui.NewItemButton(uiResources, img, nil, "", 0, func() {
			if c.config.TurretDesign != turret.Kind.String() {
				b.Toggle()
				c.onTurretToggled(turret)
			}
		})
		b.SetDisabled(!available)
		b.Button.CursorEnteredEvent.AddHandler(func(args interface{}) {
			var s string
			if available {
				s = descriptions.TurretText(c.scene.Dict(), turret)
			} else {
				s = descriptions.LockedTurretText(c.scene.Dict(), &c.state.Persistent.PlayerStats, turret)
			}
			c.setHelpText(s)
		})
		c.turretButtons = append(c.turretButtons, droneButton{
			widget: b,
			drone:  turret,
		})
		grid.AddChild(b.Widget)
		if c.config.TurretDesign == turret.Kind.String() {
			b.Toggle()
		}
	}

	panel.AddChild(grid)

	return panel
}

func (c *LobbyMenuController) createDronesPanel(uiResources *eui.Resources) *widget.Container {
	dronesPanel := eui.NewPanel(uiResources, 0, 0)

	smallFont := assets.BitmapFont1

	grid := widget.NewContainer(
		widget.ContainerOpts.Layout(widget.NewAnchorLayout()),
		widget.ContainerOpts.WidgetOpts(widget.WidgetOpts.LayoutData(widget.AnchorLayoutData{
			StretchHorizontal: true,
			StretchVertical:   true,
		})),
		widget.ContainerOpts.Layout(widget.NewGridLayout(
			widget.GridLayoutOpts.Columns(8),
			widget.GridLayoutOpts.Spacing(4, 4))))

	maxNumDrones := 8 * 3
	for i := range gamedata.Tier2agentMergeRecipes {
		recipe := gamedata.Tier2agentMergeRecipes[i]
		drone := recipe.Result
		available := xslices.Contains(c.state.Persistent.PlayerStats.DronesUnlocked, drone.Kind.String())
		costLabel := ""
		var frame *ebiten.Image
		if available {
			costLabel = strings.Repeat(".", recipe.Result.PointCost)
			img := c.scene.LoadImage(recipe.Result.Image)
			frame = img.Data.SubImage(image.Rectangle{
				Max: image.Point{X: int(img.DefaultFrameWidth), Y: int(img.DefaultFrameHeight)},
			}).(*ebiten.Image)
		} else {
			frame = c.scene.LoadImage(assets.ImageLock).Data
		}
		var b *eui.ItemButton
		b = eui.NewItemButton(uiResources, frame, smallFont, costLabel, 26, func() {
			b.Toggle()
			c.onDroneToggled()
			c.updateTier2Recipes()
		})
		grid.AddChild(b.Widget)
		if xslices.Contains(c.config.Tier2Recipes, recipe.Result.Kind.String()) {
			b.Toggle()
		}
		c.droneButtons = append(c.droneButtons, droneButton{
			widget:    b,
			drone:     drone,
			recipe:    recipe,
			available: available,
		})
		b.Button.CursorEnteredEvent.AddHandler(func(args interface{}) {
			if available {
				c.helpLabel.Label = descriptions.DroneText(c.scene.Dict(), drone, false, false)
				c.helpRecipe.SetImages(c.recipeIcons[recipe.Drone1], c.recipeIcons[recipe.Drone2])
			} else {
				c.helpLabel.Label = descriptions.LockedDroneText(c.scene.Dict(), &c.state.Persistent.PlayerStats, drone)
				c.helpRecipe.SetImages(nil, nil)
			}
			c.helpPanel.RequestRelayout()
		})
	}
	c.onDroneToggled()

	// Pad the remaining space with disabled buttons.
	for i := len(gamedata.Tier2agentMergeRecipes); i < maxNumDrones; i++ {
		b := eui.NewItemButton(uiResources, nil, nil, "", 0, func() {})
		b.SetDisabled(true)
		grid.AddChild(b.Widget)
	}

	dronesPanel.AddChild(grid)

	return dronesPanel
}

func (c *LobbyMenuController) updateTier2Recipes() {
	c.config.Tier2Recipes = c.config.Tier2Recipes[:0]
	for _, b := range c.droneButtons {
		if !b.widget.IsToggled() {
			continue
		}
		c.config.Tier2Recipes = append(c.config.Tier2Recipes, b.recipe.Result.Kind.String())
	}
}

func (c *LobbyMenuController) onCoreToggled(selectedCore *gamedata.ColonyCoreStats) {
	c.config.CoreDesign = selectedCore.Name
	for _, b := range c.coreButtons {
		toggle := (b.core != selectedCore && b.widget.IsToggled())
		if toggle {
			b.widget.Toggle()
		}
	}
	if c.difficultyLabel != nil {
		c.updateDifficultyScore(c.calcDifficultyScore())
	}
}

func (c *LobbyMenuController) onTurretToggled(selectedTurret *gamedata.AgentStats) {
	c.config.TurretDesign = selectedTurret.Kind.String()
	for _, b := range c.turretButtons {
		toggle := (b.drone != selectedTurret && b.widget.IsToggled())
		if toggle {
			b.widget.Toggle()
		}
	}
}

func (c *LobbyMenuController) onDroneToggled() {
	allocated := c.calcAllocatedPoints()
	pointsLeft := gamedata.ClassicModePoints - allocated
	c.updateAllocatedPoints(allocated)
	for _, b := range c.droneButtons {
		if b.widget.IsToggled() {
			continue
		}
		b.widget.SetDisabled(!b.available || b.drone.PointCost > pointsLeft)
	}
	if c.difficultyLabel != nil {
		c.updateDifficultyScore(c.calcDifficultyScore())
	}
}

func (c *LobbyMenuController) back() {
	c.saveConfig()
	c.scene.Context().ChangeScene(NewPlayMenuController(c.state))
}

func (c *LobbyMenuController) openKeyboard() {
	k := eui.NewTextKeyboard(eui.KeyboardConfig{
		Resources:  c.state.Resources.UI,
		Scene:      c.scene,
		Input:      c.state.MenuInput,
		DigitsOnly: true,
	})
	c.ui.AddWindow(k.Window)

	runeBuf := []rune{0}
	k.EventKey.Connect(nil, func(ch rune) {
		runeBuf[0] = ch
		c.seedInput.Insert(runeBuf)
		c.seedInput.Focus(true)
	})
	k.EventBackspace.Connect(nil, func(gsignal.Void) {
		c.seedInput.Backspace()
		c.seedInput.Focus(true)
	})
	k.EventSubmit.Connect(nil, func(gsignal.Void) {
		c.seedInput.Submit()
		k.Close()
	})
	k.EventClosed.Connect(nil, func(gsignal.Void) {
		c.keyboard = nil
	})
	c.keyboard = k
	c.scene.AddObject(c.keyboard)
}
