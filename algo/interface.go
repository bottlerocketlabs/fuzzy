package algo

type TextScorer interface {
	Compare(a, b string) float64
}
