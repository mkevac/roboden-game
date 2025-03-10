package menus

import (
	"fmt"
	"math"
	"strings"

	"github.com/ebitenui/ebitenui/widget"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/quasilyte/ge"
	"github.com/quasilyte/ge/xslices"
	"github.com/quasilyte/roboden-game/assets"
	"github.com/quasilyte/roboden-game/controls"
	"github.com/quasilyte/roboden-game/gamedata"
	"github.com/quasilyte/roboden-game/gameui"
	"github.com/quasilyte/roboden-game/gameui/eui"
	"github.com/quasilyte/roboden-game/session"
)

type ProfileAchievementsMenuController struct {
	state *session.State

	descriptions []string
	buttons      []eui.Widget
	helpLabel    *widget.Text

	scene *ge.Scene
}

func NewProfileAchievementsMenuController(state *session.State) *ProfileAchievementsMenuController {
	return &ProfileAchievementsMenuController{state: state}
}

func (c *ProfileAchievementsMenuController) Init(scene *ge.Scene) {
	c.scene = scene
	c.initUI()
}

func (c *ProfileAchievementsMenuController) Update(delta float64) {
	c.state.MenuInput.Update()
	if c.state.MenuInput.ActionIsJustPressed(controls.ActionMenuBack) {
		c.back()
		return
	}
}

func (c *ProfileAchievementsMenuController) paintIcon(icon *ebiten.Image) *ebiten.Image {
	painted := ebiten.NewImage(icon.Size())
	var options ebiten.DrawImageOptions
	options.ColorM.Scale(0, 0, 0, 1)
	painted.DrawImage(icon, &options)
	return painted
}

func (c *ProfileAchievementsMenuController) initUI() {
	eui.AddBackground(c.state.BackgroundImage, c.scene)
	uiResources := c.state.Resources.UI

	root := eui.NewAnchorContainer()
	rowContainer := eui.NewRowLayoutContainer(10, nil)
	root.AddChild(rowContainer)

	d := c.scene.Dict()

	smallFont := assets.BitmapFont1

	helpLabel := eui.NewLabel("", smallFont)
	helpLabel.MaxWidth = 320
	c.helpLabel = helpLabel

	navTree := gameui.NewNavTree()
	navBlock := navTree.NewBlock()
	numColumns := 7
	numRows := int(math.Ceil(float64(len(gamedata.AchievementList)) / float64(numColumns)))

	backButton := eui.NewButton(uiResources, c.scene, d.Get("menu.back"), func() {
		c.back()
	})
	backButtonElem := navBlock.NewElem(backButton)

	var gridButtonElems []*gameui.NavElem

	titleLabel := eui.NewCenteredLabel(d.Get("menu.main.profile")+" -> "+d.Get("menu.profile.achievements"), assets.BitmapFont3)
	rowContainer.AddChild(titleLabel)

	rootGrid := widget.NewContainer(
		widget.ContainerOpts.WidgetOpts(widget.WidgetOpts.LayoutData(widget.RowLayoutData{
			Stretch: true,
		})),
		widget.ContainerOpts.Layout(widget.NewGridLayout(
			widget.GridLayoutOpts.Columns(2),
			widget.GridLayoutOpts.Stretch([]bool{false, true}, nil),
			widget.GridLayoutOpts.Spacing(4, 4))))
	leftPanel := eui.NewPanel(uiResources, 0, 0)
	leftGrid := widget.NewContainer(
		widget.ContainerOpts.Layout(widget.NewGridLayout(
			widget.GridLayoutOpts.Columns(numColumns),
			widget.GridLayoutOpts.Spacing(4, 4))))
	for i := range gamedata.AchievementList {
		achievement := gamedata.AchievementList[i]
		status := xslices.Find(c.state.Persistent.PlayerStats.Achievements, func(a *session.Achievement) bool {
			return a.Name == achievement.Name
		})
		grade := 0
		img := c.scene.LoadImage(achievement.Icon).Data
		if status != nil {
			img = c.paintIcon(img)
			if status.Elite {
				grade = 2
			} else {
				grade = 1
			}
		}
		b := eui.NewItemButton(uiResources, img, smallFont, strings.Repeat("*", grade), 44, func() {})
		c.descriptions = append(c.descriptions, (func() string {
			var lines []string
			statusText := d.Get("achievement.grade.none")
			switch grade {
			case 1:
				statusText = d.Get("achievement.grade.normal")
			case 2:
				statusText = d.Get("achievement.grade.elite")
			}
			lines = append(lines, fmt.Sprintf("%s (%s)", d.Get("achievement", achievement.Name), statusText))
			lines = append(lines, "")
			lines = append(lines, d.Get("achievement", achievement.Name, "description"))
			if achievement.Mode != gamedata.ModeAny {
				lines = append(lines, "")
				lines = append(lines, fmt.Sprintf("%s: %s", d.Get("achievement.game_mode"), d.Get("achievement.mode", achievement.Mode.String())))
			}
			return strings.Join(lines, "\n")
		})())
		desc := c.descriptions[i]
		b.Button.CursorEnteredEvent.AddHandler(func(args interface{}) {
			helpLabel.Label = desc
		})
		if status != nil {
			b.Toggle()
		}
		leftGrid.AddChild(b.Widget)
		c.buttons = append(c.buttons, b.Widget)
		gridButtonElems = append(gridButtonElems, navBlock.NewElem(b.Button))
	}
	leftPanel.AddChild(leftGrid)

	rightPanel := eui.NewTextPanel(uiResources, 380, 0)
	rightPanel.AddChild(helpLabel)

	rootGrid.AddChild(leftPanel)
	rootGrid.AddChild(rightPanel)

	rowContainer.AddChild(rootGrid)

	rowContainer.AddChild(backButton)

	backButtonElem.Edges[gameui.NavUp] = gridButtonElems[len(gridButtonElems)-1]
	bindNavGrid(gridButtonElems, numColumns, numRows)
	for _, e := range gridButtonElems {
		if e.Edges[gameui.NavDown] != nil {
			continue
		}
		e.Edges[gameui.NavDown] = backButtonElem
	}

	setupUI(c.scene, root, c.state.MenuInput, navTree)
}

func (c *ProfileAchievementsMenuController) back() {
	c.scene.Context().ChangeScene(NewProfileMenuController(c.state))
}
