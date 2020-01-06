package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"sort"
	"strings"

	"github.com/gotk3/gotk3/gdk"
	"github.com/gotk3/gotk3/gtk"
)

//////////////
/// MODEL ///
////////////

type output struct {
	Name   string `json:"name"`
	Active bool   `json:"active"`

	Modes       []mode `json:"modes"`
	CurrentMode mode   `json:"current_mode"`
	Rect        rect   `json:"rect"`
	Primary     bool   `json:"primary"`
	Scale       float64 `json:"scale"`

	Make   string `json:"make"`
	Model  string `json:"model"`
	Serial string `json:"serial"`
}

type mode struct {
	Width   int `json:"width"`
	Height  int `json:"height"`
	Refresh int `json:"refresh"`
}

type rect struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

func (r mode) Name() string {
	return fmt.Sprintf("%dx%d (%.2f Hz)", r.Width, r.Height, float64(r.Refresh)/1000.0)
}

func (r output) identifier() string {
	return fmt.Sprintf("\"%s %s %s\"", r.Make, r.Model, r.Serial)
}

func (r output) ToCommand(kanshiFormat bool) []string {
	var positionFormat string
	if kanshiFormat { positionFormat = "%d,%d" } else { positionFormat = "%d %d" }
	mode := r.CurrentMode
	if r.Active {
		return []string{
			"output", r.identifier(),
			"mode", fmt.Sprintf("%dx%d@%dHz", mode.Width, mode.Height, int(mode.Refresh/1000)),
			"position", fmt.Sprintf(positionFormat, r.Rect.X, r.Rect.Y),
			"scale", fmt.Sprintf("%.2f", r.Scale),
		}
	}
	return []string{"output", r.identifier(), "disable"}
}

func (r *output) ChangeMode(mode mode) {
	r.CurrentMode = mode
	r.Rect.Width = r.CurrentMode.Width
	r.Rect.Height = r.CurrentMode.Height
}

func (r *output) ApparentSize() (float64, float64) {
	scale := r.Scale
	return float64(r.CurrentMode.Width) * scale , float64(r.CurrentMode.Height) * scale
}

func parse_outputs() []output {
	str := get_outputs()
	res := []output{}
	json.Unmarshal([]byte(str), &res)
	fmt.Println(res)
	return res
}

type extent func(*output) (int, int)

func anchorPoints(outputs *[]output, currentOutput *output, extent extent) []int {
	anchors := make([]int, 0)
	for _, output := range *outputs {
		if output.Name != currentOutput.Name {
			_, currentOutputSize := extent(currentOutput)
			start, width := extent(&output)
			end := start + width
			anchors = append(anchors,
				world2map(start),
				world2map(end),
				world2map(start-currentOutputSize),
				world2map(end-currentOutputSize),
			)
		}
	}
	return anchors
}
func abs(val int) int {
	if val < 0 {
		return -val
	}
	return val
}

func moveWithStickyPoints(pos int, outputs *[]output, currentOutput *output, extent extent) int {
	anchors := anchorPoints(outputs, currentOutput, extent)
	close_anchors := make([]int, 0)

	for _, anchor := range anchors {
		if abs(anchor-pos) < 10 {
			close_anchors = append(close_anchors, anchor)
		}
	}
	sort.SliceStable(close_anchors, func(i, j int) bool {
		return abs(close_anchors[i]-pos) < abs(close_anchors[j]-pos)
	})
	if len(close_anchors) > 0 {
		println("close!!!")
		return close_anchors[0]
	}
	return pos
}

////////////
/// APP ///
///////////

const css = `
.inactive {
  opacity: 0.5;
}
.popoverBox {
  min-width: 400px;
  min-height: 400px;
}
`

func main() {
	// Initialize GTK without parsing any command line arguments.
	gtk.Init(nil)

	// Create a new toplevel window, set its title, and connect it to the
	// "destroy" signal to exit the GTK main loop when it is destroyed.
	win, err := gtk.WindowNew(gtk.WINDOW_TOPLEVEL)
	if err != nil {
		log.Fatal("Unable to create window:", err)
	}
	win.SetTitle("Simple Example")
	win.Connect("destroy", func() {
		gtk.MainQuit()
	})
	outputs := parse_outputs()

	// HeaderBar

	headerBar, _ := gtk.HeaderBarNew()
	headerBar.SetShowCloseButton(true)
	headerBar.SetTitle("wRandR")

	applyButton, _ := gtk.ButtonNewFromIconName("media-playback-start", gtk.ICON_SIZE_BUTTON)

	headerBar.PackStart(applyButton)
	applyButton.Connect("clicked", func() {
		for _, output := range outputs {
			args := output.ToCommand(false)
			fmt.Println(args)
			handle := exec.Command("swaymsg", args...)
			stdout, _ := handle.Output()
			println(string(stdout))
		}
	})

	commandOutputButton, _ := gtk.ButtonNewFromIconName("document-edit", gtk.ICON_SIZE_BUTTON)
	headerBar.PackEnd(commandOutputButton)
	commandOutputButton.Connect("clicked", func() {
		popover, _ := gtk.PopoverNew(commandOutputButton)
		box, _ := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 10)

		tagTable, _ := gtk.TextTagTableNew()
		textBuffer, _ := gtk.TextBufferNew(tagTable)
		textView, _ := gtk.TextViewNewWithBuffer(textBuffer)
		popover.Add(box)
		box.Add(textView)
		style, _ := box.GetStyleContext()
		style.AddClass("popoverBox")
		commands := ""
		for _, output := range outputs {
			args := output.ToCommand(true)

			commands += strings.Join(args[:], " ") + "\n"
		}
		textBuffer.SetText(commands)
		popover.ShowAll()
	})

	win.SetTitlebar(headerBar)

	layout, err := gtk.FixedNew()
	win.Add(layout)

	// Set the default window size.
	win.SetDefaultSize(800, 600)

	// Recursively show all widgets contained in this window.

	// Begin executing the GTK main loop.  This blocks until
	// gtk.MainQuit() is run.

	for i, _ := range outputs {
		MonitorComponentNew(layout, &outputs[i], &outputs)
	}

	cssProvider, err := gtk.CssProviderNew()
	if err != nil {
		log.Fatal("Could not find style sheet:", err)
	}
	err = cssProvider.LoadFromData(css)
	if err != nil {
		log.Fatal("err", err)
	}
	screen, _ := win.GetScreen()
	gtk.AddProviderForScreen(screen, cssProvider, gtk.STYLE_PROVIDER_PRIORITY_USER)

	win.ShowAll()

	gtk.Main()
}

func get_outputs() string {
	cmd := exec.Command("swaymsg", "-t", "get_outputs", "-r")
	stdout, _ := cmd.Output()
	return string(stdout)
}

/// MonitorView ///

func MonitorMenu(component *monitorComponent) *gtk.Popover {
	btn := component.view
	model := *component.model
	popover, _ := gtk.PopoverNew(btn)
	box, _ := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 10)
	box.SetMarginTop(10)
	box.SetMarginBottom(10)
	box.SetMarginStart(10)
	box.SetMarginEnd(10)
	popover.Add(box)

	activeBtn, _ := gtk.CheckButtonNewWithLabel("Activate")
	activeBtn.SetActive(model.Active)
	activeBtn.Connect("toggled", func() {
		component.model.Active = activeBtn.GetActive()
		if len(model.Modes) > 0 {
			component.model.ChangeMode(model.Modes[0])
		}
		component.update()
	})
	box.Add(activeBtn)


	resolutionLabel, _ := gtk.LabelNew("Resolution")
	box.Add(resolutionLabel)


	var prev *gtk.RadioButton = nil
	for _, mode := range model.Modes {
		mode := mode
		radio, _ := gtk.RadioButtonNewWithLabelFromWidget(prev, mode.Name())
		box.Add(radio)
		prev = radio
		radio.Connect("toggled", func() {
			if radio.GetActive() {
				component.model.ChangeMode(mode)
				component.update()
			}
		})
		radio.SetActive(mode == model.CurrentMode)
	}

	dpiScaleLabel, _ := gtk.LabelNew("DPI Scale")
	box.Add(dpiScaleLabel)

	dpiScale, _ := gtk.SpinButtonNewWithRange(0.1, 4, 0.1)
	dpiScale.SetValue(component.model.Scale)
	dpiScale.Connect("value-changed", func() {
		component.model.Scale = dpiScale.GetValue()
		component.update()
	})
	box.Add(dpiScale)

	box.ShowAll()

	return popover
}

func (r monitorComponent) update() {
	fmt.Println(r.model.Rect)
	mapWidth, mapHeight := r.model.ApparentSize()
	width, height := world2map(int(mapWidth)), world2map(int(mapHeight))
	r.view.SetSizeRequest(width, height)
	r.parent.Move(r.view, r.currentX, r.currentY)
	styleCtx, _ := r.view.GetStyleContext()
	if r.model.Active {
		styleCtx.AddClass("active")
		styleCtx.RemoveClass("inactive")
	} else {
		styleCtx.AddClass("inactive")
		styleCtx.RemoveClass("active")
	}
}

type monitorComponent struct {
	model    *output
	view     *gtk.Button
	parent   *gtk.Fixed
	currentX int
	currentY int
}

func extentX(output *output) (int, int) {
	width, _ := output.ApparentSize()
	return output.Rect.X, int(width)
}

func extentY(output *output) (int, int) {
	_, height := output.ApparentSize()
	return output.Rect.Y, int(height)
}

func MonitorComponentNew(layout *gtk.Fixed, model *output, allOutputs *[]output) *monitorComponent {
	btn, _ := gtk.ButtonNewWithLabel(fmt.Sprintf("%s\n%s\n%s", model.Name, model.Make, model.Model))

	component := monitorComponent{model: model, view: btn, parent: layout, currentX: world2map(model.Rect.X), currentY: world2map(model.Rect.Y)}
	layout.Put(btn, component.currentX, component.currentY)
	if (component.model.Scale < 0.1) {
		component.model.Scale = 1.0
	}
	component.update()

	mouseOffsetX, mouseOffsetY := 0.0, 0.0
	wasMoved := false
	btn.Connect("button_press_event", func(a *gtk.Button, event *gdk.Event) {
		buttonEvent := gdk.EventButtonNewFromEvent(event)
		toplevel, _ := layout.GetToplevel()
		offsetX, offsetY, _ := layout.TranslateCoordinates(toplevel, 0, 0)
		mouseOffsetX, mouseOffsetY = float64(offsetX)+buttonEvent.X(), float64(offsetY)+buttonEvent.Y()
	})
	btn.Connect("button_release_event", func(a *gtk.Button, event *gdk.Event) {
		if !wasMoved {
			popover := MonitorMenu(&component)
			popover.Show()
		} else {
			model.Rect.X = map2world(float64(component.currentX))
			model.Rect.Y = map2world(float64(component.currentY))
			component.update()
		}
		wasMoved = false
	})
	btn.Connect("motion_notify_event", func(a *gtk.Button, event *gdk.Event) {
		buttonEvent := gdk.EventButtonNewFromEvent(event)

		component.currentX = moveWithStickyPoints(int(buttonEvent.XRoot()-mouseOffsetX), allOutputs, component.model, extentX)
		component.currentY = moveWithStickyPoints(int(buttonEvent.YRoot()-mouseOffsetY), allOutputs, component.model, extentY)

		component.update()
		wasMoved = true
	})
	return &component
}

func map2world(val float64) int {
	return int(val * 7)
}

func world2map(val int) int {
	return val / 7
}
