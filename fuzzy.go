package fuzzy

import (
	"bufio"
	"fmt"
	"io"
	"sort"

	"github.com/bottlerocketlabs/fuzzy/algo"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// InputItem and item of Stringer with a Score
type InputItem struct {
	item  fmt.Stringer
	Score float64
}

func NewInputItem(item fmt.Stringer) InputItem {
	return InputItem{
		item:  item,
		Score: 1,
	}
}

// InputItems can be Sorted by Score
type InputItems []InputItem

func (i InputItems) Len() int           { return len(i) }
func (i InputItems) Swap(x, y int)      { i[x], i[y] = i[y], i[x] }
func (i InputItems) Less(x, y int) bool { return i[x].Score > i[y].Score }

// Content holds the data for the fuzzy finder
type Content struct {
	scorer algo.TextScorer
	tview.TableContentReadOnly
	data    InputItems
	live    InputItems
	verbose bool
}

type NopScorer struct{}

func (NopScorer) Compare(a, b string) float64 {
	return 1.0
}

// SupplyNewContent creates a new Content from a slice of Stringer types
func SupplyNewContent(input []fmt.Stringer) *Content {
	ts := NopScorer{}
	data := InputItems{}
	for _, item := range input {
		data = append(data, NewInputItem(item))
	}
	c := Content{
		scorer: ts,
		data:   data,
		live:   data,
	}
	return &c
}

// ReadNewContent creates a new Content from new line separated input
func ReadNewContent(input io.Reader) *Content {
	ts := NopScorer{}
	data := InputItems{}
	scanner := bufio.NewScanner(input)
	for scanner.Scan() {
		data = append(data, NewInputItem(NewStr(scanner.Text())))
	}
	c := Content{
		scorer: ts,
		data:   data,
		live:   data,
	}
	return &c
}

// SetTextScorer sets the algorithm for scoring the query against the line
func (c *Content) SetTextScorer(textScorer algo.TextScorer) {
	c.scorer = textScorer
}

// SetVerbose outputs the scores along with the line. useful for debugging
func (c *Content) SetVerbose() {
	c.verbose = true
}

func (c *Content) GetCell(row, column int) *tview.TableCell {
	if 0 > row || row > c.GetRowCount() {
		return nil
	}
	r := c.live[row]
	if c.verbose {
		return tview.NewTableCell(fmt.Sprintf("%s [%f]", r.item.String(), r.Score))
	}
	return tview.NewTableCell(r.item.String())
}

func (c *Content) GetColumnCount() int {
	return 1
}

func (c *Content) GetRowCount() int {
	return len(c.live)
}

// Filter processes InputItems, scores them with SmithWaterman
// Any items with score less than 0 are not shown
// Items are sorted by their score
func (c *Content) Filter(query string) {
	if query == "" {
		c.live = c.data
		return
	}
	var live InputItems
	for _, item := range c.data {
		item.Score = c.scorer.Compare(item.item.String(), query)
		if item.Score < 1 {
			continue
		}
		live = append(live, item)
	}
	sort.Sort(live)
	c.live = live
}

type Str struct {
	content string
}

// NewStr returns a Stringer type from a string
func NewStr(content string) Str {
	return Str{content: content}
}

func (s Str) String() string { return s.content }

// Find takes each line from provided content, computes the smith waterman score
// orders the content and provides a user-interface to select an option
func Find(query string, content *Content) (string, error) {
	return FindWithScreen(nil, query, content)
}

// FindWithScreen is the same as Find, but you provide the Screen
func FindWithScreen(screen tcell.Screen, query string, content *Content) (string, error) {
	app := tview.NewApplication().SetScreen(screen)
	table := tview.NewTable().
		SetBorders(false).
		SetSelectable(true, false).
		SetContent(content)

	inputField := tview.NewInputField().SetLabel("> ").SetChangedFunc(func(text string) {
		content.Filter(text)
		table.ScrollToBeginning().Select(0, 0)
	})
	tableInputSend := table.InputHandler()
	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch key := event.Key(); key {
		case tcell.KeyUp:
			tableInputSend(event, nil)
			return nil
		case tcell.KeyDown:
			tableInputSend(event, nil)
			return nil
		case tcell.KeyEnter:
			tableInputSend(event, nil)
			return nil
		case tcell.KeyEscape:
			tableInputSend(event, nil)
			return nil
		}
		return event
	})
	inputField.SetText(query)
	grid := tview.NewGrid().
		SetRows(0, 1).
		SetColumns(0).
		SetBorders(false).
		AddItem(table, 0, 0, 1, 1, 0, 0, false).
		AddItem(inputField, 1, 0, 1, 1, 0, 0, true)
	var output string
	table.Select(0, 0).SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEscape {
			app.Stop()
		}
	}).SetSelectedFunc(func(row int, column int) {
		cell := table.GetCell(row, column)
		output = cell.Text
		app.Stop()
	})
	if err := app.SetRoot(grid, true).EnableMouse(true).Run(); err != nil {
		return "", err
	}
	return output, nil
}
