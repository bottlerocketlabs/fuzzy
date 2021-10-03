package algo

// TextScorer is the interface for all algorithms to impliment
type TextScorer interface {
	Compare(a, b string) float64
}
