package menus

import (
	"fmt"

	"github.com/ebitenui/ebitenui/widget"
	"github.com/quasilyte/ge"
	"github.com/quasilyte/roboden-game/assets"
	"github.com/quasilyte/roboden-game/controls"
	"github.com/quasilyte/roboden-game/gamedata"
	"github.com/quasilyte/roboden-game/gameui/eui"
	"github.com/quasilyte/roboden-game/serverapi"
	"github.com/quasilyte/roboden-game/session"
	"github.com/quasilyte/roboden-game/timeutil"
)

type ProfileStatsMenuController struct {
	state *session.State

	scene *ge.Scene
}

func NewProfileStatsMenuController(state *session.State) *ProfileStatsMenuController {
	return &ProfileStatsMenuController{state: state}
}

func (c *ProfileStatsMenuController) Init(scene *ge.Scene) {
	c.scene = scene
	c.initUI()
}

func (c *ProfileStatsMenuController) Update(delta float64) {
	c.state.MenuInput.Update()
	if c.state.MenuInput.ActionIsJustPressed(controls.ActionMenuBack) {
		c.back()
		return
	}
}

func (c *ProfileStatsMenuController) initUI() {
	eui.AddBackground(c.state.BackgroundImage, c.scene)
	uiResources := c.state.Resources.UI

	root := eui.NewAnchorContainer()
	rowContainer := eui.NewRowLayoutContainerWithMinWidth(400, 10, nil)
	root.AddChild(rowContainer)

	d := c.scene.Dict()

	titleLabel := eui.NewCenteredLabel(d.Get("menu.main.profile")+" -> "+d.Get("menu.profile.stats"), assets.BitmapFont3)
	rowContainer.AddChild(titleLabel)

	panel := eui.NewTextPanel(uiResources, 0, 0)
	rowContainer.AddChild(panel)

	smallFont := assets.BitmapFont2
	stats := c.state.Persistent.PlayerStats

	grid := eui.NewGridContainer(2, widget.GridLayoutOpts.Spacing(24, 4),
		widget.GridLayoutOpts.Stretch([]bool{true, false}, nil))
	lines := [][2]string{
		{d.Get("menu.results.time_played"), fmt.Sprintf("%v", timeutil.FormatDuration(d, stats.TotalPlayTime))},
		{d.Get("menu.profile.stats.totalscore"), fmt.Sprintf("%v", stats.TotalScore)},
		{d.Get("menu.profile.stats.classic_highscore"), fmt.Sprintf("%v (%d%%)", stats.HighestClassicScore, stats.HighestClassicScoreDifficulty)},
	}
	lines = append(lines, [2]string{d.Get("menu.profile.stats.blitz_highscore"), fmt.Sprintf("%v (%d%%)", stats.HighestBlitzScore, stats.HighestBlitzScoreDifficulty)})
	if stats.TotalScore >= gamedata.ArenaModeCost {
		lines = append(lines, [2]string{d.Get("menu.profile.stats.arena_highscore"), fmt.Sprintf("%v (%d%%)", stats.HighestArenaScore, stats.HighestArenaScoreDifficulty)})
	}
	if stats.TotalScore >= gamedata.ReverseModeCost {
		lines = append(lines, [2]string{d.Get("menu.profile.stats.reverse_highscore"), fmt.Sprintf("%v (%d%%)", stats.HighestReverseScore, stats.HighestReverseScoreDifficulty)})
	}
	if stats.TotalScore >= gamedata.InfArenaModeCost {
		lines = append(lines, [2]string{d.Get("menu.profile.stats.inf_arena_highscore"), fmt.Sprintf("%v (%d%%)", stats.HighestInfArenaScore, stats.HighestInfArenaScoreDifficulty)})
	}
	for _, pair := range lines {
		grid.AddChild(eui.NewLabel(pair[0], smallFont))
		grid.AddChild(eui.NewLabel(pair[1], smallFont))
	}

	panel.AddChild(grid)

	var buttons []eui.Widget

	var sendScoreButton *widget.Button
	sendScoreButton = eui.NewButton(uiResources, c.scene, d.Get("menu.publish_high_score"), func() {
		if c.state.Persistent.PlayerName == "" {
			backController := NewProfileStatsMenuController(c.state)
			userNameScene := c.state.SceneRegistry.UserNameMenu(backController)
			c.scene.Context().ChangeScene(userNameScene)
			return
		}
		c.state.SentHighscores = true
		sendScoreButton.GetWidget().Disabled = true
		replays := c.prepareHighscoreReplays()
		if len(replays) != 0 {
			backController := NewProfileStatsMenuController(c.state)
			submitController := c.state.SceneRegistry.SubmitScreen(backController, replays)
			c.scene.Context().ChangeScene(submitController)
			return
		}
	})
	buttons = append(buttons, sendScoreButton)
	rowContainer.AddChild(sendScoreButton)
	sendScoreButton.GetWidget().Disabled = c.state.SentHighscores ||
		(c.state.Persistent.PlayerStats.HighestClassicScore == 0 &&
			c.state.Persistent.PlayerStats.HighestBlitzScore == 0 &&
			c.state.Persistent.PlayerStats.HighestArenaScore == 0 &&
			c.state.Persistent.PlayerStats.HighestInfArenaScore == 0 &&
			c.state.Persistent.PlayerStats.HighestReverseScore == 0)

	backButton := eui.NewButton(uiResources, c.scene, d.Get("menu.back"), func() {
		c.back()
	})
	rowContainer.AddChild(backButton)
	buttons = append(buttons, backButton)

	navTree := createSimpleNavTree(buttons)
	setupUI(c.scene, root, c.state.MenuInput, navTree)
}

func (c *ProfileStatsMenuController) prepareHighscoreReplays() []serverapi.GameReplay {
	keys := []string{
		"classic_highscore.json",
		"blitz_highscore.json",
		"arena_highscore.json",
		"inf_arena_highscore.json",
		"reverse_highscore.json",
	}
	var replays []serverapi.GameReplay
	for _, key := range keys {
		if !c.state.CheckGameItem(key) {
			continue
		}
		var replay serverapi.GameReplay
		if err := c.state.LoadGameItem(key, &replay); err != nil {
			c.state.Logf("load %q highscore data: %v", key, err)
			continue
		}
		if gamedata.IsSendableReplay(replay) && gamedata.IsValidReplay(replay) {
			replays = append(replays, replay)
		}
	}
	return replays
}

func (c *ProfileStatsMenuController) back() {
	c.scene.Context().ChangeScene(NewProfileMenuController(c.state))
}
