package fuzzy

import (
	"bufio"
	"fmt"
	"io"
	"math"
	"sort"
	"strings"

	"github.com/bottlerocketlabs/fuzzy/algo"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// ValueStringer has a value and a string output, potentially different
type ValueStringer interface {
	// String used for searching
	String() string
	// Value returned to user via stdout
	Value() string
}

//EnumerableValueStringer is iterable
type EnumerableValueStringer interface {
	Each(handler func(ValueStringer))
}

// InputItem is an item of ValueStringer with a Score
type InputItem struct {
	item  ValueStringer
	Score float64
}

func NewInputItem(item ValueStringer) InputItem {
	return InputItem{
		item:  item,
		Score: 1,
	}
}

// SortableInputItems can be Sorted by their Score and difference in length of query
type SortableInputItems struct {
	items []InputItem
	query string
}

func (i SortableInputItems) Len() int      { return len(i.items) }
func (i SortableInputItems) Swap(x, y int) { i.items[x], i.items[y] = i.items[y], i.items[x] }
func (i SortableInputItems) Less(x, y int) bool {
	if i.items[x].Score < i.items[y].Score {
		return true
	}
	// If Score matches sort by length difference to Query
	if math.Abs(float64(len(i.query)-len(i.items[x].item.String()))) > math.Abs(float64(len(i.query)-len(i.items[y].item.String()))) {
		return true
	}
	return false
}

// StrMapEntry implements ValueStringer
type StrMapEntry struct {
	Val string
	Str string
}

func (s StrMapEntry) String() string { return s.Str }
func (s StrMapEntry) Value() string  { return s.Val }

// NewStrMap is a utility function to convert a string map
func NewStrMap(m map[string]string) StrMap { return StrMap(m) }

// StrMap uses the key as Value and value as String
type StrMap map[string]string

// Each allows iteration over the string map
func (m StrMap) Each(f func(ValueStringer)) {
	for v, s := range m {
		f(StrMapEntry{
			Val: v,
			Str: s,
		})
	}
}

// NewStrList is a utility fuction to convert a string slice
func NewStrList(s []string) StrList { return StrList(s) }

// StrList is a simple string slice
type StrList []string

// Each allows iteration over the string slice
func (l StrList) Each(f func(ValueStringer)) {
	for _, v := range l {
		f(NewStr(v))
	}
}

// Str is a simple string type
type Str string

// NewStr returns a ValueStringer type from a string
func NewStr(content string) Str {
	return Str(content)
}

func (s Str) String() string { return string(s) }
func (s Str) Value() string  { return string(s) }

type NopScorer struct{}

func (NopScorer) Compare(a, b string) float64 {
	return 1.0
}

// Content holds the data for the fuzzy finder
type Content struct {
	scorer algo.TextScorer
	tview.TableContentReadOnly
	data            []InputItem
	live            []InputItem
	verbose         bool
	hideLessThan    float64
	returnOneResult bool
}

// SupplyNewContent creates a new Content from a slice of ValueStringer types
func SupplyNewContent(input EnumerableValueStringer) *Content {
	data := []InputItem{}
	input.Each(func(vs ValueStringer) {
		data = append(data, NewInputItem(vs))
	})
	return newContent(data)
}

// ReadNewContent creates a new Content from new line separated input
func ReadNewContent(input io.Reader) *Content {
	data := []InputItem{}
	scanner := bufio.NewScanner(input)
	for scanner.Scan() {
		data = append(data, NewInputItem(NewStr(scanner.Text())))
	}
	return newContent(data)
}

func newContent(data []InputItem) *Content {
	return &Content{
		scorer:          NopScorer{},
		data:            data,
		live:            data,
		hideLessThan:    1,
		returnOneResult: false,
	}
}

// SetTextScorer sets the algorithm for scoring the query against the line
func (c *Content) SetTextScorer(textScorer algo.TextScorer) {
	c.scorer = textScorer
}

// SetVerbose outputs the scores along with the line. useful for debugging
func (c *Content) SetVerbose() {
	c.verbose = true
}

// SetReturnOneResult returns the one result immediatly if a passed in query only matches one item
func (c *Content) SetReturnOneResult() {
	c.returnOneResult = true
}

// SetHideLessThan remove item from output with a lower score
func (c *Content) SetHideLessThan(score float64) {
	c.hideLessThan = score
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

func removeEmpty(in []string) []string {
	var out []string
	for _, str := range in {
		if str != "" {
			out = append(out, str)
		}
	}
	return out
}

// Filter processes InputItems, scores them with SmithWaterman
// Any items with score less than 1 are not shown
// Items are sorted by their score
func (c *Content) Filter(query string) {
	if query == "" {
		c.live = c.data
		return
	}
	live := SortableInputItems{
		items: []InputItem{},
		query: query,
	}
	queryParts := removeEmpty(strings.Split(query, " "))
	for _, item := range c.data {
		var score float64
		for _, part := range queryParts {
			score = score + c.scorer.Compare(item.item.String(), part)
		}
		item.Score = score / float64(len(queryParts))
		if item.Score < c.hideLessThan {
			continue
		}
		live.items = append(live.items, item)
	}
	sort.Sort(sort.Reverse(live))
	//sort.Sort(live)
	c.live = live.items
}

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

	content.Filter(query)
	if content.GetRowCount() == 1 && content.returnOneResult {
		return content.live[0].item.Value(), nil
	}
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
		output = content.live[row].item.Value()
		app.Stop()
	})
	if err := app.SetRoot(grid, true).EnableMouse(true).Run(); err != nil {
		return "", err
	}
	return output, nil
}
