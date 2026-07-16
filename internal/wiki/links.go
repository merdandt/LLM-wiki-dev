package wiki

import (
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

func ExtractLinks(markdown []byte) []string {
	root := goldmark.DefaultParser().Parse(text.NewReader(markdown))
	var links []string
	_ = ast.Walk(root, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		if link, ok := node.(*ast.Link); ok {
			links = append(links, string(link.Destination))
		}
		return ast.WalkContinue, nil
	})
	return links
}
