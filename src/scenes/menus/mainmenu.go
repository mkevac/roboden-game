package menus

import (
	"fmt"
	"os"
	"runtime"
	"strconv"

	"github.com/ebitenui/ebitenui/widget"
	"github.com/quasilyte/ge"
	"github.com/quasilyte/roboden-game/assets"
	"github.com/quasilyte/roboden-game/buildinfo"
	"github.com/quasilyte/roboden-game/controls"
	"github.com/quasilyte/roboden-game/gamedata"
	"github.com/quasilyte/roboden-game/gameui"
	"github.com/quasilyte/roboden-game/gameui/eui"
	"github.com/quasilyte/roboden-game/session"
)

type MainMenuController struct {
	state *session.State

	scene *ge.Scene
}

func NewMainMenuController(state *session.State) *MainMenuController {
	return &MainMenuController{state: state}
}

func (c *MainMenuController) Init(scene *ge.Scene) {
	c.scene = scene

	// c.cursor = gameui.NewCursorNode(c.state.MenuInput, scene.Context().WindowRect())
	// scene.AddObject(c.cursor)

	c.state.AdjustVolumeLevels()

	if c.state.Persistent.Settings.MusicVolumeLevel != 0 {
		scene.Audio().ContinueMusic(assets.AudioMusicTrack3)
	}

	c.initUI()
}

func (c *MainMenuController) Update(delta float64) {
	c.state.MenuInput.Update()
	if c.state.MenuInput.ActionIsJustPressed(controls.ActionMenuBack) {
		c.scene.Audio().PauseCurrentMusic()
		c.scene.Context().ChangeScene(NewSplashScreenController(c.state, NewMainMenuController(c.state)))
		return
	}
}

func (c *MainMenuController) initUI() {
	eui.AddBackground(c.state.BackgroundImage, c.scene)

	uiResources := c.state.Resources.UI

	root := eui.NewAnchorContainer()
	rowContainer := eui.NewRowLayoutContainerWithMinWidth(400, 10, nil)
	root.AddChild(rowContainer)

	d := c.scene.Dict()

	logo := widget.NewGraphic(widget.GraphicOpts.Image(c.scene.LoadImage(assets.ImageLogo).Data))
	rowContainer.AddChild(logo)

	rowContainer.AddChild(eui.NewTransparentSeparator())

	playButton := eui.NewButton(uiResources, c.scene, d.Get("menu.main.play"), func() {
		c.scene.Context().ChangeScene(NewPlayMenuController(c.state))
	})
	rowContainer.AddChild(playButton)

	profileButton := eui.NewButton(uiResources, c.scene, d.Get("menu.main.profile"), func() {
		c.scene.Context().ChangeScene(NewProfileMenuController(c.state))
	})
	rowContainer.AddChild(profileButton)

	leaderboardButton := eui.NewButton(uiResources, c.scene, d.Get("menu.main.leaderboard"), func() {
		c.scene.Context().ChangeScene(NewLeaderboardMenuController(c.state))
	})
	rowContainer.AddChild(leaderboardButton)

	settingsButton := eui.NewButton(uiResources, c.scene, d.Get("menu.main.settings"), func() {
		c.scene.Context().ChangeScene(NewOptionsController(c.state))
	})
	rowContainer.AddChild(settingsButton)

	creditsButton := eui.NewButton(uiResources, c.scene, d.Get("menu.main.credits"), func() {
		c.scene.Context().ChangeScene(NewCreditsMenuController(c.state))
	})
	rowContainer.AddChild(creditsButton)

	var exitButton eui.Widget
	if runtime.GOARCH != "wasm" {
		exitButton = eui.NewButton(uiResources, c.scene, d.Get("menu.main.exit"), func() {
			os.Exit(0)
		})
		rowContainer.AddChild(exitButton)
	}

	var navTree *gameui.NavTree
	{
		buttons := []eui.Widget{
			playButton,
			profileButton,
			leaderboardButton,
			settingsButton,
			creditsButton,
		}
		if exitButton != nil {
			buttons = append(buttons, exitButton)
		}
		navTree = createSimpleNavTree(buttons)
	}

	rowContainer.AddChild(eui.NewTransparentSeparator())

	buildVersion := strconv.Itoa(gamedata.BuildNumber)
	if gamedata.BuildMinorNumber != 0 {
		buildVersion += "." + strconv.Itoa(gamedata.BuildMinorNumber)
	}

	buildLabel := fmt.Sprintf("%s %s", d.Get("menu.main.build"), buildVersion)
	if buildinfo.Distribution != buildinfo.TagUnknown {
		buildLabel += " [" + buildinfo.Distribution + "]"
	}

	buildVersionLabel := eui.NewCenteredLabel(buildLabel, assets.BitmapFont1)
	rowContainer.AddChild(buildVersionLabel)

	setupUI(c.scene, root, c.state.MenuInput, navTree)
}
