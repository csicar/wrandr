package main

import (
	"fmt"
	"log"
	"os/exec"
	"strings"
	"encoding/json"

	"github.com/gotk3/gotk3/gtk"
	"github.com/gotk3/gotk3/gdk"
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

func (r output) ToCommand() []string {
	mode := r.CurrentMode
	if r.Active {
		return []string{
			"output", r.Name,
			"mode", fmt.Sprintf("%dx%d@%dHz", mode.Width, mode.Height, int(mode.Refresh/1000)),
			"pos", fmt.Sprintf("%d %d", r.Rect.X, r.Rect.Y),
		}
	}
	return []string{"output", r.Name, "disable"}
}

func (r *output) ChangeMode(mode mode) {
	r.CurrentMode = mode
	r.Rect.Width = r.CurrentMode.Width
	r.Rect.Height = r.CurrentMode.Height
}

func parse_outputs() []output {
	str := get_outputs()
	res := []output{}
	json.Unmarshal([]byte(str), &res)
	fmt.Println(res)
	return res
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
	headerBar.SetTitle("WaRandR")

	applyButton, _ := gtk.ButtonNewFromIconName("media-playback-start", gtk.ICON_SIZE_BUTTON)

	headerBar.PackStart(applyButton)
	applyButton.Connect("clicked", func() {
		for _, output := range outputs {
			args := output.ToCommand()
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
			args := output.ToCommand()

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
		MonitorComponentNew(layout, &outputs[i])
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
		component.update()
	})
	box.Add(activeBtn)

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
	box.ShowAll()

	return popover
}

func (r monitorComponent) update() {
	fmt.Println(r.model.Rect)
	width, height := world2map(r.model.Rect.Width), world2map(r.model.Rect.Height)
	r.view.SetSizeRequest(width, height)
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
	model  *output
	view   *gtk.Button
	parent *gtk.Fixed
}

func MonitorComponentNew(layout *gtk.Fixed, model *output) *monitorComponent {
	btn, _ := gtk.ButtonNew()
	btn.SetLabel(model.Name)

	component := monitorComponent{model: model, view: btn, parent: layout}
	component.update()

	x, y := world2map(model.Rect.X), world2map(model.Rect.Y)
	layout.Put(btn, x, y)

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
		}
		wasMoved = false
	})
	btn.Connect("motion_notify_event", func(a *gtk.Button, event *gdk.Event) {
		buttonEvent := gdk.EventButtonNewFromEvent(event)
		x := buttonEvent.XRoot() - mouseOffsetX
		y := buttonEvent.YRoot() - mouseOffsetY
		model.Rect.X = map2world(x)
		model.Rect.Y = map2world(y)
		layout.Move(btn, int(x), int(y))
		wasMoved = true
	})
	return &component
}

func map2world(val float64) int {
	return int(val * 10)
}

func world2map(val int) int {
	return val / 10
}