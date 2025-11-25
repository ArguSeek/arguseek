package content

import (
	"strings"

	"github.com/PuerkitoBio/goquery"
)

type ContentExtractor struct {
	strategies []ExtractionStrategy
}

type ExtractionStrategy interface {
	Extract(doc *goquery.Document) (*goquery.Selection, float64)
	Name() string
}

func NewContentExtractor() *ContentExtractor {
	return &ContentExtractor{
		strategies: []ExtractionStrategy{
			&MainElementStrategy{},
			&ArticleStrategy{},
			&SemanticStrategy{},
			&HeuristicStrategy{},
		},
	}
}

func (ce *ContentExtractor) ExtractMainContent(doc *goquery.Document) (*goquery.Selection, float64, string) {
	var bestSelection *goquery.Selection
	var bestConfidence float64
	var bestStrategy string

	for _, strategy := range ce.strategies {
		selection, confidence := strategy.Extract(doc)
		if confidence > bestConfidence && selection != nil && selection.Length() > 0 {
			bestSelection = selection
			bestConfidence = confidence
			bestStrategy = strategy.Name()
		}

		if bestConfidence >= 0.9 {
			break
		}
	}

	if bestSelection == nil {
		return doc.Find("body"), 0.1, "fallback_body"
	}

	return bestSelection, bestConfidence, bestStrategy
}

type MainElementStrategy struct{}

func (s *MainElementStrategy) Name() string {
	return "main_element"
}

func (s *MainElementStrategy) Extract(doc *goquery.Document) (*goquery.Selection, float64) {
	main := doc.Find("main").First()
	if main.Length() > 0 {
		return main, 0.95
	}

	content := doc.Find("[role='main']").First()
	if content.Length() > 0 {
		return content, 0.9
	}

	return nil, 0
}

type ArticleStrategy struct{}

func (s *ArticleStrategy) Name() string {
	return "article"
}

func (s *ArticleStrategy) Extract(doc *goquery.Document) (*goquery.Selection, float64) {
	article := doc.Find("article").First()
	if article.Length() > 0 {
		textLength := len(strings.TrimSpace(article.Text()))
		if textLength > 200 {
			return article, 0.85
		}
	}

	return nil, 0
}

type SemanticStrategy struct{}

func (s *SemanticStrategy) Name() string {
	return "semantic"
}

func (s *SemanticStrategy) Extract(doc *goquery.Document) (*goquery.Selection, float64) {
	selectors := []string{
		"#content",
		".content",
		"#main-content",
		".main-content",
		"#post",
		".post",
		"#entry",
		".entry",
		".markdown-body",
		".post-content",
		".entry-content",
	}

	for _, selector := range selectors {
		selection := doc.Find(selector).First()
		if selection.Length() > 0 {
			textLength := len(strings.TrimSpace(selection.Text()))
			if textLength > 300 {
				return selection, 0.7
			}
		}
	}

	return nil, 0
}

type HeuristicStrategy struct{}

func (s *HeuristicStrategy) Name() string {
	return "heuristic"
}

func (s *HeuristicStrategy) Extract(doc *goquery.Document) (*goquery.Selection, float64) {
	candidates := make(map[*goquery.Selection]float64)

	doc.Find("div, section").Each(func(i int, sel *goquery.Selection) {
		score := s.scoreCandidate(sel)
		if score > 0 {
			candidates[sel] = score
		}
	})

	var bestCandidate *goquery.Selection
	var bestScore float64

	for candidate, score := range candidates {
		if score > bestScore {
			bestCandidate = candidate
			bestScore = score
		}
	}

	if bestCandidate != nil && bestScore > 0.3 {
		confidence := bestScore
		if confidence > 0.6 {
			confidence = 0.6
		}
		return bestCandidate, confidence
	}

	return nil, 0
}

func (s *HeuristicStrategy) scoreCandidate(sel *goquery.Selection) float64 {
	score := 0.0

	text := strings.TrimSpace(sel.Text())
	textLength := len(text)

	if textLength < 300 {
		return 0
	}

	score += float64(textLength) / 10000.0

	paragraphs := sel.Find("p").Length()
	score += float64(paragraphs) * 0.05

	id, _ := sel.Attr("id")
	class, _ := sel.Attr("class")

	positivePatterns := []string{"content", "main", "article", "post", "entry", "text", "body"}
	negativePatterns := []string{"nav", "sidebar", "footer", "header", "menu", "comment", "ad", "advertisement"}

	for _, pattern := range positivePatterns {
		if strings.Contains(strings.ToLower(id), pattern) || strings.Contains(strings.ToLower(class), pattern) {
			score += 0.1
		}
	}

	for _, pattern := range negativePatterns {
		if strings.Contains(strings.ToLower(id), pattern) || strings.Contains(strings.ToLower(class), pattern) {
			score -= 0.2
		}
	}

	linkDensity := float64(sel.Find("a").Length()) / float64(len(strings.Fields(text)))
	if linkDensity > 0.3 {
		score -= 0.3
	}

	return score
}
